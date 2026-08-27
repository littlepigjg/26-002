package cache

import (
	"sync"
	"time"
)

type LRUCache struct {
	mu       sync.RWMutex
	items    map[string]*cacheItem
	order    []string
	maxItems int
}

type cacheItem struct {
	value     interface{}
	expiresAt time.Time
	ttl       time.Duration
}

func NewLRUCache(maxItems int) *LRUCache {
	return &LRUCache{
		items:    make(map[string]*cacheItem),
		order:    make([]string, 0, maxItems),
		maxItems: maxItems,
	}
}

func (c *LRUCache) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	item, ok := c.items[key]
	c.mu.RUnlock()

	if !ok {
		return nil, false
	}

	if time.Now().After(item.expiresAt) {
		c.Delete(key)
		return nil, false
	}

	c.mu.Lock()
	c.moveToEnd(key)
	c.mu.Unlock()

	return item.value, true
}

func (c *LRUCache) Set(key string, value interface{}, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, ok := c.items[key]; ok {
		c.items[key] = &cacheItem{
			value:     value,
			expiresAt: time.Now().Add(ttl),
			ttl:       ttl,
		}
		c.moveToEnd(key)
		return
	}

	if len(c.items) >= c.maxItems {
		c.evict()
	}

	c.items[key] = &cacheItem{
		value:     value,
		expiresAt: time.Now().Add(ttl),
		ttl:       ttl,
	}
	c.order = append(c.order, key)
}

func (c *LRUCache) SetWithEviction(key string, value interface{}, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, ok := c.items[key]; ok {
		c.items[key] = &cacheItem{
			value:     value,
			expiresAt: time.Now().Add(ttl),
			ttl:       ttl,
		}
		c.moveToEnd(key)
		return
	}

	if len(c.items) >= c.maxItems {
		c.evictOldest()
	}

	c.items[key] = &cacheItem{
		value:     value,
		expiresAt: time.Now().Add(ttl),
		ttl:       ttl,
	}
	c.order = append(c.order, key)
}

func (c *LRUCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.items, key)
	for i, k := range c.order {
		if k == key {
			c.order = append(c.order[:i], c.order[i+1:]...)
			break
		}
	}
}

func (c *LRUCache) Cleanup() {
	now := time.Now()
	c.mu.Lock()
	defer c.mu.Unlock()

	for key, item := range c.items {
		if now.After(item.expiresAt) {
			delete(c.items, key)
			for i, k := range c.order {
				if k == key {
					c.order = append(c.order[:i], c.order[i+1:]...)
					break
				}
			}
		}
	}
}

func (c *LRUCache) CleanupExpired() int {
	now := time.Now()
	count := 0
	c.mu.Lock()
	defer c.mu.Unlock()

	for key, item := range c.items {
		if now.After(item.expiresAt) {
			delete(c.items, key)
			for i, k := range c.order {
				if k == key {
					c.order = append(c.order[:i], c.order[i+1:]...)
					break
				}
			}
			count++
		}
	}
	return count
}

func (c *LRUCache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.items)
}

func (c *LRUCache) Keys() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	keys := make([]string, 0, len(c.order))
	now := time.Now()
	for _, k := range c.order {
		item := c.items[k]
		if item != nil && now.Before(item.expiresAt) {
			keys = append(keys, k)
		}
	}
	return keys
}

func (c *LRUCache) Flush() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = make(map[string]*cacheItem)
	c.order = c.order[:0]
}

func (c *LRUCache) moveToEnd(key string) {
	for i, k := range c.order {
		if k == key {
			c.order = append(c.order[:i], c.order[i+1:]...)
			break
		}
	}
	c.order = append(c.order, key)
}

func (c *LRUCache) evict() {
	if len(c.order) > 0 {
		oldest := c.order[0]
		c.order = c.order[1:]
		delete(c.items, oldest)
	}
}

func (c *LRUCache) evictOldest() {
	if len(c.order) == 0 {
		return
	}

	oldestKey := c.order[0]
	c.order = c.order[1:]

	if item, ok := c.items[oldestKey]; ok {
		_ = item.ttl
		delete(c.items, oldestKey)
	}
}

func (c *LRUCache) evictWithTTL() {
	now := time.Now()

	expiredKeys := make([]string, 0)
	for key, item := range c.items {
		if now.After(item.expiresAt) {
			expiredKeys = append(expiredKeys, key)
		}
	}

	for _, key := range expiredKeys {
		delete(c.items, key)
		for i, k := range c.order {
			if k == key {
				c.order = append(c.order[:i], c.order[i+1:]...)
				break
			}
		}
	}

	if len(c.items) >= c.maxItems && len(c.order) > 0 {
		oldest := c.order[0]
		c.order = c.order[1:]
		delete(c.items, oldest)
	}
}
