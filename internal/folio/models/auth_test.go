package models

import (
	"testing"
	"time"
)

func TestTokenCache_IsExpired_NotExpired(t *testing.T) {
	tc := &TokenCache{
		ExpiresAt: time.Now().Add(10 * time.Minute),
	}
	if tc.IsExpired() {
		t.Error("IsExpired() = true, want false for token expiring in 10 minutes")
	}
}

func TestTokenCache_IsExpired_Expired(t *testing.T) {
	tc := &TokenCache{
		ExpiresAt: time.Now().Add(-1 * time.Minute),
	}
	if !tc.IsExpired() {
		t.Error("IsExpired() = false, want true for already-expired token")
	}
}

func TestTokenCache_IsExpired_WithinBuffer(t *testing.T) {
	// Token expires in 60 seconds — within the 90-second buffer, so should be considered expired
	tc := &TokenCache{
		ExpiresAt: time.Now().Add(60 * time.Second),
	}
	if !tc.IsExpired() {
		t.Error("IsExpired() = false, want true for token within 90s buffer")
	}
}

func TestTokenCache_IsExpired_JustOutsideBuffer(t *testing.T) {
	// Token expires in 120 seconds — outside the 90-second buffer, so should NOT be expired
	tc := &TokenCache{
		ExpiresAt: time.Now().Add(120 * time.Second),
	}
	if tc.IsExpired() {
		t.Error("IsExpired() = true, want false for token with 120s remaining")
	}
}

func TestTokenCache_NeedsRefresh_DelegatesToIsExpired(t *testing.T) {
	fresh := &TokenCache{ExpiresAt: time.Now().Add(10 * time.Minute)}
	expired := &TokenCache{ExpiresAt: time.Now().Add(-1 * time.Minute)}

	if fresh.NeedsRefresh() != fresh.IsExpired() {
		t.Error("NeedsRefresh() does not match IsExpired() for fresh token")
	}
	if expired.NeedsRefresh() != expired.IsExpired() {
		t.Error("NeedsRefresh() does not match IsExpired() for expired token")
	}
}
