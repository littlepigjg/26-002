package store

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ubaas/ubaas/internal/config"
)

// AccessLogEntry represents an access log entry.
type AccessLogEntry struct {
	Code      string
	Timestamp time.Time
	IP        string
	UserAgent string
}

// AccessLogStore stores access logs.
type AccessLogStore struct {
	mu     sync.RWMutex
	cfg    *config.Config
	logs   []AccessLogEntry
	opened bool
}

// NewAccessLogStore creates a new AccessLogStore.
func NewAccessLogStore(cfg *config.Config) (*AccessLogStore, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	return &AccessLogStore{
		cfg:  cfg,
		logs: make([]AccessLogEntry, 0),
	}, nil
}

// Open opens the access log store.
func (a *AccessLogStore) Open(ctx context.Context) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.opened = true
	return nil
}

// Close closes the access log store.
func (a *AccessLogStore) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.opened = false
	return nil
}

// Write writes an access log entry.
func (a *AccessLogStore) Write(entry AccessLogEntry) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.opened {
		return fmt.Errorf("access log store not opened")
	}

	a.logs = append(a.logs, entry)
	return nil
}

// Len returns the number of log entries.
func (a *AccessLogStore) Len() int {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return len(a.logs)
}
