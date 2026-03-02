package server

import (
	"testing"
	"time"

	"github.com/spokanepubliclibrary/fsip2/internal/config"
	"github.com/spokanepubliclibrary/fsip2/internal/types"
)

func TestNewSession(t *testing.T) {
	tenantConfig := &config.TenantConfig{
		Tenant: "test-tenant",
	}

	sessionID := "test-session-123"
	session := types.NewSession(sessionID, tenantConfig)

	if session.ID != sessionID {
		t.Errorf("Expected session ID %s, got %s", sessionID, session.ID)
	}

	if session.TenantConfig.Tenant != "test-tenant" {
		t.Errorf("Expected tenant 'test-tenant', got %s", session.TenantConfig.Tenant)
	}

	if session.IsAuthenticated {
		t.Error("Expected new session to not be authenticated")
	}

	if session.CreatedAt.IsZero() {
		t.Error("Expected CreatedAt to be set")
	}

	if session.LastActivity.IsZero() {
		t.Error("Expected LastActivity to be set")
	}
}

func TestSessionAuthentication(t *testing.T) {
	tenantConfig := &config.TenantConfig{
		Tenant: "test-tenant",
	}

	session := types.NewSession("test-session", tenantConfig)

	// Initially not authenticated
	if session.IsAuth() {
		t.Error("Expected session to not be authenticated initially")
	}

	// Set authenticated
	expiresAt := time.Now().Add(10 * time.Minute)
	session.SetAuthenticated("testuser", "patron-123", "barcode-456", "test-token", expiresAt)
	if !session.IsAuth() {
		t.Error("Expected session to be authenticated after setting")
	}

	// Verify fields were set
	if session.GetUsername() != "testuser" {
		t.Errorf("Expected username 'testuser', got %s", session.GetUsername())
	}

	if session.GetPatronID() != "patron-123" {
		t.Errorf("Expected patron ID 'patron-123', got %s", session.GetPatronID())
	}

	if session.GetPatronBarcode() != "barcode-456" {
		t.Errorf("Expected patron barcode 'barcode-456', got %s", session.GetPatronBarcode())
	}

	if session.GetAuthToken() != "test-token" {
		t.Errorf("Expected auth token 'test-token', got %s", session.GetAuthToken())
	}
}

func TestUpdateActivity(t *testing.T) {
	tenantConfig := &config.TenantConfig{
		Tenant: "test-tenant",
	}

	session := types.NewSession("test-session", tenantConfig)

	// Wait a bit to ensure time difference
	time.Sleep(10 * time.Millisecond)

	session.UpdateActivity()

	// Note: LastActivity is updated by UpdateActivity, but we can't access the mutex directly
	// Since UpdateActivity updates LastActivity, we'll check that idle time is small
	idleTime := session.GetIdleTime()
	if idleTime >= 10*time.Millisecond {
		t.Errorf("Expected idle time to be reset after UpdateActivity, got %v", idleTime)
	}
}

func TestUpdateTenant(t *testing.T) {
	initialConfig := &config.TenantConfig{
		Tenant: "initial-tenant",
	}

	session := types.NewSession("test-session", initialConfig)

	if session.GetTenant().Tenant != "initial-tenant" {
		t.Errorf("Expected initial tenant 'initial-tenant', got %s",
			session.GetTenant().Tenant)
	}

	newConfig := &config.TenantConfig{
		Tenant: "new-tenant",
	}

	session.UpdateTenant(newConfig)

	if session.GetTenant().Tenant != "new-tenant" {
		t.Errorf("Expected updated tenant 'new-tenant', got %s",
			session.GetTenant().Tenant)
	}
}

func TestSessionConcurrentAccess(t *testing.T) {
	tenantConfig := &config.TenantConfig{
		Tenant: "test-tenant",
	}

	session := types.NewSession("test-session", tenantConfig)

	// Test concurrent reads and writes
	done := make(chan bool, 3)

	// Writer 1: Update authentication
	go func() {
		expires := time.Now().Add(10 * time.Minute)
		for i := 0; i < 100; i++ {
			session.SetAuthenticated("user", "patron", "barcode", "token", expires)
		}
		done <- true
	}()

	// Writer 2: Update institution info
	go func() {
		for i := 0; i < 100; i++ {
			session.SetInstitutionID("inst")
			session.SetLocationCode("loc")
		}
		done <- true
	}()

	// Reader: Read session data
	go func() {
		for i := 0; i < 100; i++ {
			_ = session.GetUsername()
			_ = session.IsAuth()
			_ = session.GetPatronID()
			session.UpdateActivity()
		}
		done <- true
	}()

	// Wait for all goroutines
	<-done
	<-done
	<-done

	// If we get here without deadlock or race condition, test passes
}

