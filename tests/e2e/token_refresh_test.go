// +build e2e

package e2e

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/spokanepubliclibrary/fsip2/internal/folio/models"
	"github.com/spokanepubliclibrary/fsip2/tests/testutil"
)

// TestE2E_TokenRefresh_AutomaticRefreshOnExpiry tests that when a FOLIO token expires
// during an active SIP2 session, the server automatically refreshes the token and
// the patron operation succeeds without any client-visible error.
//
// This is the core test for Phase 3 (Option A: Automatic Token Refresh).
func TestE2E_TokenRefresh_AutomaticRefreshOnExpiry(t *testing.T) {
	setup := NewE2ESetup(t)
	defer setup.Close(t)

	// Configure short-lived tokens (3 seconds)
	// The session's IsTokenExpired() uses a 90s buffer, so with ExpiresIn=3,
	// the token will be considered expired almost immediately after login.
	// However, the auth.go Login() calculates expiresAt = now + ExpiresIn seconds.
	// The session's IsTokenExpired() checks: now + 90s > expiresAt
	// With ExpiresIn=3: expiresAt = now+3s, so at now+0: now+90s > now+3s => true (expired!)
	// We need the token to be valid initially, so we need ExpiresIn > 90.
	// Use 93 seconds so token is valid for ~3 seconds after the 90s buffer.
	// Actually, for a faster test, we can use a smaller value. The key is:
	// - Token must be valid at login time (first request succeeds)
	// - Token must be expired by the time we send the second request
	//
	// With ExpiresIn=92: expiresAt = now+92s. IsTokenExpired: now+90s > now+92s => false (valid for ~2s)
	// After 3s: (now+3)+90 > now+92 => now+93 > now+92 => true (expired)
	//
	// But we also need the mock server to actually reject the expired token.
	// The mock checks: now > createdAt + TokenLifetime
	// With TokenLifetime=92 and 3s wait: now > createdAt+92 => false (not expired at mock level)
	//
	// The trick: we need the SIP2 server to think the token is expired (via 90s buffer)
	// but the mock FOLIO server should accept the NEW token from re-authentication.
	// The mock server doesn't need to reject the old token - the SIP2 server's
	// IsTokenExpired() check is what triggers the refresh.
	//
	// So: set TokenLifetime to 92 (ExpiresIn=92), wait 3 seconds, and the SIP2 server
	// will detect expiry and refresh. The mock will issue a new token on re-login.
	setup.MockFolio.SetTokenLifetime(92)

	setup.MockFolio.AddUser("PATRON_REFRESH", &models.User{
		ID:       "user-token-refresh",
		Username: "patron_refresh",
		Barcode:  "PATRON_REFRESH",
		Active:   true,
		Personal: models.PersonalInfo{FirstName: "Token", LastName: "Refresh", Email: "refresh@example.com"},
	})

	conn := setup.Connect(t)
	defer conn.Close()

	// Step 1: Login
	t.Log("Step 1: Login with short-lived token")
	setup.Login(t, conn)
	initialLoginCount := setup.MockFolio.GetLoginCount()
	t.Logf("Login successful, login count: %d", initialLoginCount)

	// Step 2: Immediate patron status (should use cached token)
	t.Log("Step 2: Immediate patron status (token still valid)")
	resp := setup.Exchange(t, conn, testutil.NewPatronStatusMessage("test-inst", "PATRON_REFRESH"))
	assert.True(t, len(resp) >= 2 && resp[:2] == "24", "Expected Patron Status Response (24), got: %s", resp)

	// Step 3: Wait for token to expire (past the 90s buffer)
	t.Log("Step 3: Waiting for token to expire past 90s buffer...")
	time.Sleep(4 * time.Second)

	// Step 4: Patron status after token expiry (should trigger automatic refresh)
	t.Log("Step 4: Patron status after token expiry (should auto-refresh)")
	resp = setup.Exchange(t, conn, testutil.NewPatronStatusMessage("test-inst", "PATRON_REFRESH"))
	assert.True(t, len(resp) >= 2 && resp[:2] == "24", "Expected Patron Status Response (24) after token refresh, got: %s", resp)

	// Verify that the renewal used /authn/refresh and did not re-login
	assert.Equal(t, 1, setup.MockFolio.GetRefreshCount(), "Expected one /authn/refresh call")
	assert.Equal(t, initialLoginCount, setup.MockFolio.GetLoginCount(), "Refresh must not re-login")
	assert.Zero(t, setup.MockFolio.GetDeprecatedLoginCount(), "The deprecated /authn/login must never be called")

	t.Log("Automatic token refresh on expiry test passed")
}

