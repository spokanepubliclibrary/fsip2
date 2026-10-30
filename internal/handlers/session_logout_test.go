package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/spokanepubliclibrary/fsip2/internal/config"
	"github.com/spokanepubliclibrary/fsip2/internal/folio"
	"github.com/spokanepubliclibrary/fsip2/internal/types"
	"github.com/spokanepubliclibrary/fsip2/tests/mocks"
	"github.com/spokanepubliclibrary/fsip2/tests/testutil"
)

// setAccessExpiry changes only the stored access-token expiry, keeping both tokens.
func setAccessExpiry(sess *types.Session, exp time.Time) {
	refresh, refreshExp := sess.GetRefreshToken()
	sess.UpdateTokens(sess.GetAuthToken(), exp, refresh, refreshExp)
}

// newMockTenant starts a mock FOLIO and returns it with a tenant config pointing at it.
func newMockTenant(t *testing.T, name string) (*mocks.FolioMockServer, *config.TenantConfig) {
	t.Helper()
	m := mocks.NewFolioMockServer()
	t.Cleanup(m.Close)
	return m, testutil.NewTenantConfig(testutil.WithOkapiURL(m.GetURL()), testutil.WithTenant(name))
}

// capturedLogout records logouts requested through BaseHandler.logoutReplacedTokens.
type capturedLogout struct {
	pair     types.TokenPair
	username string
	reason   string
}

type logoutRecorder struct {
	mu    sync.Mutex
	calls []capturedLogout
}

// install makes h run replaced-pair logouts synchronously and records them.
func (r *logoutRecorder) install(h *BaseHandler) {
	h.logoutReplacedTokens = func(pair types.TokenPair, sessionID, username, reason string) {
		if pair.IsEmpty() {
			return
		}
		r.mu.Lock()
		r.calls = append(r.calls, capturedLogout{pair: pair, username: username, reason: reason})
		r.mu.Unlock()
		LogoutTokens(pair, zap.NewNop(), sessionID, username, reason)
	}
}

func (r *logoutRecorder) all() []capturedLogout {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]capturedLogout(nil), r.calls...)
}

// --- LogoutTokens / LogoutSession -------------------------------------------------------

func TestLogoutSession_CurrentAccess_LogsOutWithoutRefresh(t *testing.T) {
	mockFolio, tc := newMockTenant(t, "a")
	sess := loginSession(t, zap.NewNop(), tc)
	refresh, _ := sess.GetRefreshToken()

	core, recorded := observer.New(zap.DebugLevel)
	LogoutSession(sess, zap.New(core), LogoutReasonClientClosed)

	assert.Equal(t, 1, mockFolio.GetLogoutCount())
	assert.Equal(t, 0, mockFolio.GetRefreshCount())
	assert.Equal(t, 0, mockFolio.GetTrapTriggeredCount())
	assert.Equal(t, 0, mockFolio.GetLogoutHeaderCount(), "X-Okapi-Token must never be sent")
	assert.False(t, mockFolio.IsRefreshTokenAlive(refresh), "Keycloak session should have ended")

	accepted := recorded.FilterMessage("Keycloak logout accepted").All()
	require.Len(t, accepted, 1)
	fields := accepted[0].ContextMap()
	assert.Equal(t, LogoutReasonClientClosed, fields["reason"])
	assert.Equal(t, sess.ID, fields["session_id"])
	assert.Equal(t, "testuser", fields["username"])
	assert.Equal(t, false, fields["refreshed_first"])
}

func TestLogoutSession_LeavesNoTokens(t *testing.T) {
	_, tc := newMockTenant(t, "a")
	sess := loginSession(t, zap.NewNop(), tc)

	LogoutSession(sess, zap.NewNop(), LogoutReasonClientClosed)

	assert.Empty(t, sess.GetAuthToken())
	refresh, _ := sess.GetRefreshToken()
	assert.Empty(t, refresh)
	assert.True(t, sess.TakeTokens().IsEmpty())
}

