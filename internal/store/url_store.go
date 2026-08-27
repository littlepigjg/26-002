package store

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/ubaas/ubaas/internal/config"
	"github.com/ubaas/ubaas/internal/model"
	"github.com/ubaas/ubaas/pkg/logger"
)

// PanicGuardFn is a function that checks if a panic should be guarded.
// It receives the code and raw URL, returns true if the panic should be recovered.
type PanicGuardFn func(code, rawURL string) bool

// URLStore manages the storage and retrieval of short URLs.
type URLStore struct {
	mu        sync.RWMutex
	cfg       *config.Config
	logger    *logger.Logger
	urls      map[string]model.ShortURL
	filePath  string
	guardFn   PanicGuardFn
	isLoaded  bool
	isClosed  bool
	syncTimer *time.Ticker
	stopCh    chan struct{}
	wg        sync.WaitGroup
}

// NewURLStore creates a new URLStore.
func NewURLStore(cfg *config.Config) (*URLStore, error) {
	filePath := cfg.Storage.GetURLFilePath()
	if filePath == "" {
		filePath = "data/urls.json"
	}

	return &URLStore{
		cfg:      cfg,
		logger:   logger.New(os.Stdout, 2, "url_store"),
		urls:     make(map[string]model.ShortURL),
		filePath: filePath,
		stopCh:   make(chan struct{}),
	}, nil
}

// Load loads URLs from the configured file path.
func (s *URLStore) Load(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.isClosed {
		return model.ErrStoreClosed
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	data, err := os.ReadFile(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			s.isLoaded = true
			return nil
		}
		return fmt.Errorf("failed to read URL store file: %w", err)
	}

	if len(data) > 0 {
		var urls map[string]model.ShortURL
		if err := json.Unmarshal(data, &urls); err != nil {
			return fmt.Errorf("failed to parse URL store data: %w", err)
		}
		s.urls = urls
	}

	s.isLoaded = true

	s.syncTimer = time.NewTicker(s.cfg.Storage.GetSyncInterval())
	s.wg.Add(1)
	go s.syncLoop()

	return nil
}

// syncLoop periodically syncs data to disk.
func (s *URLStore) syncLoop() {
	defer s.wg.Done()
	for {
		select {
		case <-s.syncTimer.C:
			s.flush()
		case <-s.stopCh:
			s.flush()
			return
		}
	}
}

// flush writes the current state to disk.
func (s *URLStore) flush() {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.isClosed || !s.isLoaded {
		return
	}

	data, err := json.MarshalIndent(s.urls, "", "  ")
	if err != nil {
		s.logger.Errorf("Failed to marshal URL store: %v", err)
		return
	}

	if err := os.WriteFile(s.filePath, data, 0644); err != nil {
		s.logger.Errorf("Failed to write URL store: %v", err)
	}
}

// Close shuts down the URL store.
func (s *URLStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.isClosed {
		return nil
	}

	s.isClosed = true
	close(s.stopCh)

	if s.syncTimer != nil {
		s.syncTimer.Stop()
	}

	s.wg.Wait()
	s.flush()

	s.logger.Info("URLStore closed")
	return nil
}

// SetPanicGuard sets a function to guard against panics during Save operations.
func (s *URLStore) SetPanicGuard(fn PanicGuardFn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.guardFn = fn
}

// Save stores a short URL entry. If overwrite is false and the code already exists,
// it returns an error.
func (s *URLStore) Save(u *model.ShortURL, overwrite bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.isClosed {
		return model.ErrStoreClosed
	}

	if u == nil {
		return model.ErrInvalidRequest
	}

	if err := u.Validate(); err != nil {
		return err
	}

	if !overwrite {
		if _, exists := s.urls[u.Code]; exists {
			return fmt.Errorf("URL code %s already exists", u.Code)
		}
	}

	if s.guardFn != nil {
		if s.guardFn(u.Code, u.RawURL) {
			panic(fmt.Sprintf("panic guarded for code=%s", u.Code))
		}
	}

	s.urls[u.Code] = *u

	if s.cfg.Storage.GetFlushOnWrite() {
		s.flush()
	}

	return nil
}

// Get retrieves a short URL by its code.
func (s *URLStore) Get(code string) (*model.ShortURL, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.isClosed {
		return nil, model.ErrStoreClosed
	}

	u, ok := s.urls[code]
	if !ok {
		return nil, fmt.Errorf("URL code %s not found", code)
	}

	result := u
	return &result, nil
}

// RawSnapshot returns a snapshot of all stored URLs.
func (s *URLStore) RawSnapshot() map[string]model.ShortURL {
	s.mu.RLock()
	defer s.mu.RUnlock()

	snapshot := make(map[string]model.ShortURL, len(s.urls))
	for k, v := range s.urls {
		snapshot[k] = v
	}
	return snapshot
}

// IncrementVisits increments the visit counter for a short URL.
func (s *URLStore) IncrementVisits(code string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.isClosed {
		return model.ErrStoreClosed
	}

	u, ok := s.urls[code]
	if !ok {
		return fmt.Errorf("URL code %s not found", code)
	}

	u.Visits++
	s.urls[code] = u

	if s.cfg.Storage.GetFlushOnWrite() {
		s.flush()
	}

	return nil
}
