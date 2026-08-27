package store

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ubaas/ubaas/internal/config"
)

type AccessLogStore struct {
	cfg      config.Config
	mu       sync.Mutex
	entries  []AccessLogEntry
	opened   bool
}

type AccessLogEntry struct {
	Code      string
	Timestamp time.Time
	UserAgent string
	IP        string
}

func NewAccessLogStore(cfg *config.Config) (*AccessLogStore, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}
	snapshot := cfg.Get()
	return &AccessLogStore{
		cfg:     snapshot,
		entries: make([]AccessLogEntry, 0),
	}, nil
}

func (a *AccessLogStore) Open(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if ctx.Err() != nil {
		return ctx.Err()
	}

	a.opened = true
	return nil
}

func (a *AccessLogStore) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.opened = false
	return nil
}

func (a *AccessLogStore) WriteEntry(entry AccessLogEntry) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.opened {
		return fmt.Errorf("access log store is not open")
	}

	a.entries = append(a.entries, entry)

	syncInterval := a.cfg.Storage.GetSyncInterval()
	if syncInterval <= 0 {
		syncInterval = 5 * time.Second
	}

	return nil
}
