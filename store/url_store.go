package store

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/ubaas/ubaas/config"
	"github.com/ubaas/ubaas/model"
	"github.com/ubaas/ubaas/pkg/cache"
)

type PanicGuardFn func(code, rawURL string) bool

type URLStore struct {
	mu         sync.RWMutex
	cfg        *config.Config
	cache      *cache.LRUCache
	items      map[string]model.ShortURL
	guard      PanicGuardFn
	closed     bool
	saveCh     chan model.ShortURL
	stopCh     chan struct{}
	wg         sync.WaitGroup
}

func NewURLStore(cfg *config.Config) (*URLStore, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is required")
	}

	maxItems := cfg.Storage.GetCacheMaxSize()
	if maxItems <= 0 {
		maxItems = 1000
	}

	s := &URLStore{
		cfg:    cfg,
		cache:  cache.NewLRUCache(maxItems),
		items:  make(map[string]model.ShortURL),
		saveCh: make(chan model.ShortURL, 100),
		stopCh: make(chan struct{}),
	}

	s.wg.Add(1)
	go s.flushLoop()

	return s, nil
}

func (s *URLStore) SetPanicGuard(fn PanicGuardFn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.guard = fn
}

func (s *URLStore) Load(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return fmt.Errorf("store is closed")
	}

	urlFilePath := s.cfg.Storage.GetURLFilePath()
	if urlFilePath == "" {
		return nil
	}

	data, err := os.ReadFile(urlFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read url file: %w", err)
	}

	var items map[string]model.ShortURL
	if err := json.Unmarshal(data, &items); err != nil {
		return fmt.Errorf("failed to parse url file: %w", err)
	}

	for code, item := range items {
		s.items[code] = item
		cacheTTL := 10 * time.Minute
		if item.IsExpired(time.Now()) {
			cacheTTL = 1 * time.Nanosecond
		}
		s.cache.Set(code, item, cacheTTL)
	}

	return nil
}

func (s *URLStore) Save(u *model.ShortURL, overwrite bool) error {
	s.mu.Lock()

	if s.closed {
		s.mu.Unlock()
		return fmt.Errorf("store is closed")
	}

	if u == nil {
		s.mu.Unlock()
		return fmt.Errorf("short url is nil")
	}

	_, exists := s.items[u.Code]
	if exists && !overwrite {
		s.mu.Unlock()
		return fmt.Errorf("code %s already exists", u.Code)
	}

	if exists && s.guard != nil {
		if s.guard(u.Code, u.RawURL) {
			s.mu.Unlock()
			return fmt.Errorf("panic guard triggered for %s", u.Code)
		}
	}

	s.items[u.Code] = *u
	cacheTTL := 10 * time.Minute
	if u.IsExpired(time.Now()) {
		cacheTTL = 1 * time.Nanosecond
	}
	s.cache.Set(u.Code, *u, cacheTTL)

	if s.cfg.Storage.GetFlushOnWrite() {
		save := *u
		s.mu.Unlock()
		select {
		case s.saveCh <- save:
		default:
		}
		return nil
	}

	s.mu.Unlock()
	return nil
}

func (s *URLStore) SaveWithGuard(u *model.ShortURL, overwrite bool) error {
	s.mu.Lock()

	if s.closed {
		s.mu.Unlock()
		return fmt.Errorf("store is closed")
	}

	if u == nil {
		s.mu.Unlock()
		return fmt.Errorf("short url is nil")
	}

	if s.guard != nil && s.guard(u.Code, u.RawURL) {
		s.mu.Unlock()
		return fmt.Errorf("guard check failed for %s", u.Code)
	}

	_, exists := s.items[u.Code]
	if exists && !overwrite {
		s.mu.Unlock()
		return fmt.Errorf("code %s already exists", u.Code)
	}

	s.items[u.Code] = *u
	cacheTTL := 10 * time.Minute
	if u.IsExpired(time.Now()) {
		cacheTTL = 1 * time.Nanosecond
	}
	s.cache.SetWithEviction(u.Code, *u, cacheTTL)

	if s.cfg.Storage.GetFlushOnWrite() {
		save := *u
		s.mu.Unlock()
		select {
		case s.saveCh <- save:
		default:
		}
		return nil
	}

	s.mu.Unlock()
	return nil
}

