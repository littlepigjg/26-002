package store

import (
	"context"
	"sync"
	"time"

	"github.com/ubaas/ubaas/internal/config"
	"github.com/ubaas/ubaas/internal/model"
)

type PanicGuardFn func(code, rawURL string) bool

type URLStore struct {
	mu      sync.RWMutex
	cfg     *config.Config
	entries map[string]model.ShortURL
	closed  bool
	guard   PanicGuardFn
}

func NewURLStore(cfg *config.Config) (*URLStore, error) {
	if cfg == nil {
		cfg = config.Default()
	}
	return &URLStore{
		cfg:     cfg,
		entries: make(map[string]model.ShortURL),
	}, nil
}

func (s *URLStore) Load(ctx context.Context) error {
	if _, ok := ctx.Deadline(); !ok {
		return model.ErrInvalidRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return model.ErrStoreClosed
	}
	s.entries = make(map[string]model.ShortURL)
	return nil
}

func (s *URLStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	return nil
}

func (s *URLStore) SetPanicGuard(fn PanicGuardFn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.guard = fn
}

func (s *URLStore) Save(u *model.ShortURL, overwrite bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return model.ErrStoreClosed
	}
	if u == nil {
		return model.ErrInvalidRequest
	}
	if err := u.Validate(); err != nil {
		return err
	}
	if !overwrite {
		if _, ok := s.entries[u.Code]; ok {
			return model.ErrInvalidRequest
		}
	}
	s.entries[u.Code] = *u
	return nil
}

func (s *URLStore) Get(code string) (*model.ShortURL, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return nil, model.ErrStoreClosed
	}
	e, ok := s.entries[code]
	if !ok {
		return nil, model.ErrEventNotFound
	}
	return &e, nil
}

func (s *URLStore) RawSnapshot() map[string]model.ShortURL {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.entries) == 0 {
		return nil
	}
	snap := make(map[string]model.ShortURL, len(s.entries))
	for k, v := range s.entries {
		snap[k] = v
	}
	return snap
}

type AccessLogStore struct {
	mu     sync.Mutex
	cfg    *config.Config
	closed bool
	events []time.Time
}

func NewAccessLogStore(cfg *config.Config) (*AccessLogStore, error) {
	if cfg == nil {
		cfg = config.Default()
	}
	return &AccessLogStore{
		cfg:    cfg,
		events: make([]time.Time, 0),
	}, nil
}

func (s *AccessLogStore) Open(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = false
	return nil
}

func (s *AccessLogStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	return nil
}

func (s *AccessLogStore) Append(t time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.closed {
		s.events = append(s.events, t)
	}
}

func (s *AccessLogStore) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.events)
}