func TestSessionClear(t *testing.T) {
	tenantConfig := &config.TenantConfig{
		Tenant: "test-tenant",
	}

	session := types.NewSession("test-session", tenantConfig)

	// Set up session data
	expiresAt := time.Now().Add(10 * time.Minute)
	session.SetAuthenticated("testuser", "patron-123", "barcode-456", "token-123", expiresAt)
	session.SetInstitutionID("inst-001")
	session.SetLocationCode("LOC")

	// Clear session
	session.Clear()

	// Verify all cleared
	if session.IsAuth() {
		t.Error("Expected session to not be authenticated after clearing")
	}

	if session.GetAuthToken() != "" {
		t.Errorf("Expected empty auth token, got %s", session.GetAuthToken())
	}

	if session.GetUsername() != "" {
		t.Errorf("Expected empty username, got %s", session.GetUsername())
	}

	if session.GetPatronID() != "" {
		t.Errorf("Expected empty patron ID, got %s", session.GetPatronID())
	}

	if session.GetPatronBarcode() != "" {
		t.Errorf("Expected empty patron barcode, got %s", session.GetPatronBarcode())
	}
}

func TestSessionDurationAndIdle(t *testing.T) {
	tenantConfig := &config.TenantConfig{
		Tenant: "test-tenant",
	}

	session := types.NewSession("test-session", tenantConfig)

	// Wait a bit
	time.Sleep(50 * time.Millisecond)

	duration := session.GetDuration()
	idleTime := session.GetIdleTime()

	if duration < 50*time.Millisecond {
		t.Errorf("Expected duration >= 50ms, got %v", duration)
	}

	if idleTime < 50*time.Millisecond {
		t.Errorf("Expected idle time >= 50ms, got %v", idleTime)
	}

	// Update activity and check idle time resets
	session.UpdateActivity()
	newIdleTime := session.GetIdleTime()

	if newIdleTime >= idleTime {
		t.Error("Expected idle time to reset after activity update")
	}
}

// Tests for Option A: Automatic Token Refresh (Phase 3)

func TestSessionAuthCredentials(t *testing.T) {
	tenantConfig := &config.TenantConfig{
		Tenant: "test-tenant",
	}

	session := types.NewSession("test-session", tenantConfig)

	// Initially no credentials
	if session.HasAuthCredentials() {
		t.Error("Expected HasAuthCredentials to be false initially")
	}

	username, password := session.GetAuthCredentials()
	if username != "" || password != "" {
		t.Errorf("Expected empty credentials, got username=%s, password=%s", username, password)
	}

	// Set up authentication (which sets username)
	expiresAt := time.Now().Add(10 * time.Minute)
	session.SetAuthenticated("testuser", "patron-123", "barcode-456", "test-token", expiresAt)

	// Still no credentials (password not set yet)
	if session.HasAuthCredentials() {
		t.Error("Expected HasAuthCredentials to be false (no password)")
	}

	// Set credentials
	session.SetAuthCredentials("testpass")

	// Now should have credentials
	if !session.HasAuthCredentials() {
		t.Error("Expected HasAuthCredentials to be true after setting")
	}

	username, password = session.GetAuthCredentials()
	if username != "testuser" {
		t.Errorf("Expected username 'testuser', got %s", username)
	}
	if password != "testpass" {
		t.Errorf("Expected password 'testpass', got %s", password)
	}
}