func (s *URLStore) Get(code string) (*model.ShortURL, error) {
	if code == "" {
		return nil, fmt.Errorf("code is required")
	}

	if cached, ok := s.cache.Get(code); ok {
		if u, ok := cached.(model.ShortURL); ok {
			result := u
			return &result, nil
		}
	}

	s.mu.RLock()
	item, ok := s.items[code]
	s.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("code %s not found", code)
	}

	cacheTTL := 10 * time.Minute
	if item.IsExpired(time.Now()) {
		cacheTTL = 1 * time.Nanosecond
	}
	s.cache.Set(code, item, cacheTTL)
	result := item
	return &result, nil
}

func (s *URLStore) GetWithGuard(code string) (*model.ShortURL, error) {
	if code == "" {
		return nil, fmt.Errorf("code is required")
	}

	if s.guard != nil {
		if s.guard(code, "") {
			return nil, fmt.Errorf("guard check failed for %s", code)
		}
	}

	if cached, ok := s.cache.Get(code); ok {
		if u, ok := cached.(model.ShortURL); ok {
			result := u
			return &result, nil
		}
	}

	s.mu.RLock()
	item, ok := s.items[code]
	s.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("code %s not found", code)
	}

	cacheTTL := 10 * time.Minute
	if item.IsExpired(time.Now()) {
		cacheTTL = 1 * time.Nanosecond
	}
	s.cache.Set(code, item, cacheTTL)
	result := item
	return &result, nil
}

func (s *URLStore) IncrementVisitsWithGuard(code string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return fmt.Errorf("store is closed")
	}

	if s.guard != nil && s.guard(code, "") {
		return fmt.Errorf("guard check failed for %s", code)
	}

	item, ok := s.items[code]
	if !ok {
		return fmt.Errorf("code %s not found", code)
	}

	item.Visits++
	s.items[code] = item
	cacheTTL := 10 * time.Minute
	if item.IsExpired(time.Now()) {
		cacheTTL = 1 * time.Nanosecond
	}
	s.cache.Set(code, item, cacheTTL)

	return nil
}

func (s *URLStore) RawSnapshot() map[string]model.ShortURL {
	s.mu.RLock()
	defer s.mu.RUnlock()

	snapshot := make(map[string]model.ShortURL, len(s.items))
	for k, v := range s.items {
		snapshot[k] = v
	}
	return snapshot
}

func (s *URLStore) CacheKeys() []string {
	return s.cache.Keys()
}

func (s *URLStore) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	close(s.stopCh)
	s.mu.Unlock()

	s.wg.Wait()
	s.flush()
	return nil
}

func (s *URLStore) flushLoop() {
	defer s.wg.Done()

	syncInterval := s.cfg.Storage.GetSyncInterval()
	if syncInterval <= 0 {
		syncInterval = 5 * time.Second
	}

	ticker := time.NewTicker(syncInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case item := <-s.saveCh:
			s.mu.Lock()
			s.items[item.Code] = item
			s.mu.Unlock()
		case <-ticker.C:
			s.flush()
		}
	}
}

func (s *URLStore) flush() {
	s.mu.RLock()
	defer s.mu.RUnlock()

	urlFilePath := s.cfg.Storage.GetURLFilePath()
	if urlFilePath == "" {
		return
	}

	dir := filepath.Dir(urlFilePath)
	if dir != "" {
		_ = os.MkdirAll(dir, 0755)
	}

	data, err := json.MarshalIndent(s.items, "", "  ")
	if err != nil {
		return
	}

	_ = os.WriteFile(urlFilePath, data, 0644)
}