// TestE2E_TokenRefresh_MultipleOperationsAfterRefresh tests that after a token refresh,
// subsequent operations continue to work without requiring another refresh.
func TestE2E_TokenRefresh_MultipleOperationsAfterRefresh(t *testing.T) {
	setup := NewE2ESetup(t)
	defer setup.Close(t)

	// Short-lived token: valid for ~2 seconds with 90s buffer
	setup.MockFolio.SetTokenLifetime(92)

	setup.MockFolio.AddUser("PATRON_MULTI", &models.User{
		ID:       "user-multi-ops",
		Username: "patron_multi",
		Barcode:  "PATRON_MULTI",
		Active:   true,
		Personal: models.PersonalInfo{FirstName: "Multi", LastName: "Ops"},
	})
	setup.MockFolio.AddItem("ITEM_MULTI", &models.Item{
		ID:      "item-multi",
		Barcode: "ITEM_MULTI",
		Status:  models.ItemStatus{Name: "Available"},
	})

	conn := setup.Connect(t)
	defer conn.Close()
	setup.Login(t, conn)

	// Wait for token to expire
	t.Log("Waiting for token to expire...")
	time.Sleep(4 * time.Second)

	renewalsBefore := renewalCount(setup)

	// First operation after expiry - triggers refresh
	t.Log("First operation after expiry (triggers refresh)")
	resp := setup.Exchange(t, conn, testutil.NewPatronStatusMessage("test-inst", "PATRON_MULTI"))
	assert.True(t, len(resp) >= 2 && resp[:2] == "24", "Expected Patron Status Response (24), got: %s", resp)

	renewalsAfter := renewalCount(setup)
	assert.Greater(t, renewalsAfter, renewalsBefore, "Expected renewal to occur")

	// Second operation - should use the refreshed token (new token has 92s lifetime)
	t.Log("Second operation (should use refreshed token)")
	resp = setup.Exchange(t, conn, testutil.NewItemInformationMessage("test-inst", "ITEM_MULTI"))
	assert.True(t, len(resp) >= 2 && resp[:2] == "18", "Expected Item Information Response (18), got: %s", resp)

	// Third operation - should still use the same refreshed token
	t.Log("Third operation (should still use refreshed token)")
	resp = setup.Exchange(t, conn, testutil.NewPatronStatusMessage("test-inst", "PATRON_MULTI"))
	assert.True(t, len(resp) >= 2 && resp[:2] == "24", "Expected Patron Status Response (24), got: %s", resp)

	// Verify no additional renewals occurred for the second and third operations
	assert.Equal(t, renewalsAfter, renewalCount(setup),
		"Expected no additional renewals after the first one")

	t.Log("Multiple operations after refresh test passed")
}