func TestSessionUpdateToken(t *testing.T) {
	tenantConfig := &config.TenantConfig{
		Tenant: "test-tenant",
	}

	session := types.NewSession("test-session", tenantConfig)

	// Set initial authentication
	initialExpiry := time.Now().Add(5 * time.Minute)
	session.SetAuthenticated("testuser", "patron-123", "barcode-456", "initial-token", initialExpiry)

	// Verify initial state
	if session.GetAuthToken() != "initial-token" {
		t.Errorf("Expected initial token 'initial-token', got %s", session.GetAuthToken())
	}

	// Update token
	newExpiry := time.Now().Add(10 * time.Minute)
	session.UpdateToken("new-token", newExpiry)

	// Verify token was updated
	if session.GetAuthToken() != "new-token" {
		t.Errorf("Expected new token 'new-token', got %s", session.GetAuthToken())
	}

	// Verify expiry was updated
	tokenExpiry := session.GetTokenExpiresAt()
	if tokenExpiry.Sub(newExpiry) > time.Second || newExpiry.Sub(tokenExpiry) > time.Second {
		t.Errorf("Expected token expiry around %v, got %v", newExpiry, tokenExpiry)
	}

	// Verify other session state is preserved
	if session.GetUsername() != "testuser" {
		t.Errorf("Expected username preserved as 'testuser', got %s", session.GetUsername())
	}
	if session.GetPatronID() != "patron-123" {
		t.Errorf("Expected patron ID preserved as 'patron-123', got %s", session.GetPatronID())
	}
}

func TestSessionClearClearsCredentials(t *testing.T) {
	tenantConfig := &config.TenantConfig{
		Tenant: "test-tenant",
	}

	session := types.NewSession("test-session", tenantConfig)

	// Set up session with authentication and credentials
	expiresAt := time.Now().Add(10 * time.Minute)
	session.SetAuthenticated("testuser", "patron-123", "barcode-456", "test-token", expiresAt)
	session.SetAuthCredentials("testpass")

	// Verify credentials are set
	if !session.HasAuthCredentials() {
		t.Error("Expected HasAuthCredentials to be true before clear")
	}

	// Clear session
	session.Clear()

	// Verify credentials are cleared
	if session.HasAuthCredentials() {
		t.Error("Expected HasAuthCredentials to be false after clear")
	}

	username, password := session.GetAuthCredentials()
	if username != "" || password != "" {
		t.Errorf("Expected cleared credentials, got username=%s, password=%s", username, password)
	}
}

func TestSessionCloneDoesNotCopyPassword(t *testing.T) {
	tenantConfig := &config.TenantConfig{
		Tenant: "test-tenant",
	}

	session := types.NewSession("test-session", tenantConfig)

	// Set up session with authentication and credentials
	expiresAt := time.Now().Add(10 * time.Minute)
	session.SetAuthenticated("testuser", "patron-123", "barcode-456", "test-token", expiresAt)
	session.SetAuthCredentials("testpass")

	// Clone the session
	cloned := session.Clone()

	// Verify other fields are copied
	if cloned.GetUsername() != "testuser" {
		t.Errorf("Expected cloned username 'testuser', got %s", cloned.GetUsername())
	}
	if cloned.GetAuthToken() != "test-token" {
		t.Errorf("Expected cloned token 'test-token', got %s", cloned.GetAuthToken())
	}

	// Verify password is NOT copied (for security)
	if cloned.HasAuthCredentials() {
		t.Error("Expected cloned session to NOT have credentials (password should not be copied)")
	}

	_, password := cloned.GetAuthCredentials()
	if password != "" {
		t.Errorf("Expected empty password in cloned session, got %s", password)
	}
}

func TestSessionCredentialsConcurrentAccess(t *testing.T) {
	tenantConfig := &config.TenantConfig{
		Tenant: "test-tenant",
	}

	session := types.NewSession("test-session", tenantConfig)

	// Set initial authentication
	expires := time.Now().Add(10 * time.Minute)
	session.SetAuthenticated("testuser", "patron", "barcode", "token", expires)

	done := make(chan bool, 4)

	// Writer 1: Update credentials
	go func() {
		for i := 0; i < 100; i++ {
			session.SetAuthCredentials("pass")
		}
		done <- true
	}()

	// Writer 2: Update token
	go func() {
		for i := 0; i < 100; i++ {
			session.UpdateToken("newtoken", time.Now().Add(10*time.Minute))
		}
		done <- true
	}()

	// Reader 1: Check credentials
	go func() {
		for i := 0; i < 100; i++ {
			_ = session.HasAuthCredentials()
			_, _ = session.GetAuthCredentials()
		}
		done <- true
	}()

	// Reader 2: Check token
	go func() {
		for i := 0; i < 100; i++ {
			_ = session.GetAuthToken()
			_ = session.IsTokenExpired()
		}
		done <- true
	}()

	// Wait for all goroutines
	<-done
	<-done
	<-done
	<-done

	// If we get here without deadlock or race condition, test passes
}
