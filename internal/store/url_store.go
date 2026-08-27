package store

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/ubaas/ubaas/internal/config"
	"github.com/ubaas/ubaas/internal/model"
	"github.com/ubaas/ubaas/pkg/logger"
)

type PanicGuardFn func(code, rawURL string) bool

type URLStore struct {
	mu        sync.RWMutex
	cfg       *config.Config
	urls      map[string]model.ShortURL
	open      bool
	panicGuard PanicGuardFn
	log       *logger.Logger
}

func NewURLStore(cfg *config.Config) (*URLStore, error) {
	log := logger.New(os.Stdout, logger.LevelInfo, "[URLStore]")
	us := &URLStore{
		cfg:  cfg,
		urls: make(map[string]model.ShortURL),
		open: true,
		log:  log,
	}
	return us, nil
}

func (us *URLStore) Load(ctx context.Context) error {
	us.mu.Lock()
	defer us.mu.Unlock()

	if !us.open {
		return model.ErrStorageNotOpen
	}

	urlFilePath := us.cfg.Storage.GetURLFilePath()
	if urlFilePath == "" {
		urlFilePath = "data/urls.json"
	}

	dir := filepath.Dir(urlFilePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	data, err := os.ReadFile(urlFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read URL file: %w", err)
	}

	var urls map[string]model.ShortURL
	if err := json.Unmarshal(data, &urls); err != nil {
		return fmt.Errorf("failed to unmarshal URL data: %w", err)
	}

	us.urls = urls
	us.log.Infof("Loaded %d URLs from %s", len(us.urls), urlFilePath)
	return nil
}

func (us *URLStore) Close() error {
	us.mu.Lock()
	defer us.mu.Unlock()

	if !us.open {
		return nil
	}

	us.open = false
	us.log.Info("URLStore closed")
	return nil
}

func (us *URLStore) SetPanicGuard(fn PanicGuardFn) {
	us.mu.Lock()
	defer us.mu.Unlock()
	us.panicGuard = fn
}

func (us *URLStore) Save(u *model.ShortURL, overwrite bool) error {
	us.mu.Lock()
	defer us.mu.Unlock()

	if !us.open {
		return model.ErrStorageNotOpen
	}

	if !overwrite {
		if _, exists := us.urls[u.Code]; exists {
			return model.ErrCodeExists
		}
	}

	if us.panicGuard != nil {
		if shouldPanic := us.panicGuard(u.Code, u.RawURL); shouldPanic {
			panic(fmt.Sprintf("panic guard triggered for code=%s", u.Code))
		}
	}

	us.urls[u.Code] = *u

	if us.cfg.Storage.GetFlushOnWrite() {
		us.flushLocked()
	}

	return nil
}

func (us *URLStore) Get(code string) (*model.ShortURL, error) {
	us.mu.RLock()
	defer us.mu.RUnlock()

	if !us.open {
		return nil, model.ErrStorageNotOpen
	}

	u, ok := us.urls[code]
	if !ok {
		return nil, model.ErrURLNotFound
	}

	return &u, nil
}

func (us *URLStore) IncrementVisits(code string) error {
	us.mu.Lock()
	defer us.mu.Unlock()

	if !us.open {
		return model.ErrStorageNotOpen
	}

	u, ok := us.urls[code]
	if !ok {
		return model.ErrURLNotFound
	}

	u.Visits++
	us.urls[code] = u

	return nil
}

func (us *URLStore) RawSnapshot() map[string]model.ShortURL {
	us.mu.RLock()
	defer us.mu.RUnlock()

	snapshot := make(map[string]model.ShortURL, len(us.urls))
	for k, v := range us.urls {
		snapshot[k] = v
	}
	return snapshot
}

func (us *URLStore) flushLocked() {
	urlFilePath := us.cfg.Storage.GetURLFilePath()
	if urlFilePath == "" {
		urlFilePath = "data/urls.json"
	}

	dir := filepath.Dir(urlFilePath)
	_ = os.MkdirAll(dir, 0755)

	data, err := json.MarshalIndent(us.urls, "", "  ")
	if err != nil {
		us.log.Errorf("Failed to marshal URL data: %v", err)
		return
	}

	if err := os.WriteFile(urlFilePath, data, 0644); err != nil {
		us.log.Errorf("Failed to write URL file: %v", err)
	}
}

type AccessLogStore struct {
	mu     sync.Mutex
	cfg    *config.Config
	logFile *os.File
	open   bool
	log    *logger.Logger
	buffer  [][]byte
	done    chan struct{}
	wg      sync.WaitGroup
}

func NewAccessLogStore(cfg *config.Config) (*AccessLogStore, error) {
	log := logger.New(os.Stdout, logger.LevelInfo, "[AccessLogStore]")
	return &AccessLogStore{
		cfg:   cfg,
		open:  false,
		log:   log,
		buffer: make([][]byte, 0, 100),
		done:  make(chan struct{}),
	}, nil
}

func (als *AccessLogStore) Open(ctx context.Context) error {
	als.mu.Lock()
	defer als.mu.Unlock()

	if als.open {
		return nil
	}

	logFilePath := als.cfg.Storage.GetLogFilePath()
	if logFilePath == "" {
		logFilePath = "data/access.log"
	}

	dir := filepath.Dir(logFilePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	f, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}

	als.logFile = f
	als.open = true

	als.wg.Add(1)
	go als.flushLoop()

	als.log.Infof("AccessLogStore opened with log file: %s", logFilePath)
	return nil
}

func (als *AccessLogStore) Close() error {
	als.mu.Lock()
	defer als.mu.Unlock()

	if !als.open {
		return nil
	}

	als.open = false
	close(als.done)
	als.wg.Wait()

	if als.logFile != nil {
		als.logFile.Close()
	}

	als.log.Info("AccessLogStore closed")
	return nil
}

func (als *AccessLogStore) WriteEntry(entry []byte) error {
	als.mu.Lock()
	defer als.mu.Unlock()

	if !als.open {
		return model.ErrStorageNotOpen
	}

	als.buffer = append(als.buffer, entry)

	if len(als.buffer) >= 100 {
		als.flushLocked()
	}

	return nil
}

func (als *AccessLogStore) flushLoop() {
	defer als.wg.Done()

	interval := als.cfg.Storage.GetSyncInterval()
	if interval <= 0 {
		interval = 5 * time.Second
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			als.mu.Lock()
			if als.open {
				als.flushLocked()
			}
			als.mu.Unlock()
		case <-als.done:
			als.mu.Lock()
			if als.open {
				als.flushLocked()
			}
			als.mu.Unlock()
			return
		}
	}
}

func (als *AccessLogStore) flushLocked() {
	if len(als.buffer) == 0 {
		return
	}

	for _, entry := range als.buffer {
		_, _ = als.logFile.Write(entry)
	}

	als.buffer = als.buffer[:0]
}