// TestE2E_TokenRefresh_RefreshFailure_FOLIOUnavailable tests behavior when
// FOLIO becomes unavailable during token refresh. The operation should fail
// gracefully with an appropriate SIP2 error response.
func TestE2E_TokenRefresh_RefreshFailure_FOLIOUnavailable(t *testing.T) {
	setup := NewE2ESetup(t)
	defer setup.Close(t)

	setup.MockFolio.SetTokenLifetime(92)

	setup.MockFolio.AddUser("PATRON_RFAIL", &models.User{
		ID:       "user-refresh-fail",
		Username: "patron_rfail",
		Barcode:  "PATRON_RFAIL",
		Active:   true,
		Personal: models.PersonalInfo{FirstName: "Refresh", LastName: "Fail"},
	})

	conn := setup.Connect(t)
	defer conn.Close()

	// Login successfully first
	setup.Login(t, conn)

	// Wait for token to expire
	t.Log("Waiting for token to expire...")
	time.Sleep(4 * time.Second)

	// Make FOLIO reject refreshes and logins (simulate unavailability during renewal)
	setup.MockFolio.SetRejectRefresh(true)
	setup.MockFolio.SetRejectLogins(true)

	// Try patron status - refresh should fail, but should still get a SIP2 response
	t.Log("Patron status with FOLIO unavailable (refresh should fail gracefully)")
	resp := setup.Exchange(t, conn, testutil.NewPatronStatusMessage("test-inst", "PATRON_RFAIL"))
	assert.True(t, len(resp) >= 2, "Expected valid SIP2 response even when refresh fails, got: %s", resp)
	t.Logf("Response when refresh fails: %s", resp)

	// Re-enable refreshes and logins
	setup.MockFolio.SetRejectRefresh(false)
	setup.MockFolio.SetRejectLogins(false)

	t.Log("Refresh failure test passed - server handled gracefully")
}

// TestE2E_TokenRefresh_RecoveryAfterFailure tests that after a refresh failure,
// the system can recover when FOLIO becomes available again.
func TestE2E_TokenRefresh_RecoveryAfterFailure(t *testing.T) {
	setup := NewE2ESetup(t)
	defer setup.Close(t)

	setup.MockFolio.SetTokenLifetime(92)

	setup.MockFolio.AddUser("PATRON_RECOVERY", &models.User{
		ID:       "user-recovery",
		Username: "patron_recovery",
		Barcode:  "PATRON_RECOVERY",
		Active:   true,
		Personal: models.PersonalInfo{FirstName: "Recovery", LastName: "Test"},
	})

	conn := setup.Connect(t)
	defer conn.Close()
	setup.Login(t, conn)

	// Wait for token to expire
	t.Log("Waiting for token to expire...")
	time.Sleep(4 * time.Second)

	// Make FOLIO reject refreshes and logins temporarily
	setup.MockFolio.SetRejectRefresh(true)
	setup.MockFolio.SetRejectLogins(true)

	t.Log("First attempt: FOLIO unavailable")
	resp := setup.Exchange(t, conn, testutil.NewPatronStatusMessage("test-inst", "PATRON_RECOVERY"))
	assert.True(t, len(resp) >= 2, "Expected valid response, got: %s", resp)
	t.Logf("Response during outage: %s", resp)

	// Re-enable refreshes and logins - FOLIO is back
	setup.MockFolio.SetRejectRefresh(false)
	setup.MockFolio.SetRejectLogins(false)

	t.Log("Second attempt: FOLIO recovered")
	resp = setup.Exchange(t, conn, testutil.NewPatronStatusMessage("test-inst", "PATRON_RECOVERY"))
	assert.True(t, len(resp) >= 2 && resp[:2] == "24", "Expected Patron Status Response (24) after FOLIO recovery, got: %s", resp)

	t.Log("Recovery after failure test passed")
}

