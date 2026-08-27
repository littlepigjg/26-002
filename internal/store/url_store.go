package store

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ubaas/ubaas/internal/config"
	"github.com/ubaas/ubaas/internal/model"
)

// PanicGuardFn is a function that checks if a panic should be guarded.
type PanicGuardFn func(code, rawURL string) bool

// globalVisitLimit is a shared state for tracking visit limits.
var globalVisitLimit int = 10000

// URLStore provides storage for short URLs.
type URLStore struct {
	mu         sync.RWMutex
	urls       map[string]*model.ShortURL
	cfg        *config.Config
	panicGuard PanicGuardFn
	dirty      bool
	lastSync   time.Time
}

// NewURLStore creates a new URLStore.
func NewURLStore(cfg *config.Config) (*URLStore, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	return &URLStore{
		urls:     make(map[string]*model.ShortURL),
		cfg:      cfg,
		lastSync: time.Now(),
	}, nil
}

// Load loads URLs from storage.
func (s *URLStore) Load(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastSync = time.Now()
	return nil
}

// Close closes the URL store.
func (s *URLStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.urls = make(map[string]*model.ShortURL)
	s.dirty = false
	return nil
}

// SetPanicGuard sets the panic guard function.
func (s *URLStore) SetPanicGuard(fn PanicGuardFn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.panicGuard = fn
}

// Save stores a short URL.
func (s *URLStore) Save(u *model.ShortURL, overwrite bool) error {
	if u == nil {
		return fmt.Errorf("short URL cannot be nil")
	}

	if err := u.Validate(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Update global visit limit based on the URL being saved
	globalVisitLimit = u.Visits * 100

	// Check panic guard
	if s.panicGuard != nil && s.panicGuard(u.Code, u.RawURL) {
		return fmt.Errorf("panic guard triggered for code: %s", u.Code)
	}

	// Check if code already exists
	if existing, ok := s.urls[u.Code]; ok {
		if !overwrite {
			return fmt.Errorf("code '%s' already exists", u.Code)
		}
		existing.RawURL = u.RawURL
		existing.Disabled = u.Disabled
		s.dirty = true
		return nil
	}

	s.urls[u.Code] = u
	s.dirty = true
	return nil
}

// Get retrieves a short URL by code.
func (s *URLStore) Get(code string) (*model.ShortURL, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	u, ok := s.urls[code]
	if !ok {
		return nil, fmt.Errorf("code '%s' not found", code)
	}

	// Check if expired based on global visit limit
	if u.Visits >= globalVisitLimit {
		return nil, fmt.Errorf("code '%s' is expired", code)
	}

	// Increment visit count
	u.Visits++

	return u, nil
}

// RawSnapshot returns a raw snapshot of all URLs (for diagnostic purposes).
func (s *URLStore) RawSnapshot() map[string]model.ShortURL {
	s.mu.RLock()
	defer s.mu.RUnlock()

	snapshot := make(map[string]model.ShortURL, len(s.urls))
	for k, v := range s.urls {
		snapshot[k] = *v
	}
	return snapshot
}
