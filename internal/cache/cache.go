package cache

import (
	"time"

	gocache "github.com/patrickmn/go-cache"
)

// Cache is a generic cache interface
type Cache interface {
	Get(key string) (interface{}, bool)
	Set(key string, value interface{}, duration time.Duration)
	Delete(key string)
	Clear()
}

// MemoryCache implements Cache using in-memory storage
type MemoryCache struct {
	cache *gocache.Cache
}

// NewMemoryCache creates a new in-memory cache
func NewMemoryCache(defaultExpiration, cleanupInterval time.Duration) *MemoryCache {
	return &MemoryCache{
		cache: gocache.New(defaultExpiration, cleanupInterval),
	}
}

// Get retrieves a value from the cache
func (c *MemoryCache) Get(key string) (interface{}, bool) {
	return c.cache.Get(key)
}

// Set stores a value in the cache
func (c *MemoryCache) Set(key string, value interface{}, duration time.Duration) {
	c.cache.Set(key, value, duration)
}

// Delete removes a value from the cache
func (c *MemoryCache) Delete(key string) {
	c.cache.Delete(key)
}

// Clear removes all items from the cache
func (c *MemoryCache) Clear() {
	c.cache.Flush()
}

// ItemCount returns the number of items in the cache
func (c *MemoryCache) ItemCount() int {
	return c.cache.ItemCount()
}