// TestLogoutSession_ExpiredAccess_RefreshesFirst: heartbeats (99) never renew the token, so
// at close the access token is often long expired. The gateway would answer 401 (V4b-4).
func TestLogoutSession_ExpiredAccess_RefreshesFirst(t *testing.T) {
	mockFolio, tc := newMockTenant(t, "a")
	sess := loginSession(t, zap.NewNop(), tc)
	refresh, _ := sess.GetRefreshToken()
	access := sess.GetAuthToken()
	setAccessExpiry(sess, time.Now().Add(-2*time.Hour))
	mockFolio.ExpireAccessToken(access, time.Now().Add(-2*time.Hour))

	core, recorded := observer.New(zap.DebugLevel)
	LogoutSession(sess, zap.New(core), LogoutReasonReadTimeout)

	assert.Equal(t, 1, mockFolio.GetRefreshCount())
	assert.Equal(t, 1, mockFolio.GetLogoutCount())
	assert.False(t, mockFolio.IsRefreshTokenAlive(refresh))
	accepted := recorded.FilterMessage("Keycloak logout accepted").All()
	require.Len(t, accepted, 1)
	assert.Equal(t, true, accepted[0].ContextMap()["refreshed_first"])
}

// TestLogoutTokens_UnexpectedUnauthorized_RefreshesAndRetries: fsip2 thinks the access
// token is current, but the gateway says 401 (clock drift). One refresh, then logout.
func TestLogoutTokens_UnexpectedUnauthorized_RefreshesAndRetries(t *testing.T) {
	mockFolio, tc := newMockTenant(t, "a")
	sess := loginSession(t, zap.NewNop(), tc)
	mockFolio.ExpireAccessToken(sess.GetAuthToken(), time.Now().Add(-5*time.Minute))

	LogoutSession(sess, zap.NewNop(), LogoutReasonClientClosed)

	assert.Equal(t, 1, mockFolio.GetRefreshCount())
	assert.Equal(t, 1, mockFolio.GetLogoutCount())
}

func TestLogoutSession_MissingAccess_RefreshesFirst(t *testing.T) {
	mockFolio, tc := newMockTenant(t, "a")
	sess := loginSession(t, zap.NewNop(), tc)
	refresh, refreshExp := sess.GetRefreshToken()
	sess.UpdateTokens("", time.Time{}, refresh, refreshExp)

	LogoutSession(sess, zap.NewNop(), LogoutReasonClientClosed)

	assert.Equal(t, 1, mockFolio.GetRefreshCount())
	assert.Equal(t, 1, mockFolio.GetLogoutCount())
	assert.Equal(t, 0, mockFolio.GetTrapTriggeredCount())
}

func TestLogoutSession_RefreshRejected_NoLogout(t *testing.T) {
	mockFolio, tc := newMockTenant(t, "a")
	sess := loginSession(t, zap.NewNop(), tc)
	setAccessExpiry(sess, time.Now().Add(-time.Hour))
	mockFolio.InvalidateAllTokens() // the session already ended server-side

	LogoutSession(sess, zap.NewNop(), LogoutReasonClientClosed)

	assert.Equal(t, 0, mockFolio.GetLogoutCount())
	assert.Empty(t, mockFolio.GetLogoutTenants(), "no logout request expected")
}

func TestLogoutSession_RefreshUnavailable_WarnsNoLogout(t *testing.T) {
	mockFolio, tc := newMockTenant(t, "a")
	sess := loginSession(t, zap.NewNop(), tc)
	setAccessExpiry(sess, time.Now().Add(-time.Hour))
	mockFolio.SetRefreshStatus(http.StatusServiceUnavailable)

	core, recorded := observer.New(zap.DebugLevel)
	LogoutSession(sess, zap.New(core), LogoutReasonClientClosed)

	assert.Empty(t, mockFolio.GetLogoutTenants())
	failed := recorded.FilterMessage("Keycloak logout failed").All()
	require.Len(t, failed, 1)
	assert.Equal(t, zap.WarnLevel, failed[0].Level)
	assert.Equal(t, "refresh", failed[0].ContextMap()["stage"])
	assert.EqualValues(t, http.StatusServiceUnavailable, failed[0].ContextMap()["status_code"])
}

