package types

import (
	"sync"
	"testing"
	"time"

	"github.com/spokanepubliclibrary/fsip2/internal/config"
)

func newTestTenantConfig() *config.TenantConfig {
	return &config.TenantConfig{
		Tenant:      "test-tenant",
		OkapiURL:    "http://okapi.example.com",
		OkapiTenant: "test",
	}
}

func TestNewSession(t *testing.T) {
	tc := newTestTenantConfig()
	before := time.Now()
	s := NewSession("session-1", tc)
	after := time.Now()

	if s.ID != "session-1" {
		t.Errorf("ID = %q, want %q", s.ID, "session-1")
	}
	if s.TenantConfig != tc {
		t.Error("TenantConfig not set")
	}
	if s.IsAuthenticated {
		t.Error("new session should not be authenticated")
	}
	if s.CreatedAt.Before(before) || s.CreatedAt.After(after) {
		t.Errorf("CreatedAt out of range")
	}
	if s.LastActivity.Before(before) || s.LastActivity.After(after) {
		t.Errorf("LastActivity out of range")
	}
}

func TestSetAuthenticated(t *testing.T) {
	s := NewSession("s1", newTestTenantConfig())
	expiry := time.Now().Add(1 * time.Hour)

	s.SetAuthenticated("user1", "patron-id", "patron-barcode", "token-abc", expiry)

	if !s.IsAuthenticated {
		t.Error("should be authenticated after SetAuthenticated")
	}
	if s.Username != "user1" {
		t.Errorf("Username = %q, want %q", s.Username, "user1")
	}
	if s.PatronID != "patron-id" {
		t.Errorf("PatronID = %q, want %q", s.PatronID, "patron-id")
	}
	if s.PatronBarcode != "patron-barcode" {
		t.Errorf("PatronBarcode = %q, want %q", s.PatronBarcode, "patron-barcode")
	}
	if s.AuthToken != "token-abc" {
		t.Errorf("AuthToken = %q, want %q", s.AuthToken, "token-abc")
	}
	if !s.TokenExpiresAt.Equal(expiry) {
		t.Errorf("TokenExpiresAt = %v, want %v", s.TokenExpiresAt, expiry)
	}
}

func TestSetAuthCredentials(t *testing.T) {
	s := NewSession("s1", nil)
	s.Username = "user1"

	if s.HasAuthCredentials() {
		t.Error("should not have credentials before SetAuthCredentials")
	}

	s.SetAuthCredentials("secret")

	if !s.HasAuthCredentials() {
		t.Error("should have credentials after SetAuthCredentials")
	}

	u, p := s.GetAuthCredentials()
	if u != "user1" {
		t.Errorf("username = %q, want %q", u, "user1")
	}
	if p != "secret" {
		t.Errorf("password = %q, want %q", p, "secret")
	}
}

func TestHasAuthCredentials_MissingPassword(t *testing.T) {
	s := NewSession("s1", nil)
	s.Username = "user1"
	// no password set
	if s.HasAuthCredentials() {
		t.Error("should not have credentials without password")
	}
}

func TestHasAuthCredentials_MissingUsername(t *testing.T) {
	s := NewSession("s1", nil)
	s.SetAuthCredentials("pass")
	// Username is empty
	if s.HasAuthCredentials() {
		t.Error("should not have credentials without username")
	}
}

func TestUpdateToken(t *testing.T) {
	s := NewSession("s1", nil)
	expiry := time.Now().Add(2 * time.Hour)
	s.UpdateToken("new-token", expiry)

	if s.GetAuthToken() != "new-token" {
		t.Errorf("GetAuthToken = %q, want %q", s.GetAuthToken(), "new-token")
	}
	if !s.GetTokenExpiresAt().Equal(expiry) {
		t.Errorf("GetTokenExpiresAt = %v, want %v", s.GetTokenExpiresAt(), expiry)
	}
}

func TestUpdateActivity(t *testing.T) {
	s := NewSession("s1", nil)
	before := s.LastActivity
	time.Sleep(2 * time.Millisecond)
	s.UpdateActivity()
	if !s.LastActivity.After(before) {
		t.Error("LastActivity should have been updated")
	}
}

func TestSetLocationCode(t *testing.T) {
	s := NewSession("s1", nil)
	s.SetLocationCode("loc-456")
	if s.GetLocationCode() != "loc-456" {
		t.Errorf("GetLocationCode = %q, want %q", s.GetLocationCode(), "loc-456")
	}
}

func TestGetPatronID(t *testing.T) {
	s := NewSession("s1", nil)
	s.PatronID = "p123"
	if s.GetPatronID() != "p123" {
		t.Errorf("GetPatronID = %q, want %q", s.GetPatronID(), "p123")
	}
}

func TestGetPatronBarcode(t *testing.T) {
	s := NewSession("s1", nil)
	s.PatronBarcode = "bc123"
	if s.GetPatronBarcode() != "bc123" {
		t.Errorf("GetPatronBarcode = %q, want %q", s.GetPatronBarcode(), "bc123")
	}
}