// TestE2E_TokenRefresh_CheckoutAfterExpiry tests that checkout operations
// (which involve multiple FOLIO API calls) work correctly after token refresh.
func TestE2E_TokenRefresh_CheckoutAfterExpiry(t *testing.T) {
	setup := NewE2ESetup(t)
	defer setup.Close(t)

	setup.MockFolio.SetTokenLifetime(92)

	setup.MockFolio.AddUser("PATRON_COREFRESH", &models.User{
		ID:       "user-checkout-refresh",
		Username: "patron_corefresh",
		Barcode:  "PATRON_COREFRESH",
		Active:   true,
		Personal: models.PersonalInfo{FirstName: "Checkout", LastName: "Refresh"},
	})
	setup.MockFolio.AddItem("ITEM_COREFRESH", &models.Item{
		ID:      "item-co-refresh",
		Barcode: "ITEM_COREFRESH",
		Status:  models.ItemStatus{Name: "Available"},
	})

	conn := setup.Connect(t)
	defer conn.Close()
	setup.Login(t, conn)

	// Wait for token to expire
	t.Log("Waiting for token to expire...")
	time.Sleep(4 * time.Second)

	// Checkout after token expiry - should trigger refresh and succeed
	t.Log("Checkout after token expiry")
	resp := setup.Exchange(t, conn, testutil.NewCheckoutMessage("test-inst", "PATRON_COREFRESH", "ITEM_COREFRESH"))
	assert.True(t, len(resp) >= 2 && resp[:2] == "12", "Expected Checkout Response (12) after token refresh, got: %s", resp)

	t.Log("Checkout after token expiry test passed")
}

// TestE2E_TokenRefresh_SessionSurvivesMultipleExpirations tests that a session
// can survive multiple token expirations by refreshing each time.
func TestE2E_TokenRefresh_SessionSurvivesMultipleExpirations(t *testing.T) {
	setup := NewE2ESetup(t)
	defer setup.Close(t)

	// Very short effective lifetime: ExpiresIn=92 means ~2s before 90s buffer expires it
	setup.MockFolio.SetTokenLifetime(92)

	setup.MockFolio.AddUser("PATRON_MULTIEXP", &models.User{
		ID:       "user-multi-expire",
		Username: "patron_multiexp",
		Barcode:  "PATRON_MULTIEXP",
		Active:   true,
		Personal: models.PersonalInfo{FirstName: "Multi", LastName: "Expire"},
	})

	conn := setup.Connect(t)
	defer conn.Close()
	setup.Login(t, conn)

	// Cycle through multiple expiration/refresh cycles
	for cycle := 1; cycle <= 3; cycle++ {
		t.Logf("Cycle %d: Waiting for token to expire...", cycle)
		time.Sleep(4 * time.Second)

		refreshesBefore := setup.MockFolio.GetRefreshCount()

		t.Logf("Cycle %d: Sending patron status (should trigger refresh)", cycle)
		resp := setup.Exchange(t, conn, testutil.NewPatronStatusMessage("test-inst", "PATRON_MULTIEXP"))
		assert.True(t, len(resp) >= 2 && resp[:2] == "24",
			"Cycle %d: Expected Patron Status Response (24), got: %s", cycle, resp)

		assert.Greater(t, setup.MockFolio.GetRefreshCount(), refreshesBefore,
			"Cycle %d: Expected a /authn/refresh call", cycle)
	}

	// Refresh replaces re-login: one login for the whole session, a refresh per expiry
	assert.Equal(t, 1, setup.MockFolio.GetLoginCount(), "Expected exactly one login")
	assert.GreaterOrEqual(t, setup.MockFolio.GetRefreshCount(), 3, "Expected a refresh per expiry")

	t.Log("Session survived multiple token expirations successfully")
}

// renewalCount is the number of token renewals the mock has served (refreshes + logins).
func renewalCount(setup *E2ESetup) int {
	return setup.MockFolio.GetRefreshCount() + setup.MockFolio.GetLoginCount()
}