func TestLogoutTokens_SkipsWithoutAnyRequest(t *testing.T) {
	mockFolio, tc := newMockTenant(t, "a")
	valid := types.TokenPair{
		AccessToken:      "access",
		AccessExpiresAt:  time.Now().Add(time.Hour),
		RefreshToken:     "h.p.s",
		RefreshExpiresAt: time.Now().Add(time.Hour),
		OkapiURL:         tc.OkapiURL,
		OkapiTenant:      tc.OkapiTenant,
	}

	tests := map[string]func(p *types.TokenPair){
		"blank refresh":      func(p *types.TokenPair) { p.RefreshToken = "" },
		"whitespace refresh": func(p *types.TokenPair) { p.RefreshToken = "  " },
		"expired refresh":    func(p *types.TokenPair) { p.RefreshExpiresAt = time.Now().Add(-time.Second) },
		"no issuer":          func(p *types.TokenPair) { p.OkapiURL = "" },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			pair := valid
			mutate(&pair)
			LogoutTokens(pair, zap.NewNop(), "sess", "testuser", LogoutReasonClientClosed)
			assert.Empty(t, mockFolio.GetLogoutTenants())
			assert.Equal(t, 0, mockFolio.GetRefreshCount())
		})
	}
}

// TestLogoutTokens_GuardRefusal_Warns: a corrupted refresh token never reaches the network.
func TestLogoutTokens_GuardRefusal_Warns(t *testing.T) {
	mockFolio, tc := newMockTenant(t, "a")
	pair := types.TokenPair{
		AccessToken: "access", AccessExpiresAt: time.Now().Add(time.Hour),
		RefreshToken: "null", RefreshExpiresAt: time.Now().Add(time.Hour),
		OkapiURL: tc.OkapiURL,
	}

	core, recorded := observer.New(zap.DebugLevel)
	LogoutTokens(pair, zap.New(core), "sess", "testuser", LogoutReasonClientClosed)

	assert.Empty(t, mockFolio.GetLogoutTenants())
	assert.Len(t, recorded.FilterMessage("Keycloak logout refused by guard").All(), 1)
}

func TestLogoutTokens_ServerError_Warns(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()
	pair := types.TokenPair{
		AccessToken: "access", AccessExpiresAt: time.Now().Add(time.Hour),
		RefreshToken: "h.p.s", RefreshExpiresAt: time.Now().Add(time.Hour),
		OkapiURL: server.URL,
	}

	core, recorded := observer.New(zap.DebugLevel)
	LogoutTokens(pair, zap.New(core), "sess", "testuser", LogoutReasonClientClosed)

	failed := recorded.FilterMessage("Keycloak logout failed").All()
	require.Len(t, failed, 1)
	assert.Equal(t, "logout", failed[0].ContextMap()["stage"])
	assert.EqualValues(t, http.StatusInternalServerError, failed[0].ContextMap()["status_code"])
}

func TestLogoutTokens_HungServer_BoundedByTimeout(t *testing.T) {
	mockFolio, tc := newMockTenant(t, "a")
	sess := loginSession(t, zap.NewNop(), tc)
	mockFolio.SetLogoutDelay(10 * time.Second)

	start := time.Now()
	LogoutSession(sess, zap.NewNop(), LogoutReasonClientClosed)
	elapsed := time.Since(start)

	assert.Less(t, elapsed, logoutTimeout+time.Second)
	assert.True(t, sess.TakeTokens().IsEmpty(), "tokens are cleared even when the logout fails")
}

// TestLogoutSession_UsesIssuingTenant (R-4): a 93 can move the session to another tenant;
// the old pair must be sent to the tenant that issued it.
func TestLogoutSession_UsesIssuingTenant(t *testing.T) {
	mockA, tcA := newMockTenant(t, "a")
	mockB, tcB := newMockTenant(t, "b")
	sess := loginSession(t, zap.NewNop(), tcA)
	sess.UpdateTenant(tcB)

	LogoutSession(sess, zap.NewNop(), LogoutReasonClientClosed)

	assert.Equal(t, 1, mockA.GetLogoutCount())
	assert.Empty(t, mockB.GetLogoutTenants())
}

