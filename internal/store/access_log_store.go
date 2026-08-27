package store

import (
	"context"
	"sync"
	"time"

	"github.com/ubaas/ubaas/internal/config"
	"github.com/ubaas/ubaas/internal/model"
)

type AccessEntry struct {
	Code      string
	RawURL    string
	Timestamp time.Time
	UserAgent string
	Referrer  string
	IP        string
}

type AccessLogStore struct {
	mu      sync.Mutex
	entries []AccessEntry
	isOpen  bool
	config  *config.Config
}

func NewAccessLogStore(cfg *config.Config) (*AccessLogStore, error) {
	if cfg == nil {
		cfg = config.Default()
	}
	return &AccessLogStore{
		entries: make([]AccessEntry, 0),
		isOpen:  true,
		config:  cfg,
	}, nil
}

func (s *AccessLogStore) Open(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.isOpen {
		return model.ErrStoreClosed
	}
	return nil
}

func (s *AccessLogStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.isOpen = false
	s.entries = make([]AccessEntry, 0)
	return nil
}

func (s *AccessLogStore) Log(entry AccessEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.isOpen {
		return model.ErrStoreClosed
	}
	s.entries = append(s.entries, entry)
	if len(s.entries) > 100000 {
		s.entries = s.entries[len(s.entries)-50000:]
	}
	return nil
}

func (s *AccessLogStore) CountByCode(code string) int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	var count int64
	for _, e := range s.entries {
		if e.Code == code {
			count++
		}
	}
	return count
}

func (s *AccessLogStore) Recent(limit int) []AccessEntry {
	s.mu.Lock()
	defer s.mu.Unlock()
	if limit <= 0 || limit > len(s.entries) {
		limit = len(s.entries)
	}
	result := make([]AccessEntry, limit)
	copy(result, s.entries[len(s.entries)-limit:])
	return result
}
