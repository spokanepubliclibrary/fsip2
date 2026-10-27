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

func TestSession_ClearPatron(t *testing.T) {
	s := NewSession("test-id", newTestTenantConfig())
	s.SetAuthenticated("user1", "patron-id", "patron-barcode", "token-abc", time.Now().Add(time.Hour))

	s.ClearPatron()

	if s.GetPatronID() != "" {
		t.Errorf("PatronID should be empty after ClearPatron, got %q", s.GetPatronID())
	}
	if s.GetPatronBarcode() != "" {
		t.Errorf("PatronBarcode should be empty after ClearPatron, got %q", s.GetPatronBarcode())
	}
	if !s.IsAuth() {
		t.Error("IsAuthenticated should still be true after ClearPatron")
	}
	if s.GetAuthToken() == "" {
		t.Error("AuthToken should be preserved after ClearPatron")
	}
	if s.GetUsername() == "" {
		t.Error("Username should be preserved after ClearPatron")
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

func TestSession_SetPatronBarcode(t *testing.T) {
	s := NewSession("s1", nil)
	s.SetPatronBarcode("barcode-xyz")
	if got := s.GetPatronBarcode(); got != "barcode-xyz" {
		t.Errorf("GetPatronBarcode = %q, want %q", got, "barcode-xyz")
	}
}

func TestSession_SetPatronID_Standalone(t *testing.T) {
	s := NewSession("s1", nil)
	s.SetPatronID("patron-123")
	if got := s.GetPatronID(); got != "patron-123" {
		t.Errorf("GetPatronID = %q, want %q", got, "patron-123")
	}
}

func TestSession_PatronBarcode_ConcurrentAccess(t *testing.T) {
	s := NewSession("s1", nil)
	var wg sync.WaitGroup
	const goroutines = 10

	for i := 0; i < goroutines; i++ {
		wg.Add(2)
		go func(n int) {
			defer wg.Done()
			s.SetPatronBarcode("barcode-" + string(rune('A'+n)))
		}(i)
		go func() {
			defer wg.Done()
			_ = s.GetPatronBarcode()
		}()
	}
	wg.Wait()
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

func TestUpdateTokens(t *testing.T) {
	session := NewSession("test-session", newTestTenantConfig())
	accessExp := time.Now().Add(10 * time.Minute)
	refreshExp := time.Now().Add(24 * time.Hour)

	session.UpdateTokens("access-1", accessExp, "refresh-1", refreshExp)

	if session.GetAuthToken() != "access-1" {
		t.Errorf("GetAuthToken() = %q, want access-1", session.GetAuthToken())
	}
	if !session.GetTokenExpiresAt().Equal(accessExp) {
		t.Errorf("GetTokenExpiresAt() = %v, want %v", session.GetTokenExpiresAt(), accessExp)
	}
	refresh, gotRefreshExp := session.GetRefreshToken()
	if refresh != "refresh-1" || !gotRefreshExp.Equal(refreshExp) {
		t.Errorf("GetRefreshToken() = (%q, %v), want (refresh-1, %v)", refresh, gotRefreshExp, refreshExp)
	}
}

func TestSetAuthenticated_KeepsRefreshToken(t *testing.T) {
	// Patron Status (23) calls SetAuthenticated with the current token; the refresh token must survive.
	session := NewSession("test-session", newTestTenantConfig())
	session.UpdateTokens("access-1", time.Now().Add(10*time.Minute), "refresh-1", time.Now().Add(time.Hour))

	session.SetAuthenticated("user", "patron-id", "barcode", "access-1", time.Now().Add(10*time.Minute))

	if refresh, _ := session.GetRefreshToken(); refresh != "refresh-1" {
		t.Errorf("refresh token = %q after SetAuthenticated, want refresh-1", refresh)
	}
}

func TestHasValidRefreshToken(t *testing.T) {
	buffer := TokenExpiryBuffer
	tests := []struct {
		name    string
		token   string
		expires time.Time
		want    bool
	}{
		{"no token", "", time.Now().Add(time.Hour), false},
		{"no expiry", "refresh-1", time.Time{}, false},
		{"expired", "refresh-1", time.Now().Add(-time.Minute), false},
		{"inside buffer", "refresh-1", time.Now().Add(buffer - 5*time.Second), false},
		{"just outside buffer", "refresh-1", time.Now().Add(buffer + 5*time.Second), true},
		{"well in future", "refresh-1", time.Now().Add(24 * time.Hour), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := NewSession("test-session", newTestTenantConfig())
			session.UpdateTokens("access-1", time.Now().Add(time.Hour), tt.token, tt.expires)
			if got := session.HasValidRefreshToken(buffer); got != tt.want {
				t.Errorf("HasValidRefreshToken() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClear_ClearsRefreshToken(t *testing.T) {
	session := NewSession("test-session", newTestTenantConfig())
	session.UpdateTokens("access-1", time.Now().Add(time.Hour), "refresh-1", time.Now().Add(time.Hour))

	session.Clear()

	if refresh, exp := session.GetRefreshToken(); refresh != "" || !exp.IsZero() {
		t.Errorf("GetRefreshToken() after Clear = (%q, %v), want empty", refresh, exp)
	}
}

func TestClone_DoesNotCopyRefreshToken(t *testing.T) {
	session := NewSession("test-session", newTestTenantConfig())
	session.UpdateTokens("access-1", time.Now().Add(time.Hour), "refresh-1", time.Now().Add(time.Hour))

	clone := session.Clone()

	if refresh, _ := clone.GetRefreshToken(); refresh != "" {
		t.Errorf("Clone copied the refresh token %q", refresh)
	}
	if clone.GetAuthToken() != "access-1" {
		t.Errorf("Clone access token = %q, want access-1", clone.GetAuthToken())
	}
}

func TestRenewalLock_Serialises(t *testing.T) {
	session := NewSession("test-session", newTestTenantConfig())
	var mu sync.Mutex
	active, maxActive := 0, 0

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			session.LockRenewal()
			defer session.UnlockRenewal()
			mu.Lock()
			active++
			if active > maxActive {
				maxActive = active
			}
			mu.Unlock()
			time.Sleep(time.Millisecond)
			mu.Lock()
			active--
			mu.Unlock()
		}()
	}
	wg.Wait()

	if maxActive != 1 {
		t.Errorf("max concurrent holders = %d, want 1", maxActive)
	}
}