func TestLogoutTokens_NeverLogsTokens(t *testing.T) {
	mockFolio, tc := newMockTenant(t, "a")
	core, recorded := observer.New(zap.DebugLevel)
	logger := zap.New(core)
	folio.SetAuthLogger(logger)
	folio.SetClientLogger(logger)
	defer folio.SetAuthLogger(nil)
	defer folio.SetClientLogger(nil)

	sess := loginSession(t, logger, tc)
	access := sess.GetAuthToken()
	refresh, _ := sess.GetRefreshToken()
	setAccessExpiry(sess, time.Now().Add(-time.Hour)) // exercise refresh-first too
	mockFolio.ExpireAccessToken(access, time.Now().Add(-time.Hour))
	LogoutSession(sess, logger, LogoutReasonClientClosed)
	require.Equal(t, 1, mockFolio.GetLogoutCount())

	for _, entry := range recorded.All() {
		for _, token := range []string{access, refresh} {
			assert.NotContains(t, entry.Message, token)
			for k, v := range entry.ContextMap() {
				if s, ok := v.(string); ok {
					assert.NotContains(t, s, token, "field %s", k)
				}
			}
		}
	}
}

// --- 93 Login (4.2) ---------------------------------------------------------------------

func TestLoginRelogin_LogsOutReplacedPair(t *testing.T) {
	mockFolio, tc := newMockTenant(t, "a")
	sess := loginSession(t, zap.NewNop(), tc)
	oldRefresh, _ := sess.GetRefreshToken()
	setAccessExpiry(sess, time.Now().Add(-time.Minute)) // F-8: a 93 after expiry logs in again

	h := NewLoginHandler(zap.NewNop(), tc)
	var rec logoutRecorder
	rec.install(h.BaseHandler)
	resp, err := h.Handle(context.Background(), loginMsg("testuser", "testpass"), sess)
	require.NoError(t, err)
	require.Contains(t, resp, "941")

	calls := rec.all()
	require.Len(t, calls, 1)
	assert.Equal(t, LogoutReasonRelogin93, calls[0].reason)
	assert.Equal(t, oldRefresh, calls[0].pair.RefreshToken)
	assert.False(t, mockFolio.IsRefreshTokenAlive(oldRefresh), "old Keycloak session should have ended")
	newRefresh, _ := sess.GetRefreshToken()
	assert.True(t, mockFolio.IsRefreshTokenAlive(newRefresh), "new session must stay alive")
	assert.Equal(t, 0, mockFolio.GetTrapTriggeredCount())
}

// TestLoginRelogin_DifferentCredentials_LogsOutReplacedPair (Q6): a valid token with
// different stored credentials still logs in, so the old pair is ended.
func TestLoginRelogin_DifferentCredentials_LogsOutReplacedPair(t *testing.T) {
	mockFolio, tc := newMockTenant(t, "a")
	sess := loginSession(t, zap.NewNop(), tc)
	oldRefresh, _ := sess.GetRefreshToken()
	sess.SetAuthCredentials("previous-password")

	h := NewLoginHandler(zap.NewNop(), tc)
	var rec logoutRecorder
	rec.install(h.BaseHandler)
	resp, err := h.Handle(context.Background(), loginMsg("testuser", "testpass"), sess)
	require.NoError(t, err)
	require.Contains(t, resp, "941")

	require.Len(t, rec.all(), 1)
	assert.False(t, mockFolio.IsRefreshTokenAlive(oldRefresh))
	assert.Equal(t, 2, mockFolio.GetLoginCount())
}

func TestLoginRelogin_DefaultRunsInBackground(t *testing.T) {
	mockFolio, tc := newMockTenant(t, "a")
	sess := loginSession(t, zap.NewNop(), tc)
	oldRefresh, _ := sess.GetRefreshToken()
	setAccessExpiry(sess, time.Now().Add(-time.Minute))
	mockFolio.SetLogoutDelay(2 * time.Second)

	h := NewLoginHandler(zap.NewNop(), tc)
	start := time.Now()
	resp, err := h.Handle(context.Background(), loginMsg("testuser", "testpass"), sess)
	require.NoError(t, err)
	require.Contains(t, resp, "941")
	assert.Less(t, time.Since(start), time.Second, "a slow logout must not delay the 94")

	assert.Eventually(t, func() bool { return mockFolio.GetLogoutCount() == 1 }, 5*time.Second, 20*time.Millisecond)
	assert.False(t, mockFolio.IsRefreshTokenAlive(oldRefresh))
}

