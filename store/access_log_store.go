package store

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/ubaas/ubaas/config"
)

type AccessLogEntry struct {
	Code      string    `json:"code"`
	RawURL    string    `json:"raw_url"`
	Timestamp time.Time `json:"timestamp"`
	Referrer  string    `json:"referrer,omitempty"`
	UserAgent string    `json:"user_agent,omitempty"`
	IP        string    `json:"ip,omitempty"`
}

type AccessLogStore struct {
	cfg     *config.Config
	mu      sync.Mutex
	file    *os.File
	closed  bool
	logCh   chan AccessLogEntry
	stopCh  chan struct{}
	wg      sync.WaitGroup
}

func NewAccessLogStore(cfg *config.Config) (*AccessLogStore, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is required")
	}

	return &AccessLogStore{
		cfg:    cfg,
		logCh:  make(chan AccessLogEntry, 500),
		stopCh: make(chan struct{}),
	}, nil
}

func (a *AccessLogStore) Open(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.closed {
		return fmt.Errorf("store is closed")
	}

	logFilePath := a.cfg.Storage.GetLogFilePath()
	if logFilePath == "" {
		return fmt.Errorf("log file path not configured")
	}

	dir := filepath.Dir(logFilePath)
	if dir != "" {
		_ = os.MkdirAll(dir, 0755)
	}

	f, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}

	a.file = f

	a.wg.Add(1)
	go a.writeLoop()

	return nil
}

func (a *AccessLogStore) Close() error {
	a.mu.Lock()
	if a.closed {
		a.mu.Unlock()
		return nil
	}
	a.closed = true
	close(a.stopCh)
	a.mu.Unlock()

	a.wg.Wait()

	if a.file != nil {
		return a.file.Close()
	}
	return nil
}

func (a *AccessLogStore) Log(entry AccessLogEntry) {
	a.mu.Lock()
	if a.closed {
		a.mu.Unlock()
		return
	}
	a.mu.Unlock()

	select {
	case a.logCh <- entry:
	default:
	}
}

func (a *AccessLogStore) writeLoop() {
	defer a.wg.Done()

	for {
		select {
		case <-a.stopCh:
			return
		case entry := <-a.logCh:
			a.writeEntry(entry)
		}
	}
}

func (a *AccessLogStore) writeEntry(entry AccessLogEntry) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.file == nil {
		return
	}

	line := fmt.Sprintf("%s\t%s\t%s\t%s\t%s\n",
		entry.Timestamp.Format(time.RFC3339),
		entry.Code,
		entry.RawURL,
		entry.Referrer,
		entry.IP,
	)
	_, _ = a.file.WriteString(line)
}
