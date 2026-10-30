// +build e2e

package e2e

import (
	"net"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/spokanepubliclibrary/fsip2/internal/folio/models"
	"github.com/spokanepubliclibrary/fsip2/tests/testutil"
)

// Keycloak session cleanup (plan 01b). Every test also asserts that the blank-refresh-token
// trap never fired and that no logout carried X-Okapi-Token: either would end the session
// of every SIP device sharing the kiosk username.

func assertNoTrap(t *testing.T, setup *E2ESetup) {
	t.Helper()
	assert.Zero(t, setup.MockFolio.GetTrapTriggeredCount(), "logout trap must never fire")
	assert.Zero(t, setup.MockFolio.GetLogoutHeaderCount(), "logout must never send X-Okapi-Token")
}

func waitForLogouts(t *testing.T, setup *E2ESetup, want int) {
	t.Helper()
	require.Eventually(t, func() bool { return setup.MockFolio.GetLogoutCount() >= want },
		5*time.Second, 20*time.Millisecond, "expected %d logouts", want)
	time.Sleep(100 * time.Millisecond) // let any unexpected extra logout arrive
	assert.Equal(t, want, setup.MockFolio.GetLogoutCount())
}

func addLogoutPatron(setup *E2ESetup) {
	setup.MockFolio.AddUser("PATRON_LOGOUT", &models.User{
		ID:       "user-logout",
		Username: "patron_logout",
		Barcode:  "PATRON_LOGOUT",
		Active:   true,
		Personal: models.PersonalInfo{FirstName: "Logout", LastName: "Test"},
	})
}

func patronStatusOK(t *testing.T, setup *E2ESetup, conn net.Conn) {
	t.Helper()
	resp := setup.Exchange(t, conn, testutil.NewPatronStatusMessage("test-inst", "PATRON_LOGOUT"))
	require.True(t, strings.HasPrefix(resp, "24"), "expected Patron Status Response (24), got: %s", resp)
}

// TestE2E_Logout_MultiAMHIsolation: 8 devices share one kiosk username (one Keycloak session
// each). Closing 3 ends exactly those 3; the other 5 keep working.
func TestE2E_Logout_MultiAMHIsolation(t *testing.T) {
	setup := NewE2ESetup(t)
	defer setup.Close(t)
	addLogoutPatron(setup)

	const devices = 8
	conns := make([]net.Conn, devices)
	for i := range conns {
		conns[i] = setup.Connect(t)
		setup.Login(t, conns[i])
	}
	require.Equal(t, devices, setup.MockFolio.GetLoginCount())

	for i := 0; i < 3; i++ {
		conns[i].Close()
	}
	waitForLogouts(t, setup, 3)

	for i := 3; i < devices; i++ {
		patronStatusOK(t, setup, conns[i])
		conns[i].Close()
	}
	waitForLogouts(t, setup, devices)
	assertNoTrap(t, setup)
}

// TestE2E_Logout_HeartbeatOnlyConnection: SC Status (99) never renews the token, so at close
// the access token has expired; fsip2 refreshes first, then logs out.
func TestE2E_Logout_HeartbeatOnlyConnection(t *testing.T) {
	setup := NewE2ESetup(t)
	defer setup.Close(t)
	setup.MockFolio.SetTokenLifetime(2)

	conn := setup.Connect(t)
	setup.Login(t, conn)
	for i := 0; i < 3; i++ {
		time.Sleep(1 * time.Second)
		resp := setup.Exchange(t, conn, "9900302.00\r")
		require.True(t, strings.HasPrefix(resp, "98"), "expected SC Status response, got: %s", resp)
	}
	assert.Zero(t, setup.MockFolio.GetRefreshCount(), "heartbeats must not renew the token")

	conn.Close()
	waitForLogouts(t, setup, 1)
	assert.Equal(t, 1, setup.MockFolio.GetRefreshCount(), "expected one refresh before the logout")
	assertNoTrap(t, setup)
}

// TestE2E_Logout_Relogin93: a 93 after access expiry logs in again; the replaced Keycloak
// session is ended in the background and the connection keeps working on the new one.
func TestE2E_Logout_Relogin93(t *testing.T) {
	setup := NewE2ESetup(t)
	defer setup.Close(t)
	setup.MockFolio.SetTokenLifetime(92) // expired for fsip2 (90 s buffer) after ~2 s
	addLogoutPatron(setup)

	conn := setup.Connect(t)
	defer conn.Close()
	setup.Login(t, conn)
	time.Sleep(3 * time.Second)

	setup.Login(t, conn)
	assert.Equal(t, 2, setup.MockFolio.GetLoginCount())
	waitForLogouts(t, setup, 1)

	patronStatusOK(t, setup, conn)
	assertNoTrap(t, setup)
}

// TestE2E_Logout_Relogin93_ValidToken_NoLogout: a repeated 93 with the same credentials and
// a valid token is short-circuited, so no new session and no logout.
func TestE2E_Logout_Relogin93_ValidToken_NoLogout(t *testing.T) {
	setup := NewE2ESetup(t)
	defer setup.Close(t)

	conn := setup.Connect(t)
	defer conn.Close()
	setup.Login(t, conn)
	setup.Login(t, conn)

	assert.Equal(t, 1, setup.MockFolio.GetLoginCount())
	time.Sleep(200 * time.Millisecond)
	assert.Zero(t, setup.MockFolio.GetLogoutCount())
}

// TestE2E_Logout_FailedRelogin_KeepsSession: a failed 93 on the same tenant leaves the
// existing session (and its Keycloak session) untouched.
func TestE2E_Logout_FailedRelogin_KeepsSession(t *testing.T) {
	setup := NewE2ESetup(t)
	defer setup.Close(t)
	addLogoutPatron(setup)

	conn := setup.Connect(t)
	defer conn.Close()
	setup.Login(t, conn)

	resp := setup.Exchange(t, conn, testutil.NewLoginMessage("testuser", "wrong", testServicePointUUID))
	require.Contains(t, resp, "940")
	time.Sleep(200 * time.Millisecond)
	assert.Zero(t, setup.MockFolio.GetLogoutCount())

	patronStatusOK(t, setup, conn)
}

// TestE2E_Logout_EndPatronSession_NoLogout: SIP 35 is patron-level; the kiosk session stays.
func TestE2E_Logout_EndPatronSession_NoLogout(t *testing.T) {
	setup := NewE2ESetup(t)
	defer setup.Close(t)
	addLogoutPatron(setup)

	conn := setup.Connect(t)
	defer conn.Close()
	setup.Login(t, conn)

	resp := setup.Exchange(t, conn, "3520250110    081500|AOtest-inst|AAPATRON_LOGOUT\r")
	require.True(t, strings.HasPrefix(resp, "36"), "expected End Session response, got: %s", resp)
	time.Sleep(200 * time.Millisecond)
	assert.Zero(t, setup.MockFolio.GetLogoutCount())
	patronStatusOK(t, setup, conn)
}

// TestE2E_Logout_NeverLoggedIn_NoRequest: a connection that never sent a 93 sends nothing.
func TestE2E_Logout_NeverLoggedIn_NoRequest(t *testing.T) {
	setup := NewE2ESetup(t)
	defer setup.Close(t)

	conn := setup.Connect(t)
	resp := setup.Exchange(t, conn, "9900302.00\r")
	require.True(t, strings.HasPrefix(resp, "98"))
	conn.Close()

	time.Sleep(300 * time.Millisecond)
	assert.Empty(t, setup.MockFolio.GetLogoutTenants())
}
