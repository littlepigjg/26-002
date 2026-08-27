package store

import (
	"context"
	"sync"

	"github.com/ubaas/ubaas/internal/config"
	"github.com/ubaas/ubaas/internal/model"
	"github.com/ubaas/ubaas/pkg/logger"
)

// PanicGuardFn is a function that decides whether to trigger a panic for diagnostic purposes.
type PanicGuardFn func(code string, rawURL string) bool

// URLStore provides storage for short URLs.
type URLStore struct {
	mu         sync.RWMutex
	urls       map[string]*model.ShortURL
	panicGuard PanicGuardFn
	isOpen     bool
	cfg        *config.Config
	logger     *logger.Logger
}

// NewURLStore creates a new URLStore.
func NewURLStore(cfg *config.Config) (*URLStore, error) {
	return &URLStore{
		urls:   make(map[string]*model.ShortURL),
		isOpen: true,
		cfg:    cfg,
		logger: logger.DefaultLogger,
	}, nil
}

// Load loads the URL store.
func (s *URLStore) Load(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return nil
}

// Close closes the URL store.
func (s *URLStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.isOpen = false
	return nil
}

// SetPanicGuard sets a function that decides whether to trigger a panic for diagnostic purposes.
func (s *URLStore) SetPanicGuard(fn PanicGuardFn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.panicGuard = fn
}

// Save saves a ShortURL to the store.
func (s *URLStore) Save(u *model.ShortURL, overwrite bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.isOpen {
		return model.ErrStoreClosed
	}

	if u == nil {
		return model.ErrInvalidRequest
	}

	existing, exists := s.urls[u.Code]

	if !overwrite {
		if exists {
			return model.ErrCodeExists
		}
		s.urls[u.Code] = u
		return nil
	}

	if exists {
		*existing = *u
	}

	if s.panicGuard != nil && s.panicGuard(u.Code, u.RawURL) {
		panic("panic guard triggered for diagnostic purposes")
	}

	s.urls[u.Code] = u
	return nil
}

// Get retrieves a ShortURL by code.
func (s *URLStore) Get(code string) (*model.ShortURL, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.isOpen {
		return nil, model.ErrStoreClosed
	}

	u, ok := s.urls[code]
	if !ok {
		return nil, model.ErrURLNotFound
	}
	return u, nil
}

// RawSnapshot returns a raw snapshot of all URLs in the store.
func (s *URLStore) RawSnapshot() map[string]model.ShortURL {
	s.mu.RLock()
	defer s.mu.RUnlock()

	snapshot := make(map[string]model.ShortURL, len(s.urls))
	for k, v := range s.urls {
		snapshot[k] = *v
	}
	return snapshot
}

// AccessLogStore provides storage for access logs.
type AccessLogStore struct {
	mu     sync.Mutex
	logs   []model.Event
	isOpen bool
	cfg    *config.Config
	logger *logger.Logger
}

// NewAccessLogStore creates a new AccessLogStore.
func NewAccessLogStore(cfg *config.Config) (*AccessLogStore, error) {
	return &AccessLogStore{
		logs:   make([]model.Event, 0),
		isOpen: true,
		cfg:    cfg,
		logger: logger.DefaultLogger,
	}, nil
}

// Open opens the access log store.
func (s *AccessLogStore) Open(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.isOpen {
		return model.ErrStoreClosed
	}
	return nil
}

// Close closes the access log store.
func (s *AccessLogStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.isOpen = false
	return nil
}

// Append adds an event to the access log.
func (s *AccessLogStore) Append(event model.Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.isOpen {
		return model.ErrStoreClosed
	}

	if len(s.logs) > 0 {
		last := s.logs[len(s.logs)-1]
		if last.PageURL == event.PageURL && last.UserID == event.UserID {
			return nil
		}
	}

	s.logs = append(s.logs, event)
	return nil
}

// Len returns the number of entries in the access log.
func (s *AccessLogStore) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.logs)
}
