package cache

import (
	"testing"
	"time"

	"github.com/spokanepubliclibrary/fsip2/internal/folio/models"
)

func TestNewTokenCache(t *testing.T) {
	cache := NewTokenCache(1000)
	if cache == nil {
		t.Fatal("NewTokenCache should not return nil")
	}

	if cache.capacity != 1000 {
		t.Errorf("Expected capacity 1000, got %d", cache.capacity)
	}
}

func TestTokenCacheSetAndGet(t *testing.T) {
	cache := NewTokenCache(1000)

	token := &models.TokenCache{
		AccessToken: "test-token",
		UserID:      "user-123",
		ExpiresAt:   time.Now().Add(10 * time.Minute),
	}

	// Set token
	err := cache.Set("test-key", token)
	if err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	// Get token
	retrieved, found := cache.Get("test-key")
	if !found {
		t.Error("Should find the token")
	}

	if retrieved.AccessToken != "test-token" {
		t.Errorf("Expected token 'test-token', got '%s'", retrieved.AccessToken)
	}
}

func TestTokenCacheExpiredToken(t *testing.T) {
	cache := NewTokenCache(1000)

	// Create expired token
	expiredToken := &models.TokenCache{
		AccessToken: "expired-token",
		UserID:      "user-123",
		ExpiresAt:   time.Now().Add(-1 * time.Minute),
	}

	// Should return error when setting expired token
	err := cache.Set("expired-key", expiredToken)
	if err == nil {
		t.Error("Set() should return error for expired token")
	}
}

func TestTokenCacheNilToken(t *testing.T) {
	cache := NewTokenCache(1000)

	// Should return error when setting nil token
	err := cache.Set("nil-key", nil)
	if err == nil {
		t.Error("Set() should return error for nil token")
	}
}

func TestTokenCacheDelete(t *testing.T) {
	cache := NewTokenCache(1000)

	token := &models.TokenCache{
		AccessToken: "test-token",
		UserID:      "user-123",
		ExpiresAt:   time.Now().Add(10 * time.Minute),
	}

	cache.Set("test-key", token)
	cache.Delete("test-key")

	// Verify deleted
	_, found := cache.Get("test-key")
	if found {
		t.Error("Token should be deleted")
	}
}

func TestTokenCacheClear(t *testing.T) {
	cache := NewTokenCache(1000)

	token1 := &models.TokenCache{
		AccessToken: "token-1",
		ExpiresAt:   time.Now().Add(10 * time.Minute),
	}

	token2 := &models.TokenCache{
		AccessToken: "token-2",
		ExpiresAt:   time.Now().Add(10 * time.Minute),
	}

	cache.Set("key1", token1)
	cache.Set("key2", token2)

	// Clear cache
	cache.Clear()

	// Verify cleared
	_, found := cache.Get("key1")
	if found {
		t.Error("key1 should be cleared")
	}

	_, found = cache.Get("key2")
	if found {
		t.Error("key2 should be cleared")
	}
}

func TestTokenCacheGetByUsername(t *testing.T) {
	cache := NewTokenCache(1000)

	token := &models.TokenCache{
		AccessToken: "user-token",
		ExpiresAt:   time.Now().Add(10 * time.Minute),
	}

	cache.SetByUsername("john_doe", token)

	retrieved, found := cache.GetByUsername("john_doe")
	if !found {
		t.Error("Should find token by username")
	}

	if retrieved.AccessToken != "user-token" {
		t.Errorf("Expected token 'user-token', got '%s'", retrieved.AccessToken)
	}
}

func TestTokenCacheGetByUserID(t *testing.T) {
	cache := NewTokenCache(1000)

	token := &models.TokenCache{
		AccessToken: "user-id-token",
		UserID:      "user-456",
		ExpiresAt:   time.Now().Add(10 * time.Minute),
	}

	cache.SetByUserID("user-456", token)

	retrieved, found := cache.GetByUserID("user-456")
	if !found {
		t.Error("Should find token by user ID")
	}

	if retrieved.AccessToken != "user-id-token" {
		t.Errorf("Expected token 'user-id-token', got '%s'", retrieved.AccessToken)
	}
}

func TestBuildCacheKey(t *testing.T) {
	key := BuildCacheKey("john_doe", "diku")
	expected := "john_doe@diku"

	if key != expected {
		t.Errorf("Expected key '%s', got '%s'", expected, key)
	}
}

func TestTokenCacheAutoExpiration(t *testing.T) {
	cache := NewTokenCache(1000)

	// Create token that expires in 2 minutes (beyond the 90s buffer)
	token := &models.TokenCache{
		AccessToken: "short-lived-token",
		ExpiresAt:   time.Now().Add(2 * time.Minute),
	}

	cache.Set("short-key", token)

	// Should exist immediately
	_, found := cache.Get("short-key")
	if !found {
		t.Error("Token should exist immediately")
	}

	// Create an already-expired token (for auto-removal test)
	expiredToken := &models.TokenCache{
		AccessToken: "expired-token",
		ExpiresAt:   time.Now().Add(-2 * time.Minute),
	}

	// Set will fail for already-expired token, so manually set and then get
	cache.cache.Set("expired-key", expiredToken, 5*time.Minute)

	// Get should remove the expired token
	_, found = cache.Get("expired-key")
	if found {
		t.Error("Expired token should be auto-removed on Get")
	}
}

func TestTokenCacheInvalidTypeHandling(t *testing.T) {
	cache := NewTokenCache(1000)

	// Manually insert wrong type into underlying cache
	cache.cache.Set("wrong-type", "not a token", 5*time.Minute)

	// Get should handle invalid type gracefully
	_, found := cache.Get("wrong-type")
	if found {
		t.Error("Should not return invalid type as token")
	}

	// Should remove invalid entry
	_, stillExists := cache.cache.Get("wrong-type")
	if stillExists {
		t.Error("Invalid type should be removed from cache")
	}
}

func TestTokenCacheConcurrentAccess(t *testing.T) {
	cache := NewTokenCache(1000)

	done := make(chan bool, 3)

	// Writer 1
	go func() {
		for i := 0; i < 50; i++ {
			token := &models.TokenCache{
				AccessToken: "token-1",
				ExpiresAt:   time.Now().Add(10 * time.Minute),
			}
			cache.Set("key1", token)
		}
		done <- true
	}()

	// Writer 2
	go func() {
		for i := 0; i < 50; i++ {
			token := &models.TokenCache{
				AccessToken: "token-2",
				ExpiresAt:   time.Now().Add(10 * time.Minute),
			}
			cache.Set("key2", token)
		}
		done <- true
	}()

	// Reader
	go func() {
		for i := 0; i < 50; i++ {
			_, _ = cache.Get("key1")
			_, _ = cache.Get("key2")
		}
		done <- true
	}()

	<-done
	<-done
	<-done
}
