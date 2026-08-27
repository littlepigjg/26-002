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
	mu          sync.RWMutex
	cfg         *config.Config
	urls        map[string]*model.ShortURL
	panicGuard  PanicGuardFn
	isOpen      bool
}

type AccessLogStore struct {
	mu     sync.Mutex
	cfg    *config.Config
	logs   []AccessLogEntry
	isOpen bool
}

type AccessLogEntry struct {
	Code      string
	Timestamp time.Time
	Status    int
	UserAgent string
	IP        string
}

func NewURLStore(cfg *config.Config) (*URLStore, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is required")
	}
	return &URLStore{
		cfg:    cfg,
		urls:   make(map[string]*model.ShortURL),
		isOpen: true,
	}, nil
}

func NewAccessLogStore(cfg *config.Config) (*AccessLogStore, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is required")
	}
	return &AccessLogStore{
		cfg:    cfg,
		logs:   make([]AccessLogEntry, 0),
		isOpen: true,
	}, nil
}

func (s *URLStore) Load(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.isOpen {
		return fmt.Errorf("store is closed")
	}
	s.urls = make(map[string]*model.ShortURL)
	return nil
}

func (s *URLStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.isOpen = false
	return nil
}

func (s *URLStore) SetPanicGuard(fn PanicGuardFn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.panicGuard = fn
}

func (s *URLStore) Save(u *model.ShortURL, overwrite bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.isOpen {
		return fmt.Errorf("store is closed")
	}
	if u == nil {
		return fmt.Errorf("short URL is nil")
	}
	if !overwrite {
		if _, exists := s.urls[u.Code]; exists {
			return fmt.Errorf("code already exists: %s", u.Code)
		}
	}
	if u.CreatedAt.IsZero() {
		u.CreatedAt = timeutil.GetCurrentTime()
	}
	s.urls[u.Code] = u
	return nil
}

func (s *URLStore) Get(code string) (*model.ShortURL, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.isOpen {
		return nil, fmt.Errorf("store is closed")
	}
	u, ok := s.urls[code]
	if !ok {
		return nil, fmt.Errorf("url not found: %s", code)
	}
	return u, nil
}

func (s *URLStore) RawSnapshot() map[string]model.ShortURL {
	s.mu.RLock()
	defer s.mu.RUnlock()
	snapshot := make(map[string]model.ShortURL, len(s.urls))
	for code, u := range s.urls {
		snapshot[code] = *u
	}
	return snapshot
}

func (l *AccessLogStore) Open(ctx context.Context) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.isOpen {
		return fmt.Errorf("log store is closed")
	}
	l.logs = make([]AccessLogEntry, 0)
	return nil
}

func (l *AccessLogStore) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.isOpen = false
	return nil
}

func (l *AccessLogStore) Record(code string, status int, userAgent, ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.isOpen {
		return
	}
	l.logs = append(l.logs, AccessLogEntry{
		Code:      code,
		Timestamp: time.Now(),
		Status:    status,
		UserAgent: userAgent,
		IP:        ip,
	})
}

func (l *AccessLogStore) Logs() []AccessLogEntry {
	l.mu.Lock()
	defer l.mu.Unlock()
	result := make([]AccessLogEntry, len(l.logs))
	copy(result, l.logs)
	return result
}
