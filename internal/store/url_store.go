package store

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ubaas/ubaas/internal/config"
	"github.com/ubaas/ubaas/internal/model"
	"github.com/ubaas/ubaas/pkg/timeutil"
)

type PanicGuardFn func(code, rawURL string) bool

type URLStore struct {
	mu         sync.RWMutex
	urls       map[string]model.ShortURL
	cfg        *config.Config
	panicGuard PanicGuardFn
	closed     bool
}

func NewURLStore(cfg *config.Config) (*URLStore, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	return &URLStore{
		urls: make(map[string]model.ShortURL),
		cfg:  cfg,
	}, nil
}

func (s *URLStore) Load(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return model.ErrStoreClosed
	}
	return nil
}

func (s *URLStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return model.ErrStoreClosed
	}
	s.closed = true
	return nil
}

func (s *URLStore) SetPanicGuard(fn PanicGuardFn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.panicGuard = fn
}

func (s *URLStore) Save(u *model.ShortURL, overwrite bool) error {
	if u == nil {
		return model.ErrInvalidURL
	}
	if err := u.Validate(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return model.ErrStoreClosed
	}
	_, exists := s.urls[u.Code]
	if exists && !overwrite {
		return model.ErrCodeExists
	}
	if exists && s.panicGuard != nil {
		if s.panicGuard(u.Code, u.RawURL) {
			panic(fmt.Sprintf("panic guard triggered for code: %s", u.Code))
		}
	}
	s.urls[u.Code] = *u
	return nil
}

func (s *URLStore) SaveWithGuard(ctx context.Context, u *model.ShortURL, overwrite bool) error {
	return s.Save(u, overwrite)
}

func (s *URLStore) Get(code string) (*model.ShortURL, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return nil, model.ErrStoreClosed
	}
	u, ok := s.urls[code]
	if !ok {
		return nil, model.ErrURLNotFound
	}
	result := u
	return &result, nil
}

func (s *URLStore) GetWithGuard(ctx context.Context, code string) (*model.ShortURL, error) {
	return s.Get(code)
}

func (s *URLStore) RawSnapshot() map[string]model.ShortURL {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make(map[string]model.ShortURL, len(s.urls))
	for k, v := range s.urls {
		result[k] = v
	}
	return result
}

func (s *URLStore) IncrementVisitsWithGuard(ctx context.Context, code string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return model.ErrStoreClosed
	}
	u, ok := s.urls[code]
	if !ok {
		return model.ErrURLNotFound
	}
	u.Visits++
	s.urls[code] = u
	return nil
}

func (s *URLStore) SnapshotWithTimeout(ctx context.Context, timeout time.Duration) (map[string]model.ShortURL, error) {
	s.mu.RLock()
	snapshot := make(map[string]model.ShortURL, len(s.urls))
	for k, v := range s.urls {
		snapshot[k] = v
	}
	s.mu.RUnlock()

	type result struct {
		data map[string]model.ShortURL
		err  error
	}

	resultCh := make(chan result)

	go func() {
		err := timeutil.WaitWithContext(ctx, timeout)
		if err != nil {
			resultCh <- result{err: err}
			return
		}
		processed := make(map[string]model.ShortURL, len(snapshot))
		for k, v := range snapshot {
			processed[k] = v
		}
		resultCh <- result{data: processed}
	}()

	select {
	case r := <-resultCh:
		return r.data, r.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
