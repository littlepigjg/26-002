package store

import (
	"context"
	"sync"
	"time"

	"github.com/ubaas/ubaas/internal/config"
)

type AccessLogEntry struct {
	Code      string
	RawURL    string
	Timestamp time.Time
	UserAgent string
	IP        string
}

type AccessLogStore struct {
	mu      sync.RWMutex
	entries []AccessLogEntry
	cfg     *config.Config
	closed  bool
}

func NewAccessLogStore(cfg *config.Config) (*AccessLogStore, error) {
	if cfg == nil {
		return nil, nil
	}
	return &AccessLogStore{
		entries: make([]AccessLogEntry, 0),
		cfg:     cfg,
	}, nil
}

func (s *AccessLogStore) Open(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		s.closed = false
	}
	return nil
}

func (s *AccessLogStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	return nil
}

func (s *AccessLogStore) Log(entry AccessLogEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	s.entries = append(s.entries, entry)
}

func (s *AccessLogStore) Entries() []AccessLogEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]AccessLogEntry, len(s.entries))
	copy(result, s.entries)
	return result
}
