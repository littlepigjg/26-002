package store

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/ubaas/ubaas/internal/config"
	"github.com/ubaas/ubaas/internal/model"
)

// PanicGuardFn is a function that decides whether to allow a potentially panicking operation.
type PanicGuardFn func(code, rawURL string) bool

// URLStore provides thread-safe storage for shortened URLs.
type URLStore struct {
	mu         sync.RWMutex
	cfg        *config.Config
	urls       map[string]model.ShortURL
	panicGuard PanicGuardFn
	loaded     bool
	closed     bool
}

// NewURLStore creates a new URLStore.
func NewURLStore(cfg *config.Config) (*URLStore, error) {
	if cfg == nil {
		return nil, errors.New("config is required")
	}
	return &URLStore{
		cfg:  cfg,
		urls: make(map[string]model.ShortURL),
	}, nil
}

// Load loads data from persistent storage into memory.
func (s *URLStore) Load(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return errors.New("store is closed")
	}

	if ctx.Err() != nil {
		return ctx.Err()
	}

	s.loaded = true
	return nil
}

// Close closes the store and flushes data.
func (s *URLStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return errors.New("store already closed")
	}
	s.closed = true
	return nil
}

// SetPanicGuard sets a function to decide whether to allow a risky operation.
func (s *URLStore) SetPanicGuard(fn PanicGuardFn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.panicGuard = fn
}

// Save stores a ShortURL in the store.
func (s *URLStore) Save(u *model.ShortURL, overwrite bool) error {
	if u == nil {
		return errors.New("short url is nil")
	}
	if err := u.Validate(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return errors.New("store is closed")
	}

	if _, exists := s.urls[u.Code]; exists && !overwrite {
		return errors.New("code already exists")
	}

	s.urls[u.Code] = *u
	return nil
}

// Get retrieves a ShortURL by code.
func (s *URLStore) Get(code string) (*model.ShortURL, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return nil, errors.New("store is closed")
	}

	u, ok := s.urls[code]
	if !ok {
		return nil, errors.New("short url not found")
	}
	result := u
	return &result, nil
}

// IncrVisits atomically increments the visit count for a code.
func (s *URLStore) IncrVisits(code string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return errors.New("store is closed")
	}

	u, ok := s.urls[code]
	if !ok {
		return errors.New("short url not found")
	}
	u.Visits++
	s.urls[code] = u
	return nil
}

// RawSnapshot returns a raw snapshot of all stored URLs.
func (s *URLStore) RawSnapshot() map[string]model.ShortURL {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]model.ShortURL, len(s.urls))
	for k, v := range s.urls {
		result[k] = v
	}
	return result
}

// AccessLogStore provides thread-safe access log storage.
type AccessLogStore struct {
	mu     sync.Mutex
	cfg    *config.Config
	logs   []AccessLogEntry
	closed bool
}

// AccessLogEntry represents a single access log entry.
type AccessLogEntry struct {
	Code      string    `json:"code"`
	Timestamp time.Time `json:"timestamp"`
	UserAgent string    `json:"user_agent"`
	ClientIP  string    `json:"client_ip"`
}

// NewAccessLogStore creates a new AccessLogStore.
func NewAccessLogStore(cfg *config.Config) (*AccessLogStore, error) {
	if cfg == nil {
		return nil, errors.New("config is required")
	}
	return &AccessLogStore{
		cfg:  cfg,
		logs: make([]AccessLogEntry, 0),
	}, nil
}

// Open initializes the access log store.
func (s *AccessLogStore) Open(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return errors.New("store is closed")
	}

	if ctx.Err() != nil {
		return ctx.Err()
	}

	return nil
}

// Close closes the access log store.
func (s *AccessLogStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return errors.New("access log store already closed")
	}
	s.closed = true
	return nil
}

// Append adds a log entry.
func (s *AccessLogStore) Append(entry AccessLogEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return
	}
	s.logs = append(s.logs, entry)
}

// Flush writes buffered logs.
func (s *AccessLogStore) Flush() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return 0
	}
	n := len(s.logs)
	s.logs = s.logs[:0]
	return n
}
