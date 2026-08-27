// Package cache provides an in-memory cache implementation with TTL support.
package cache

import (
	"sync"
	"time"
)

// DefaultTTL is the default time-to-live for cache entries.
const DefaultTTL = 5 * time.Minute

// Entry represents a single cache entry.
type Entry struct {
	Key       string
	Value     interface{}
	ExpiresAt time.Time
	CreatedAt time.Time
	HitCount  int
}

// IsExpired checks if the entry has expired.
func (e *Entry) IsExpired() bool {
	return time.Now().After(e.ExpiresAt)
}

// Cache is a thread-safe in-memory cache with TTL support.
type Cache struct {
	mu    sync.RWMutex
	items map[string]*Entry
	ttl   time.Duration
}

// New creates a new Cache with the given default TTL.
func New(ttl time.Duration) *Cache {
	if ttl <= 0 {
		ttl = DefaultTTL
	}
	return &Cache{
		items: make(map[string]*Entry),
		ttl:   ttl,
	}
}

// Set adds or updates a cache entry with the default TTL.
func (c *Cache) Set(key string, value interface{}) {
	c.SetWithTTL(key, value, c.ttl)
}

// SetWithTTL adds or updates a cache entry with a custom TTL.
func (c *Cache) SetWithTTL(key string, value interface{}, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	c.items[key] = &Entry{
		Key:       key,
		Value:     value,
		ExpiresAt: now.Add(ttl),
		CreatedAt: now,
		HitCount:  0,
	}
}

// Get retrieves a value from the cache.
// Returns the value and true if found (and not expired), or nil and false otherwise.
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
		c.mu.Unlock()
		return nil, false
	}

	entry.HitCount++
	return entry.Value, true
}

// GetEntry returns the full cache entry for inspection.
func (c *Cache) GetEntry(key string) (*Entry, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	entry, ok := c.items[key]
	if !ok {
		return nil, false
	}
	return entry, !entry.IsExpired()
}

// Delete removes a key from the cache.
func (c *Cache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.items, key)
}

// Clear removes all entries from the cache.
func (c *Cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = make(map[string]*Entry)
}

// Len returns the number of entries in the cache (including expired ones).
func (c *Cache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.items)
}

// Cleanup removes expired entries from the cache.
func (c *Cache) Cleanup() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	count := 0
	for k, v := range c.items {
		if now.After(v.ExpiresAt) {
			delete(c.items, k)
			count++
		}
	}
	return count
}

// Keys returns all non-expired keys in the cache.
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

// Stats returns cache statistics.
type CacheStats struct {
	TotalEntries int
	ActiveEntries int
	DefaultTTL    time.Duration
}

// Stats returns the current cache statistics.
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
	}
}
