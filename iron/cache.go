package iron

import "sync"

// Cache stores processed IR results for reuse.
type Cache interface {
	Get(key string) (Result, bool)
	Set(key string, result Result)
}

// MemoryCache is an in-memory cache implementation.
type MemoryCache struct {
	mu   sync.RWMutex
	data map[string]Result
}

// NewMemoryCache creates a new MemoryCache.
func NewMemoryCache() *MemoryCache {
	return &MemoryCache{
		data: make(map[string]Result),
	}
}

// Get returns a cached result for the key.
func (c *MemoryCache) Get(key string) (Result, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result, ok := c.data[key]
	return result, ok
}

// Set stores a result for the key.
func (c *MemoryCache) Set(key string, result Result) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data[key] = result
}
