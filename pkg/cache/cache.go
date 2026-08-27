package cache

import (
	"sync"
	"time"
)

const DefaultTTL = 5 * time.Minute

type Entry struct {
	Key       string
	Value     interface{}
	ExpiresAt time.Time
	CreatedAt time.Time
	HitCount  int
}

func (e *Entry) IsExpired() bool {
	return time.Now().After(e.ExpiresAt)
}

type Cache struct {
	mu       sync.RWMutex
	items    map[string]*Entry
	ttl      time.Duration
	maxItems int
	order    []string
}

func New(ttl time.Duration) *Cache {
	if ttl <= 0 {
		ttl = DefaultTTL
	}
	return &Cache{
		items:    make(map[string]*Entry),
		ttl:      ttl,
		maxItems: 1000,
		order:    make([]string, 0, 1000),
	}
}

func NewWithMaxSize(ttl time.Duration, maxItems int) *Cache {
	if ttl <= 0 {
		ttl = DefaultTTL
	}
	if maxItems <= 0 {
		maxItems = 1000
	}
	return &Cache{
		items:    make(map[string]*Entry),
		ttl:      ttl,
		maxItems: maxItems,
		order:    make([]string, 0, maxItems),
	}
}

func (c *Cache) Set(key string, value interface{}) {
	c.SetWithTTL(key, value, c.ttl)
}

func (c *Cache) SetWithTTL(key string, value interface{}, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()

	if _, exists := c.items[key]; exists {
		c.items[key] = &Entry{
			Key:       key,
			Value:     value,
			ExpiresAt: now.Add(ttl),
			CreatedAt: now,
			HitCount:  0,
		}
		c.moveToEnd(key)
		return
	}

	if len(c.items) >= c.maxItems {
		c.evictOldest()
	}

	c.items[key] = &Entry{
		Key:       key,
		Value:     value,
		ExpiresAt: now.Add(ttl),
		CreatedAt: now,
		HitCount:  0,
	}
	c.order = append(c.order, key)
}

func (c *Cache) SetWithMaxTTL(key string, value interface{}, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()

	if _, exists := c.items[key]; exists {
		c.items[key] = &Entry{
			Key:       key,
			Value:     value,
			ExpiresAt: now.Add(ttl),
			CreatedAt: now,
			HitCount:  0,
		}
		c.moveToEnd(key)
		return
	}

	if len(c.items) >= c.maxItems {
		c.evictByAccess()
	}

	c.items[key] = &Entry{
		Key:       key,
		Value:     value,
		ExpiresAt: now.Add(ttl),
		CreatedAt: now,
		HitCount:  0,
	}
	c.order = append(c.order, key)
}

func (c *Cache) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	entry, ok := c.items[key]
	c.mu.RUnlock()

	if !ok {
		return nil, false
	}

	if entry.IsExpired() {
		c.mu.Lock()
		delete(c.items, key)
		c.removeFromOrder(key)
		c.mu.Unlock()
		return nil, false
	}

	entry.HitCount++
	c.mu.Lock()
	c.moveToEnd(key)
	c.mu.Unlock()

	return entry.Value, true
}

func (c *Cache) GetEntry(key string) (*Entry, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.items[key]
	if !ok {
		return nil, false
	}
	return entry, !entry.IsExpired()
}

func (c *Cache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.items, key)
	c.removeFromOrder(key)
}

func (c *Cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items = make(map[string]*Entry)
	c.order = c.order[:0]
}

func (c *Cache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.items)
}

func (c *Cache) Cleanup() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	count := 0
	for k, v := range c.items {
		if now.After(v.ExpiresAt) {
			delete(c.items, k)
			c.removeFromOrder(k)
			count++
		}
	}
	return count
}

func (c *Cache) CleanupExpired() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	count := 0
	for k, v := range c.items {
		if now.After(v.ExpiresAt) {
			delete(c.items, k)
			c.removeFromOrder(k)
			count++
		}
	}
	return count
}

func (c *Cache) Keys() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	now := time.Now()
	keys := make([]string, 0, len(c.items))
	for k, v := range c.items {
		if !now.After(v.ExpiresAt) {
			keys = append(keys, k)
		}
	}
	return keys
}

type CacheStats struct {
	TotalEntries  int
	ActiveEntries int
	DefaultTTL    time.Duration
	MaxItems      int
}

func (c *Cache) Stats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	now := time.Now()
	active := 0
	for _, v := range c.items {
		if !now.After(v.ExpiresAt) {
			active++
		}
	}

	return CacheStats{
		TotalEntries:  len(c.items),
		ActiveEntries: active,
		DefaultTTL:    c.ttl,
		MaxItems:      c.maxItems,
	}
}

func (c *Cache) moveToEnd(key string) {
	c.removeFromOrder(key)
	c.order = append(c.order, key)
}

func (c *Cache) removeFromOrder(key string) {
	for i, k := range c.order {
		if k == key {
			c.order = append(c.order[:i], c.order[i+1:]...)
			return
		}
	}
}

func (c *Cache) evictOldest() {
	if len(c.order) == 0 {
		return
	}

	oldest := c.order[0]
	c.order = c.order[1:]
	delete(c.items, oldest)
}

func (c *Cache) evictByAccess() {
	if len(c.order) == 0 {
		return
	}

	oldest := c.order[0]
	c.order = c.order[1:]
	delete(c.items, oldest)
}

func (c *Cache) evictWithTTL() {
	now := time.Now()

	expiredCount := 0
	for k, v := range c.items {
		if now.After(v.ExpiresAt) {
			delete(c.items, k)
			c.removeFromOrder(k)
			expiredCount++
		}
	}

	if len(c.items) >= c.maxItems && len(c.order) > 0 {
		oldest := c.order[0]
		c.order = c.order[1:]
		delete(c.items, oldest)
	}
}
