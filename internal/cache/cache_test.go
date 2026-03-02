package cache

import (
	"testing"
	"time"
)

func TestNewMemoryCache(t *testing.T) {
	cache := NewMemoryCache(5*time.Minute, 1*time.Minute)
	if cache == nil {
		t.Fatal("NewMemoryCache should not return nil")
	}
}

func TestMemoryCacheSetAndGet(t *testing.T) {
	cache := NewMemoryCache(5*time.Minute, 1*time.Minute)

	// Set a value
	cache.Set("key1", "value1", 5*time.Minute)

	// Get the value
	value, found := cache.Get("key1")
	if !found {
		t.Error("Should find the key")
	}

	if value != "value1" {
		t.Errorf("Expected 'value1', got '%v'", value)
	}
}

func TestMemoryCacheGetNonExistent(t *testing.T) {
	cache := NewMemoryCache(5*time.Minute, 1*time.Minute)

	// Try to get non-existent key
	_, found := cache.Get("nonexistent")
	if found {
		t.Error("Should not find non-existent key")
	}
}

func TestMemoryCacheDelete(t *testing.T) {
	cache := NewMemoryCache(5*time.Minute, 1*time.Minute)

	// Set and delete
	cache.Set("key1", "value1", 5*time.Minute)
	cache.Delete("key1")

	// Verify deleted
	_, found := cache.Get("key1")
	if found {
		t.Error("Key should be deleted")
	}
}

func TestMemoryCacheClear(t *testing.T) {
	cache := NewMemoryCache(5*time.Minute, 1*time.Minute)

	// Set multiple values
	cache.Set("key1", "value1", 5*time.Minute)
	cache.Set("key2", "value2", 5*time.Minute)
	cache.Set("key3", "value3", 5*time.Minute)

	// Clear cache
	cache.Clear()

	// Verify all cleared
	_, found := cache.Get("key1")
	if found {
		t.Error("key1 should be cleared")
	}

	_, found = cache.Get("key2")
	if found {
		t.Error("key2 should be cleared")
	}

	count := cache.ItemCount()
	if count != 0 {
		t.Errorf("Expected 0 items after clear, got %d", count)
	}
}

func TestMemoryCacheExpiration(t *testing.T) {
	cache := NewMemoryCache(50*time.Millisecond, 10*time.Millisecond)

	// Set with short expiration
	cache.Set("key1", "value1", 50*time.Millisecond)

	// Should exist immediately
	_, found := cache.Get("key1")
	if !found {
		t.Error("Key should exist immediately after setting")
	}

	// Wait for expiration
	time.Sleep(100 * time.Millisecond)

	// Should not exist after expiration
	_, found = cache.Get("key1")
	if found {
		t.Error("Key should expire after duration")
	}
}

func TestMemoryCacheItemCount(t *testing.T) {
	cache := NewMemoryCache(5*time.Minute, 1*time.Minute)

	if cache.ItemCount() != 0 {
		t.Error("New cache should have 0 items")
	}

	cache.Set("key1", "value1", 5*time.Minute)
	if cache.ItemCount() != 1 {
		t.Error("Cache should have 1 item after adding")
	}

	cache.Set("key2", "value2", 5*time.Minute)
	if cache.ItemCount() != 2 {
		t.Error("Cache should have 2 items after adding")
	}

	cache.Delete("key1")
	if cache.ItemCount() != 1 {
		t.Error("Cache should have 1 item after deleting")
	}
}

func TestMemoryCacheConcurrentAccess(t *testing.T) {
	cache := NewMemoryCache(5*time.Minute, 1*time.Minute)

	done := make(chan bool, 3)

	// Writer 1
	go func() {
		for i := 0; i < 100; i++ {
			cache.Set("key1", i, 5*time.Minute)
		}
		done <- true
	}()

	// Writer 2
	go func() {
		for i := 0; i < 100; i++ {
			cache.Set("key2", i, 5*time.Minute)
		}
		done <- true
	}()

	// Reader
	go func() {
		for i := 0; i < 100; i++ {
			_, _ = cache.Get("key1")
			_, _ = cache.Get("key2")
		}
		done <- true
	}()

	// Wait for all goroutines
	<-done
	<-done
	<-done
}
