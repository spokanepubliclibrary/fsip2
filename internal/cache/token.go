package cache

import (
	"fmt"
	"time"

	"github.com/spokanepubliclibrary/fsip2/internal/folio/models"
)

// TokenCache manages cached authentication tokens
type TokenCache struct {
	cache    Cache
	capacity int
}

// NewTokenCache creates a new token cache
func NewTokenCache(capacity int) *TokenCache {
	// Default expiration: 10 minutes, cleanup: 5 minutes
	return &TokenCache{
		cache:    NewMemoryCache(10*time.Minute, 5*time.Minute),
		capacity: capacity,
	}
}

// Get retrieves a token from the cache
func (tc *TokenCache) Get(key string) (*models.TokenCache, bool) {
	value, found := tc.cache.Get(key)
	if !found {
		return nil, false
	}

	token, ok := value.(*models.TokenCache)
	if !ok {
		// Invalid type, remove from cache
		tc.cache.Delete(key)
		return nil, false
	}

	// Check if token is expired
	if token.IsExpired() {
		tc.cache.Delete(key)
		return nil, false
	}

	return token, true
}

// Set stores a token in the cache
func (tc *TokenCache) Set(key string, token *models.TokenCache) error {
	if token == nil {
		return fmt.Errorf("cannot cache nil token")
	}

	// Calculate duration until expiration (with safety margin)
	duration := time.Until(token.ExpiresAt)
	if duration <= 0 {
		return fmt.Errorf("token is already expired")
	}

	tc.cache.Set(key, token, duration)
	return nil
}

// Delete removes a token from the cache
func (tc *TokenCache) Delete(key string) {
	tc.cache.Delete(key)
}

// Clear removes all tokens from the cache
func (tc *TokenCache) Clear() {
	tc.cache.Clear()
}

// GetByUsername retrieves a token by username
func (tc *TokenCache) GetByUsername(username string) (*models.TokenCache, bool) {
	return tc.Get(username)
}

// SetByUsername stores a token by username
func (tc *TokenCache) SetByUsername(username string, token *models.TokenCache) error {
	return tc.Set(username, token)
}

// GetByUserID retrieves a token by user ID
func (tc *TokenCache) GetByUserID(userID string) (*models.TokenCache, bool) {
	return tc.Get(userID)
}

// SetByUserID stores a token by user ID
func (tc *TokenCache) SetByUserID(userID string, token *models.TokenCache) error {
	return tc.Set(userID, token)
}

// BuildCacheKey builds a cache key from username and tenant
func BuildCacheKey(username, tenant string) string {
	return fmt.Sprintf("%s@%s", username, tenant)
}