func TestLoginFailure_SameTenant_SessionUntouched(t *testing.T) {
	mockFolio, tc := newMockTenant(t, "a")
	sess := loginSession(t, zap.NewNop(), tc)
	oldAccess := sess.GetAuthToken()
	oldRefresh, _ := sess.GetRefreshToken()

	h := NewLoginHandler(zap.NewNop(), tc)
	var rec logoutRecorder
	rec.install(h.BaseHandler)
	resp, err := h.Handle(context.Background(), loginMsg("testuser", "wrong"), sess)
	require.NoError(t, err)
	require.Contains(t, resp, "940")

	assert.Empty(t, rec.all())
	assert.True(t, sess.IsAuth())
	assert.Equal(t, oldAccess, sess.GetAuthToken())
	refresh, _ := sess.GetRefreshToken()
	assert.Equal(t, oldRefresh, refresh)
	assert.True(t, mockFolio.IsRefreshTokenAlive(oldRefresh))
}

// TestLoginFailure_AfterTenantSwitch_DiscardsOldTenantSession (R-4b).
func TestLoginFailure_AfterTenantSwitch_DiscardsOldTenantSession(t *testing.T) {
	mockA, tcA := newMockTenant(t, "a")
	mockB, tcB := newMockTenant(t, "b")
	sess := loginSession(t, zap.NewNop(), tcA)
	oldRefresh, _ := sess.GetRefreshToken()
	sess.UpdateTenant(tcB) // what handleLoginTenantResolution does before the 93 handler

	h := NewLoginHandler(zap.NewNop(), tcB)
	var rec logoutRecorder
	rec.install(h.BaseHandler)
	resp, err := h.Handle(context.Background(), loginMsg("testuser", "wrong"), sess)
	require.NoError(t, err)
	require.Contains(t, resp, "940")

	calls := rec.all()
	require.Len(t, calls, 1)
	assert.Equal(t, LogoutReasonTenantChanged, calls[0].reason)
	assert.Equal(t, "testuser", calls[0].username)
	assert.Equal(t, mockA.GetURL(), calls[0].pair.OkapiURL)
	assert.Equal(t, 1, mockA.GetLogoutCount())
	assert.Empty(t, mockB.GetLogoutTenants())
	assert.False(t, mockA.IsRefreshTokenAlive(oldRefresh))

	assert.False(t, sess.IsAuth(), "session must not stay authenticated with another tenant's tokens")
	assert.Empty(t, sess.GetAuthToken())
	assert.False(t, sess.HasAuthCredentials())
}

func TestLoginSuccess_AfterTenantSwitch_LogsOutAgainstOldTenant(t *testing.T) {
	mockA, tcA := newMockTenant(t, "a")
	mockB, tcB := newMockTenant(t, "b")
	sess := loginSession(t, zap.NewNop(), tcA)
	oldRefresh, _ := sess.GetRefreshToken()
	sess.UpdateTenant(tcB)

	h := NewLoginHandler(zap.NewNop(), tcB)
	var rec logoutRecorder
	rec.install(h.BaseHandler)
	// Same credentials and a valid token: without R-4 this would short-circuit and keep A's tokens.
	resp, err := h.Handle(context.Background(), loginMsg("testuser", "testpass"), sess)
	require.NoError(t, err)
	require.Contains(t, resp, "941")

	assert.Equal(t, 1, mockB.GetLoginCount(), "a token from another tenant must not short-circuit the 93")
	assert.Equal(t, 1, mockA.GetLogoutCount())
	assert.Empty(t, mockB.GetLogoutTenants())
	assert.False(t, mockA.IsRefreshTokenAlive(oldRefresh))
	assert.False(t, sess.HasTokensFromOtherTenant())
	newRefresh, _ := sess.GetRefreshToken()
	assert.True(t, mockB.IsRefreshTokenAlive(newRefresh))
}