func TestGetUsername(t *testing.T) {
	s := NewSession("s1", nil)
	s.Username = "john"
	if s.GetUsername() != "john" {
		t.Errorf("GetUsername = %q, want %q", s.GetUsername(), "john")
	}
}

func TestIsAuth(t *testing.T) {
	s := NewSession("s1", nil)
	if s.IsAuth() {
		t.Error("new session should not be authenticated")
	}
	s.IsAuthenticated = true
	if !s.IsAuth() {
		t.Error("session should be authenticated after setting flag")
	}
}

func TestIsTokenExpired_ZeroTime(t *testing.T) {
	s := NewSession("s1", nil)
	// Zero time should be considered expired
	if !s.IsTokenExpired() {
		t.Error("zero TokenExpiresAt should be expired")
	}
}

func TestIsTokenExpired_FutureExpiry(t *testing.T) {
	s := NewSession("s1", nil)
	// Set expiry far in the future (beyond 90s buffer)
	s.TokenExpiresAt = time.Now().Add(10 * time.Minute)
	if s.IsTokenExpired() {
		t.Error("token with future expiry should not be expired")
	}
}

func TestIsTokenExpired_ImminentExpiry(t *testing.T) {
	s := NewSession("s1", nil)
	// Set expiry within the 90-second buffer
	s.TokenExpiresAt = time.Now().Add(30 * time.Second)
	if !s.IsTokenExpired() {
		t.Error("token expiring within 90s buffer should be considered expired")
	}
}

func TestClear(t *testing.T) {
	s := NewSession("s1", newTestTenantConfig())
	s.SetAuthenticated("user", "pid", "pbc", "tok", time.Now().Add(time.Hour))
	s.SetAuthCredentials("pass")
	s.LocationCode = "loc"

	s.Clear()

	if s.IsAuthenticated {
		t.Error("IsAuthenticated should be false after Clear")
	}
	if s.Username != "" {
		t.Errorf("Username should be empty after Clear, got %q", s.Username)
	}
	if s.PatronID != "" {
		t.Errorf("PatronID should be empty after Clear, got %q", s.PatronID)
	}
	if s.PatronBarcode != "" {
		t.Errorf("PatronBarcode should be empty after Clear, got %q", s.PatronBarcode)
	}
	if s.AuthToken != "" {
		t.Errorf("AuthToken should be empty after Clear, got %q", s.AuthToken)
	}
	if !s.TokenExpiresAt.IsZero() {
		t.Error("TokenExpiresAt should be zero after Clear")
	}
	if s.HasAuthCredentials() {
		t.Error("credentials should be cleared after Clear")
	}
}

func TestGetDuration(t *testing.T) {
	s := NewSession("s1", nil)
	time.Sleep(5 * time.Millisecond)
	d := s.GetDuration()
	if d <= 0 {
		t.Errorf("GetDuration = %v, want > 0", d)
	}
}

func TestGetIdleTime(t *testing.T) {
	s := NewSession("s1", nil)
	time.Sleep(5 * time.Millisecond)
	idle := s.GetIdleTime()
	if idle <= 0 {
		t.Errorf("GetIdleTime = %v, want > 0", idle)
	}
}

func TestUpdateTenant(t *testing.T) {
	s := NewSession("s1", nil)
	tc := newTestTenantConfig()
	s.UpdateTenant(tc)
	if s.GetTenant() != tc {
		t.Error("GetTenant should return updated config")
	}
}

func TestClone(t *testing.T) {
	tc := newTestTenantConfig()
	s := NewSession("s1", tc)
	s.SetAuthenticated("user", "pid", "pbc", "tok", time.Now().Add(time.Hour))
	s.SetAuthCredentials("secret")
	s.SetLocationCode("loc")

	clone := s.Clone()

	if clone.ID != s.ID {
		t.Errorf("Clone ID = %q, want %q", clone.ID, s.ID)
	}
	if clone.Username != s.Username {
		t.Errorf("Clone Username = %q, want %q", clone.Username, s.Username)
	}
	if clone.IsAuthenticated != s.IsAuthenticated {
		t.Error("Clone IsAuthenticated mismatch")
	}
	if clone.LocationCode != s.LocationCode {
		t.Error("Clone LocationCode mismatch")
	}
	// Password should NOT be copied
	_, clonePass := clone.GetAuthCredentials()
	if clonePass != "" {
		t.Error("Clone should not copy password")
	}
	// Cloning should not share mutex state
	if &clone.mu == &s.mu {
		t.Error("Clone should have its own mutex")
	}
}

func TestConcurrentAccess(t *testing.T) {
	s := NewSession("s1", nil)
	var wg sync.WaitGroup
	const goroutines = 20

	for i := 0; i < goroutines; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			s.SetAuthenticated("user", "pid", "pbc", "tok", time.Now().Add(time.Hour))
		}()
		go func() {
			defer wg.Done()
			_ = s.IsAuth()
			_ = s.GetAuthToken()
			_ = s.GetPatronID()
		}()
	}
	wg.Wait()
}