// TestE2E_TokenRefresh_ExpiredRefreshToken tests that when the refresh token itself has
// expired, the session re-logs in exactly once and never attempts a refresh.
func TestE2E_TokenRefresh_ExpiredRefreshToken(t *testing.T) {
	setup := NewE2ESetup(t)
	defer setup.Close(t)

	// Both tokens are usable for ~2-3s past the 90s buffer
	setup.MockFolio.SetTokenLifetime(92)
	setup.MockFolio.SetRefreshTokenLifetime(93)

	setup.MockFolio.AddUser("PATRON_RTEXP", &models.User{
		ID:       "user-rt-expired",
		Username: "patron_rtexp",
		Barcode:  "PATRON_RTEXP",
		Active:   true,
		Personal: models.PersonalInfo{FirstName: "Refresh", LastName: "Expired"},
	})

	conn := setup.Connect(t)
	defer conn.Close()
	setup.Login(t, conn)
	loginsBefore := setup.MockFolio.GetLoginCount()

	t.Log("Idling past the refresh token's expiry...")
	time.Sleep(5 * time.Second)

	resp := setup.Exchange(t, conn, testutil.NewPatronStatusMessage("test-inst", "PATRON_RTEXP"))
	assert.True(t, len(resp) >= 2 && resp[:2] == "24", "Expected Patron Status Response (24), got: %s", resp)

	assert.Equal(t, loginsBefore+1, setup.MockFolio.GetLoginCount(), "Expected exactly one re-login")
	assert.Zero(t, setup.MockFolio.GetRefreshCount(), "An expired refresh token must not be used")
}

// TestE2E_TokenRefresh_RefreshRejected tests that a rejected refresh falls back to
// re-login and the SIP message still succeeds.
func TestE2E_TokenRefresh_RefreshRejected(t *testing.T) {
	setup := NewE2ESetup(t)
	defer setup.Close(t)

	setup.MockFolio.SetTokenLifetime(92)

	setup.MockFolio.AddUser("PATRON_RTREJ", &models.User{
		ID:       "user-rt-rejected",
		Username: "patron_rtrej",
		Barcode:  "PATRON_RTREJ",
		Active:   true,
		Personal: models.PersonalInfo{FirstName: "Refresh", LastName: "Rejected"},
	})

	conn := setup.Connect(t)
	defer conn.Close()
	setup.Login(t, conn)
	loginsBefore := setup.MockFolio.GetLoginCount()

	time.Sleep(4 * time.Second)
	setup.MockFolio.SetRejectRefresh(true)

	resp := setup.Exchange(t, conn, testutil.NewPatronStatusMessage("test-inst", "PATRON_RTREJ"))
	assert.True(t, len(resp) >= 2 && resp[:2] == "24", "Expected Patron Status Response (24), got: %s", resp)
	assert.Equal(t, loginsBefore+1, setup.MockFolio.GetLoginCount(), "Rejected refresh must fall back to re-login")
}

// TestE2E_TokenRefresh_ShortLifetimeTenant tests a tenant whose access tokens live for
// less than the 90s buffer. fsip2 must honour the server's expiry (the old code assumed
// 600s) and renew before every FOLIO-bound message, so the mock never sees an expired token.
func TestE2E_TokenRefresh_ShortLifetimeTenant(t *testing.T) {
	setup := NewE2ESetup(t)
	defer setup.Close(t)

	// The mock rejects these tokens after 2s
	setup.MockFolio.SetTokenLifetime(2)

	setup.MockFolio.AddUser("PATRON_SHORT", &models.User{
		ID:       "user-short-lifetime",
		Username: "patron_short",
		Barcode:  "PATRON_SHORT",
		Active:   true,
		Personal: models.PersonalInfo{FirstName: "Short", LastName: "Lifetime"},
	})

	conn := setup.Connect(t)
	defer conn.Close()
	setup.Login(t, conn)

	for i := 1; i <= 3; i++ {
		time.Sleep(2500 * time.Millisecond)
		resp := setup.Exchange(t, conn, testutil.NewPatronStatusMessage("test-inst", "PATRON_SHORT"))
		assert.True(t, len(resp) >= 2 && resp[:2] == "24",
			"Message %d: Expected Patron Status Response (24) with no 401s, got: %s", i, resp)
	}

	assert.GreaterOrEqual(t, setup.MockFolio.GetRefreshCount(), 3, "Expected a refresh before each message")
	assert.Zero(t, setup.MockFolio.GetDeprecatedLoginCount(), "The deprecated /authn/login must never be called")
}