func TestLoginFirst_NoLogout(t *testing.T) {
	mockFolio, tc := newMockTenant(t, "a")
	sess := testutil.NewSession(tc)

	h := NewLoginHandler(zap.NewNop(), tc)
	var rec logoutRecorder
	rec.install(h.BaseHandler)
	_, err := h.Handle(context.Background(), loginMsg("testuser", "testpass"), sess)
	require.NoError(t, err)

	assert.Empty(t, rec.all())
	assert.Empty(t, mockFolio.GetLogoutTenants())
}

// --- Renewal fallback (4.3) --------------------------------------------------------------

// expireForRenewal puts the access token inside the renewal buffer but not past expiry,
// which is when renewal normally runs.
func expireForRenewal(sess *types.Session) {
	setAccessExpiry(sess, time.Now().Add(types.TokenExpiryBuffer/2))
}

func TestRenewalFallback_TransientRefreshFailure_LogsOutOldPair(t *testing.T) {
	mockFolio, tc := newMockTenant(t, "a")
	sess := loginSession(t, zap.NewNop(), tc)
	oldRefresh, _ := sess.GetRefreshToken()
	expireForRenewal(sess)
	mockFolio.SetRefreshStatus(http.StatusServiceUnavailable)

	h := NewBaseHandler(zap.NewNop(), tc)
	var rec logoutRecorder
	rec.install(h)
	_, err := h.renewToken(context.Background(), sess)
	require.NoError(t, err)

	calls := rec.all()
	require.Len(t, calls, 1)
	assert.Equal(t, LogoutReasonRenewalFallback, calls[0].reason)
	assert.Equal(t, oldRefresh, calls[0].pair.RefreshToken)
	assert.Equal(t, 1, mockFolio.GetLogoutCount())
	assert.False(t, mockFolio.IsRefreshTokenAlive(oldRefresh))
	newRefresh, _ := sess.GetRefreshToken()
	assert.True(t, mockFolio.IsRefreshTokenAlive(newRefresh))
}

func TestRenewalFallback_NoLogoutCases(t *testing.T) {
	tests := map[string]func(m *mocks.FolioMockServer, sess *types.Session){
		"refresh 401": func(m *mocks.FolioMockServer, sess *types.Session) { m.SetRejectRefresh(true) },
		"refresh 422": func(m *mocks.FolioMockServer, sess *types.Session) {
			m.SetRefreshStatus(http.StatusUnprocessableEntity)
		},
		"refresh token expired": func(m *mocks.FolioMockServer, sess *types.Session) {
			sess.UpdateTokens(sess.GetAuthToken(), time.Now().Add(-time.Minute), "h.p.s", time.Now().Add(-time.Minute))
		},
	}
	for name, setup := range tests {
		t.Run(name, func(t *testing.T) {
			mockFolio, tc := newMockTenant(t, "a")
			sess := loginSession(t, zap.NewNop(), tc)
			expireForRenewal(sess)
			setup(mockFolio, sess)

			h := NewBaseHandler(zap.NewNop(), tc)
			var rec logoutRecorder
			rec.install(h)
			_, err := h.renewToken(context.Background(), sess)
			require.NoError(t, err)

			assert.Empty(t, rec.all())
			assert.Empty(t, mockFolio.GetLogoutTenants())
		})
	}
}

func TestRenewalFallback_ReloginFails_KeepsOldPair(t *testing.T) {
	mockFolio, tc := newMockTenant(t, "a")
	sess := loginSession(t, zap.NewNop(), tc)
	oldRefresh, _ := sess.GetRefreshToken()
	expireForRenewal(sess)
	mockFolio.SetRefreshStatus(http.StatusServiceUnavailable)
	mockFolio.SetRejectLogins(true)

	h := NewBaseHandler(zap.NewNop(), tc)
	var rec logoutRecorder
	rec.install(h)
	_, err := h.renewToken(context.Background(), sess)
	require.Error(t, err)

	assert.Empty(t, rec.all())
	refresh, _ := sess.GetRefreshToken()
	assert.Equal(t, oldRefresh, refresh, "a failed re-login keeps the stored pair")
}
