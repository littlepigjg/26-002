package store

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ubaas/ubaas/internal/config"
	"github.com/ubaas/ubaas/internal/model"
)

type URLStore struct {
	cfg        config.Config
	mu         sync.RWMutex
	data       map[string]model.ShortURL
	panicGuard model.PanicGuardFn
	loaded     bool
}

func NewURLStore(cfg *config.Config) (*URLStore, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}
	snapshot := cfg.Get()
	return &URLStore{
		cfg:  snapshot,
		data: make(map[string]model.ShortURL),
	}, nil
}

func (s *URLStore) Load(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if ctx.Err() != nil {
		return ctx.Err()
	}

	idx := s.cfg.Storage.GetURLIndex()
	for code, rawURL := range idx {
		s.data[code] = model.ShortURL{
			Code:      code,
			RawURL:    rawURL,
			CreatedAt: time.Now(),
			Visits:    0,
			Custom:    false,
			Disabled:  false,
		}
	}
	s.loaded = true
	return nil
}

func (s *URLStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cfg.Storage.GetFlushOnWrite() {
		idx := make(map[string]string)
		for code, u := range s.data {
			idx[code] = u.RawURL
		}
		s.cfg.Storage.SetURLIndex(idx)
	}
	return nil
}

func (s *URLStore) SetPanicGuard(fn model.PanicGuardFn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.panicGuard = fn
}

func (s *URLStore) Save(u *model.ShortURL, overwrite bool) error {
	if u == nil {
		return fmt.Errorf("short_url is nil")
	}
	if err := u.Validate(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.data[u.Code]; exists && !overwrite {
		return fmt.Errorf("code %s already exists", u.Code)
	}

	if s.panicGuard != nil && s.panicGuard(u.Code, u.RawURL) {
		return fmt.Errorf("panic guard triggered for %s", u.Code)
	}

	s.data[u.Code] = *u

	idx := s.cfg.Storage.GetURLIndex()
	if idx == nil {
		idx = make(map[string]string)
	}
	idx[u.Code] = u.RawURL
	s.cfg.Storage.SetURLIndex(idx)

	return nil
}

func (s *URLStore) Get(code string) (*model.ShortURL, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	u, ok := s.data[code]
	if !ok {
		return nil, fmt.Errorf("code %s not found", code)
	}

	result := u
	return &result, nil
}

func (s *URLStore) RawSnapshot() map[string]model.ShortURL {
	s.mu.RLock()
	defer s.mu.RUnlock()

	snapshot := make(map[string]model.ShortURL, len(s.data))
	for k, v := range s.data {
		snapshot[k] = v
	}
	return snapshot
}
