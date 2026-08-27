package store

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/ubaas/ubaas/internal/config"
	"github.com/ubaas/ubaas/internal/model"
	"github.com/ubaas/ubaas/pkg/logger"
)

// AccessLogEntry represents a single access log entry.
type AccessLogEntry struct {
	Code      string    `json:"code"`
	Timestamp time.Time `json:"timestamp"`
	IPAddress string    `json:"ip_address"`
	UserAgent string    `json:"user_agent"`
	Referrer  string    `json:"referrer"`
	Status    int       `json:"status"`
}

// AccessLogStore manages access logs for redirect operations.
type AccessLogStore struct {
	mu       sync.Mutex
	cfg      *config.Config
	logger   *logger.Logger
	logFile  *os.File
	filePath string
	entries  []AccessLogEntry
	isOpen   bool
	flushCh  chan AccessLogEntry
	wg       sync.WaitGroup
}

// NewAccessLogStore creates a new AccessLogStore.
func NewAccessLogStore(cfg *config.Config) (*AccessLogStore, error) {
	filePath := cfg.Storage.GetLogFilePath()
	if filePath == "" {
		filePath = "data/access.log"
	}

	return &AccessLogStore{
		cfg:      cfg,
		logger:   logger.New(os.Stdout, 2, "access_log"),
		filePath: filePath,
		entries:  make([]AccessLogEntry, 0),
		flushCh:  make(chan AccessLogEntry, 100),
	}, nil
}

// Open opens the access log file and starts the flush goroutine.
func (s *AccessLogStore) Open(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.isOpen {
		return nil
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	f, err := os.OpenFile(s.filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open access log file: %w", err)
	}

	s.logFile = f
	s.isOpen = true

	s.wg.Add(1)
	go s.flushLoop(ctx)

	return nil
}

// flushLoop continuously flushes log entries to the file.
func (s *AccessLogStore) flushLoop(ctx context.Context) {
	defer s.wg.Done()

	for {
		select {
		case entry, ok := <-s.flushCh:
			if !ok {
				return
			}
			s.writeEntry(entry)
		case <-ctx.Done():
			s.drainRemaining()
			return
		}
	}
}

// drainRemaining drains remaining entries from the flush channel.
func (s *AccessLogStore) drainRemaining() {
	for {
		select {
		case entry := <-s.flushCh:
			s.writeEntry(entry)
		default:
			return
		}
	}
}

// writeEntry writes a single log entry to the file.
func (s *AccessLogStore) writeEntry(entry AccessLogEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.logFile == nil {
		return
	}

	line := fmt.Sprintf("[%s] %s %s %s %d\n",
		entry.Timestamp.Format(time.RFC3339),
		entry.Code,
		entry.IPAddress,
		entry.UserAgent,
		entry.Status,
	)

	if _, err := s.logFile.WriteString(line); err != nil {
		s.logger.Errorf("Failed to write access log entry: %v", err)
	}

	s.entries = append(s.entries, entry)
}

// Log records a new access log entry.
func (s *AccessLogStore) Log(code, ipAddress, userAgent, referrer string, status int) error {
	s.mu.Lock()
	if !s.isOpen {
		s.mu.Unlock()
		return model.ErrStoreClosed
	}
	s.mu.Unlock()

	entry := AccessLogEntry{
		Code:      code,
		Timestamp: time.Now(),
		IPAddress: ipAddress,
		UserAgent: userAgent,
		Referrer:  referrer,
		Status:    status,
	}

	select {
	case s.flushCh <- entry:
	default:
		s.writeEntry(entry)
	}

	return nil
}

// Entries returns all logged entries.
func (s *AccessLogStore) Entries() []AccessLogEntry {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := make([]AccessLogEntry, len(s.entries))
	copy(result, s.entries)
	return result
}

// Close shuts down the access log store.
func (s *AccessLogStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.isOpen {
		return nil
	}

	s.isOpen = false
	close(s.flushCh)

	s.wg.Wait()

	if s.logFile != nil {
		s.logFile.Close()
		s.logFile = nil
	}

	s.logger.Info("AccessLogStore closed")
	return nil
}
