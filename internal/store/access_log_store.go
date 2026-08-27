package store

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ubaas/ubaas/internal/config"
)

// AccessLogEntry represents a single access log entry.
type AccessLogEntry struct {
	Code      string    `json:"code"`
	Timestamp time.Time `json:"timestamp"`
	UserAgent string    `json:"user_agent"`
	IP        string    `json:"ip"`
}

// AccessLogStore provides storage for access logs.
type AccessLogStore struct {
	mu     sync.RWMutex
	logs   []AccessLogEntry
	cfg    *config.Config
	opened bool
}

// NewAccessLogStore creates a new AccessLogStore.
func NewAccessLogStore(cfg *config.Config) (*AccessLogStore, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	return &AccessLogStore{
		logs: make([]AccessLogEntry, 0),
		cfg:  cfg,
	}, nil
}

// Open opens the access log store.
func (s *AccessLogStore) Open(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.opened = true
	return nil
}

// Close closes the access log store.
func (s *AccessLogStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.opened = false
	s.logs = make([]AccessLogEntry, 0)
	return nil
}

// LogAccess records an access log entry.
func (s *AccessLogStore) LogAccess(entry AccessLogEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.opened {
		return fmt.Errorf("access log store is not opened")
	}

	if entry.Code == "" {
		return fmt.Errorf("code is required for access log")
	}

	s.logs = append(s.logs, entry)
	return nil
}

// GetAccessCount returns the number of accesses for a given code.
func (s *AccessLogStore) GetAccessCount(code string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	count := 0
	for _, log := range s.logs {
		if log.Code == code {
			count++
		}
	}
	return count
}

// ListLogs returns all access log entries.
func (s *AccessLogStore) ListLogs() []AccessLogEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]AccessLogEntry, len(s.logs))
	copy(result, s.logs)
	return result
}
