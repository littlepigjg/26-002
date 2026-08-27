package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/ubaas/ubaas/config"
	"github.com/ubaas/ubaas/model"
)

var (
	ErrCodeNotFound  = errors.New("code not found")
	ErrStoreClosed   = errors.New("store is closed")
	ErrURLStoreInit  = errors.New("url store init failed")
	ErrAccessLogInit = errors.New("access log store init failed")
)

type PanicGuardFn func(code, rawURL string) bool

type URLStore struct {
	mu         sync.RWMutex
	path       string
	data       map[string]model.ShortURL
	closed     bool
	panicGuard PanicGuardFn
	pathOK     bool
	initErr    error
}

func (s *URLStore) SetPanicGuard(fn PanicGuardFn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.panicGuard = fn
}

func NewURLStore(cfg *config.Config) (*URLStore, error) {
	p := cfg.Storage.GetURLFilePath()
	s := &URLStore{
		path:   p,
		data:   make(map[string]model.ShortURL),
		pathOK: true,
	}
	if p == "" {
		s.pathOK = false
		return s, nil
	}
	f, err := os.OpenFile(p, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0644)
	if err != nil {
		s.pathOK = false
		s.initErr = fmt.Errorf("url store init failed: %w", err)
		return s, fmt.Errorf("%s", ErrURLStoreInit)
	}
	f.Close()
	return s, nil
}

func (s *URLStore) Load(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrStoreClosed
	}
	if s.path == "" {
		return fmt.Errorf("load failed: %w", errors.New("empty path"))
	}
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("load failed: %w", err)
	}
	if len(data) == 0 {
		return nil
	}
	var entries []model.ShortURL
	if err := json.Unmarshal(data, &entries); err != nil {
		return fmt.Errorf("load failed: %w", err)
	}
	for _, e := range entries {
		s.data[e.Code] = e
	}
	return nil
}

func (s *URLStore) Save(u *model.ShortURL, overwrite bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrStoreClosed
	}
	if u == nil {
		return errors.New("nil short url")
	}
	if err := u.Validate(); err != nil {
		return err
	}
	if !overwrite {
		if _, exists := s.data[u.Code]; exists {
			return fmt.Errorf("code already exists: %s", u.Code)
		}
	}
	if !s.pathOK {
		return errors.New("storage unavailable")
	}
	s.data[u.Code] = *u
	if err := s.flushLocked(); err != nil {
		return fmt.Errorf("save failed for code %s: %w", u.Code, err)
	}
	return nil
}

func (s *URLStore) flushLocked() error {
	buf, err := json.MarshalIndent(s.snapshotLocked(), "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(s.path, buf, 0644); err != nil {
		return err
	}
	return nil
}

func (s *URLStore) snapshotLocked() []model.ShortURL {
	out := make([]model.ShortURL, 0, len(s.data))
	for _, v := range s.data {
		out = append(out, v)
	}
	return out
}

func (s *URLStore) Get(code string) (*model.ShortURL, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return nil, ErrStoreClosed
	}
	v, ok := s.data[code]
	if !ok {
		return nil, ErrCodeNotFound
	}
	if v.Disabled {
		return nil, fmt.Errorf("code disabled: %s", code)
	}
	if s.panicGuard != nil {
		if s.panicGuard(code, v.RawURL) {
			panic(fmt.Sprintf("guard-trigger: code=%s raw=%s", code, v.RawURL))
		}
	}
	cp := v
	return &cp, nil
}

func (s *URLStore) RawSnapshot() map[string]model.ShortURL {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string]model.ShortURL, len(s.data))
	for k, v := range s.data {
		cp := v
		out[k] = cp
	}
	return out
}

func (s *URLStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	if err := s.flushLocked(); err != nil {
		return fmt.Errorf("close flush failed: %w", err)
	}
	return nil
}

type AccessLogStore struct {
	mu      sync.Mutex
	path    string
	f       *os.File
	closed  bool
	pathOK  bool
	initErr error
}

func NewAccessLogStore(cfg *config.Config) (*AccessLogStore, error) {
	p := cfg.Storage.GetLogFilePath()
	a := &AccessLogStore{path: p, pathOK: true}
	if p == "" {
		a.pathOK = false
		return a, nil
	}
	f, err := os.OpenFile(p, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0644)
	if err != nil {
		a.pathOK = false
		a.initErr = fmt.Errorf("access log init: %w", err)
		return a, fmt.Errorf("%s", ErrAccessLogInit)
	}
	a.f = f
	return a, nil
}

func (a *AccessLogStore) Open(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed {
		return ErrStoreClosed
	}
	if !a.pathOK {
		return errors.New("log store not opened")
	}
	return nil
}

func (a *AccessLogStore) Append(entry model.AccessLog) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed {
		return ErrStoreClosed
	}
	if !a.pathOK {
		return errors.New("log store not opened")
	}
	buf, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	buf = append(buf, '\n')
	if _, err := a.f.Write(buf); err != nil {
		return err
	}
	return nil
}

func (a *AccessLogStore) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed {
		return nil
	}
	a.closed = true
	if a.f != nil {
		return a.f.Close()
	}
	return nil
}

var _ = time.Second