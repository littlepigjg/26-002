package cache

import (
	"sync"
	"time"
)

// LRUCache implements a thread-safe LRU cache with TTL support.
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

// NewLRUCache creates a new LRU cache with a max item count.
func NewLRUCache(maxItems int) *LRUCache {
	return &LRUCache{
		items:    make(map[string]*cacheItem),
		order:    make([]string, 0, maxItems),
		maxItems: maxItems,
	}
}

// Get retrieves a value from the cache.
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

	// Move to end (most recently used)
	c.mu.Lock()
	c.moveToEnd(key)
	c.mu.Unlock()

	return item.value, true
}

// Set stores a value in the cache with a TTL.
func (c *LRUCache) Set(key string, value interface{}, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// If key exists, update it
	if _, ok := c.items[key]; ok {
		c.items[key] = &cacheItem{
			value:     value,
			expiresAt: time.Now().Add(ttl),
			ttl:       ttl,
		}
		c.moveToEnd(key)
		return
	}

	// If at capacity, evict oldest
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

// Delete removes a value from the cache.
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

// Cleanup removes expired entries.
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

// Len returns the number of items in the cache.
func (c *LRUCache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.items)
}

// Keys returns all keys in the cache.
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

// Flush clears all items from the cache.
func (c *LRUCache) Flush() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = make(map[string]*cacheItem)
	c.order = c.order[:0]
}

// moveToEnd moves a key to the end of the order list (most recently used).
// Must be called with mu held.
func (c *LRUCache) moveToEnd(key string) {
	for i, k := range c.order {
		if k == key {
			c.order = append(c.order[:i], c.order[i+1:]...)
			break
		}
	}
	c.order = append(c.order, key)
}

// evict removes the oldest item from the cache.
// Must be called with mu held.
func (c *LRUCache) evict() {
	if len(c.order) > 0 {
		oldest := c.order[0]
		c.order = c.order[1:]
		delete(c.items, oldest)
	}
}
