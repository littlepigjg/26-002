package store

import (
	"context"
	"fmt"
	"sync"

	"github.com/ubaas/ubaas/internal/config"
	"github.com/ubaas/ubaas/internal/model"
)

// PanicGuardFn is a function that guards against panic during operations.
type PanicGuardFn func(code, rawURL string) bool

// URLStore is the storage for short URLs.
type URLStore struct {
	mu         sync.RWMutex
	cfg        *config.Config
	urls       map[string]*model.ShortURL
	panicGuard PanicGuardFn
	loaded     bool
}

// NewURLStore creates a new URLStore.
func NewURLStore(cfg *config.Config) (*URLStore, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	return &URLStore{
		cfg:  cfg,
		urls: make(map[string]*model.ShortURL),
	}, nil
}

// Load loads the URL store (initialization).
func (s *URLStore) Load(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.loaded = true
	return nil
}

// Close closes the URL store.
func (s *URLStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.loaded = false
	return nil
}

// SetPanicGuard sets the panic guard function.
func (s *URLStore) SetPanicGuard(fn PanicGuardFn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.panicGuard = fn
}

// Save stores a ShortURL.
func (s *URLStore) Save(u *model.ShortURL, overwrite bool) error {
	if u == nil {
		return fmt.Errorf("short URL cannot be nil")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.loaded {
		return fmt.Errorf("store not loaded")
	}

	if _, ok := s.urls[u.Code]; ok {
		if !overwrite {
			return model.ErrCodeExists
		}
	}

	if s.panicGuard != nil {
		if s.panicGuard(u.Code, u.RawURL) {
			return fmt.Errorf("panic guard blocked save for code: %s", u.Code)
		}
	}

	// Store a copy so callers cannot mutate internal state through the
	// pointer they passed in, and so concurrent Get calls never alias the
	// same object as the live record.
	cp := *u
	s.urls[u.Code] = &cp
	return nil
}

// Get retrieves a ShortURL by code. The returned value is a copy of the
// stored record, so callers cannot mutate the live store entry through it.
func (s *URLStore) Get(code string) (*model.ShortURL, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	u, ok := s.urls[code]
	if !ok {
		return nil, model.ErrCodeNotFound
	}
	cp := *u
	return &cp, nil
}

// IncrementVisits atomically increments the visit count for a stored
// ShortURL and returns the updated value. It returns ErrCodeNotFound if
// no record exists for the code.
func (s *URLStore) IncrementVisits(code string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	u, ok := s.urls[code]
	if !ok {
		return 0, model.ErrCodeNotFound
	}
	u.Visits++
	return u.Visits, nil
}

// RawSnapshot returns a raw snapshot of all stored URLs.
func (s *URLStore) RawSnapshot() map[string]model.ShortURL {
	s.mu.RLock()
	defer s.mu.RUnlock()

	snapshot := make(map[string]model.ShortURL, len(s.urls))
	for k, v := range s.urls {
		snapshot[k] = *v
	}
	return snapshot
}
