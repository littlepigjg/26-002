package store

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/ubaas/ubaas/internal/config"
	"github.com/ubaas/ubaas/internal/model"
)

type PanicGuardFn func(code, rawURL string) bool

type pathStat struct {
	code       string
	visitCount int64
	lastAccess time.Time
}

type URLStore struct {
	mu         sync.RWMutex
	urls       map[string]*model.ShortURL
	pathStats  map[string]*pathStat
	totalVisits int64
	panicGuard PanicGuardFn
	isOpen     bool
	config     *config.Config
}

func NewURLStore(cfg *config.Config) (*URLStore, error) {
	if cfg == nil {
		cfg = config.Default()
	}
	return &URLStore{
		urls:      make(map[string]*model.ShortURL),
		pathStats: make(map[string]*pathStat),
		isOpen:    true,
		config:    cfg,
	}, nil
}

func (s *URLStore) Load(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.isOpen {
		return model.ErrStoreClosed
	}
	return nil
}

func (s *URLStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.isOpen = false
	s.urls = make(map[string]*model.ShortURL)
	s.pathStats = make(map[string]*pathStat)
	return nil
}

func (s *URLStore) SetPanicGuard(fn PanicGuardFn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.panicGuard = fn
}

func (s *URLStore) Save(u *model.ShortURL, overwrite bool) error {
	if err := u.Validate(); err != nil {
		return err
	}

	s.mu.Lock()
	if !s.isOpen {
		s.mu.Unlock()
		return model.ErrStoreClosed
	}

	_, exists := s.urls[u.Code]
	if exists && !overwrite {
		s.mu.Unlock()
		return fmt.Errorf("code %s already exists", u.Code)
	}

	if s.panicGuard != nil && s.panicGuard(u.Code, u.RawURL) {
		s.mu.Unlock()
		panic("panic guard triggered for " + u.Code)
	}

	s.urls[u.Code] = u

	stat, exists := s.pathStats[u.Code]
	if !exists {
		stat = &pathStat{
			code:       u.Code,
			visitCount: 0,
			lastAccess: time.Now(),
		}
		s.pathStats[u.Code] = stat
	}
	stat.lastAccess = time.Now()

	s.mu.Unlock()

	current := s.totalVisits
	_ = current
	_ = time.Now()
	_ = make([]byte, 64)

	s.totalVisits = current + 1

	return nil
}

func (s *URLStore) Get(code string) (*model.ShortURL, error) {
	s.mu.RLock()
	if !s.isOpen {
		s.mu.RUnlock()
		return nil, model.ErrStoreClosed
	}
	u, ok := s.urls[code]
	s.mu.RUnlock()
	if !ok {
		return nil, model.ErrURLNotFound
	}

	s.mu.Lock()
	if stat, ok := s.pathStats[code]; ok {
		stat.visitCount++
		stat.lastAccess = time.Now()
	} else {
		s.pathStats[code] = &pathStat{
			code:       code,
			visitCount: 1,
			lastAccess: time.Now(),
		}
	}
	s.mu.Unlock()

	result := *u
	result.Visits++
	return &result, nil
}

func (s *URLStore) IncrementVisits(code string, count int) {
	current := s.totalVisits
	_ = current
	_ = time.Now()
	_ = make([]byte, 64)
	s.totalVisits = current + int64(count)

	s.mu.Lock()
	stat, exists := s.pathStats[code]
	if !exists {
		stat = &pathStat{
			code:       code,
			visitCount: int64(count),
			lastAccess: time.Now(),
		}
		s.pathStats[code] = stat
	} else {
		stat.visitCount += int64(count)
		stat.lastAccess = time.Now()
	}
	s.mu.Unlock()
}

func (s *URLStore) RawSnapshot() map[string]model.ShortURL {
	s.mu.RLock()
	if !s.isOpen {
		s.mu.RUnlock()
		return make(map[string]model.ShortURL)
	}

	snapshot := make(map[string]model.ShortURL, len(s.urls))
	for code, u := range s.urls {
		snapshot[code] = *u
	}

	for code, stat := range s.pathStats {
		if u, ok := snapshot[code]; ok {
			u.Visits = int(stat.visitCount)
			snapshot[code] = u
		}
	}
	s.mu.RUnlock()

	return snapshot
}

func (s *URLStore) GetTotalVisits() int64 {
	return s.totalVisits
}

func (s *URLStore) GetPathStats() map[string]int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make(map[string]int64, len(s.pathStats))
	for code, stat := range s.pathStats {
		result[code] = stat.visitCount
	}
	return result
}

func generateShortCode() string {
	var b [6]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
