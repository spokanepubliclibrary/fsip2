package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/spokanepubliclibrary/fsip2/internal/config"
	"github.com/spokanepubliclibrary/fsip2/internal/folio"
	"github.com/spokanepubliclibrary/fsip2/internal/sip2/parser"
	"github.com/spokanepubliclibrary/fsip2/internal/types"
	"github.com/spokanepubliclibrary/fsip2/tests/mocks"
	"github.com/spokanepubliclibrary/fsip2/tests/testutil"
)

// loginSession performs a real 93 login against the mock and returns the session.
func loginSession(t *testing.T, logger *zap.Logger, tc *config.TenantConfig) *types.Session {
	t.Helper()
	sess := testutil.NewSession(tc)
	h := NewLoginHandler(logger, tc)
	msg := buildTestMsg(parser.LoginRequest, map[parser.FieldCode]string{
		parser.LoginUserID:   "testuser",
		parser.LoginPassword: "testpass",
	})
	resp, err := h.Handle(context.Background(), msg, sess)
	require.NoError(t, err)
	require.Contains(t, resp, "941")
	return sess
}

// expireAccessToken marks the session's access token as expired, keeping the refresh token.
func expireAccessToken(sess *types.Session) {
	refresh, refreshExp := sess.GetRefreshToken()
	sess.UpdateTokens(sess.GetAuthToken(), time.Now().Add(-time.Minute), refresh, refreshExp)
}

func TestRenewToken_RefreshReplacesBothTokens(t *testing.T) {
	mockFolio := mocks.NewFolioMockServer()
	defer mockFolio.Close()
	tc := testutil.NewTenantConfig(testutil.WithOkapiURL(mockFolio.GetURL()))
	sess := loginSession(t, zap.NewNop(), tc)

	oldAccess := sess.GetAuthToken()
	oldRefresh, _ := sess.GetRefreshToken()
	expireAccessToken(sess)

	h := NewBaseHandler(zap.NewNop(), tc)
	token, err := h.renewToken(context.Background(), sess)
	require.NoError(t, err)

	newRefresh, _ := sess.GetRefreshToken()
	assert.NotEqual(t, oldAccess, token)
	assert.Equal(t, token, sess.GetAuthToken())
	assert.NotEqual(t, oldRefresh, newRefresh, "refresh token must be rotated")
	assert.False(t, sess.IsTokenExpired())
	assert.Equal(t, 1, mockFolio.GetRefreshCount())
	assert.Equal(t, 1, mockFolio.GetLoginCount(), "refresh must not re-login")
}

func TestRenewToken_RefreshRejected_FallsBackToRelogin(t *testing.T) {
	mockFolio := mocks.NewFolioMockServer()
	defer mockFolio.Close()
	tc := testutil.NewTenantConfig(testutil.WithOkapiURL(mockFolio.GetURL()))
	sess := loginSession(t, zap.NewNop(), tc)
	expireAccessToken(sess)
	mockFolio.SetRejectRefresh(true)

	h := NewBaseHandler(zap.NewNop(), tc)
	_, err := h.renewToken(context.Background(), sess)
	require.NoError(t, err)

	assert.Equal(t, 0, mockFolio.GetRefreshCount())
	assert.Equal(t, 2, mockFolio.GetLoginCount(), "rejected refresh falls back to re-login")
	assert.False(t, sess.IsTokenExpired())
}

func TestRenewToken_ExpiredRefreshToken_ReloginWithoutRefreshCall(t *testing.T) {
	mockFolio := mocks.NewFolioMockServer()
	defer mockFolio.Close()
	tc := testutil.NewTenantConfig(testutil.WithOkapiURL(mockFolio.GetURL()))
	sess := loginSession(t, zap.NewNop(), tc)

	// Refresh token inside the expiry buffer: must not be used
	refresh, _ := sess.GetRefreshToken()
	sess.UpdateTokens(sess.GetAuthToken(), time.Now().Add(-time.Minute), refresh, time.Now().Add(30*time.Second))

	h := NewBaseHandler(zap.NewNop(), tc)
	_, err := h.renewToken(context.Background(), sess)
	require.NoError(t, err)

	assert.Equal(t, 0, mockFolio.GetRefreshCount(), "a refresh token inside the buffer must not be used")
	assert.Equal(t, 2, mockFolio.GetLoginCount())
}

func TestRenewToken_RefreshMissingCookie_FallsBackToRelogin(t *testing.T) {
	var refreshCalls, loginCalls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := `{"accessTokenExpiration":"` + time.Now().Add(10*time.Minute).UTC().Format(time.RFC3339) +
			`","refreshTokenExpiration":"` + time.Now().Add(24*time.Hour).UTC().Format(time.RFC3339) + `"}`
		switch r.URL.Path {
		case "/authn/refresh":
			atomic.AddInt32(&refreshCalls, 1)
			// Gateway that drops the refresh cookie
			http.SetCookie(w, &http.Cookie{Name: folio.AccessTokenCookie, Value: "refreshed-access"})
		case "/authn/login-with-expiry":
			atomic.AddInt32(&loginCalls, 1)
			http.SetCookie(w, &http.Cookie{Name: folio.AccessTokenCookie, Value: "relogin-access"})
			http.SetCookie(w, &http.Cookie{Name: folio.RefreshTokenCookie, Value: "relogin-refresh"})
		}
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(body))
	}))
	defer srv.Close()

	tc := testutil.NewTenantConfig(testutil.WithOkapiURL(srv.URL))
	sess := testutil.NewSession(tc)
	sess.SetAuthenticated("testuser", "", "", "old-access", time.Now().Add(-time.Minute))
	sess.SetAuthCredentials("testpass")
	sess.UpdateTokens("old-access", time.Now().Add(-time.Minute), "old-refresh", time.Now().Add(time.Hour))

	h := NewBaseHandler(zap.NewNop(), tc)
	token, err := h.renewToken(context.Background(), sess)
	require.NoError(t, err)

	assert.Equal(t, "relogin-access", token, "a refresh without both cookies must not be used")
	refresh, _ := sess.GetRefreshToken()
	assert.Equal(t, "relogin-refresh", refresh)
	assert.Equal(t, int32(1), atomic.LoadInt32(&refreshCalls))
	assert.Equal(t, int32(1), atomic.LoadInt32(&loginCalls))
}

func TestRenewToken_AlreadyRenewed_NoCall(t *testing.T) {
	mockFolio := mocks.NewFolioMockServer()
	defer mockFolio.Close()
	tc := testutil.NewTenantConfig(testutil.WithOkapiURL(mockFolio.GetURL()))
	sess := loginSession(t, zap.NewNop(), tc)

	// Token still valid (another caller renewed while this one waited for the lock)
	h := NewBaseHandler(zap.NewNop(), tc)
	token, err := h.renewToken(context.Background(), sess)
	require.NoError(t, err)
	assert.Equal(t, sess.GetAuthToken(), token)
	assert.Equal(t, 0, mockFolio.GetRefreshCount())
	assert.Equal(t, 1, mockFolio.GetLoginCount())
}

func TestRenewToken_ConcurrentCallers_SingleRenewal(t *testing.T) {
	mockFolio := mocks.NewFolioMockServer()
	defer mockFolio.Close()
	tc := testutil.NewTenantConfig(testutil.WithOkapiURL(mockFolio.GetURL()))
	sess := loginSession(t, zap.NewNop(), tc)
	expireAccessToken(sess)

	h := NewBaseHandler(zap.NewNop(), tc)
	const callers = 8
	tokens := make([]string, callers)
	var wg sync.WaitGroup
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, token, err := h.getAuthenticatedFolioClient(context.Background(), sess)
			assert.NoError(t, err)
			tokens[i] = token
		}(i)
	}
	wg.Wait()

	assert.Equal(t, 1, mockFolio.GetRefreshCount(), "concurrent callers must renew exactly once")
	assert.Equal(t, 1, mockFolio.GetLoginCount())
	for _, token := range tokens {
		assert.Equal(t, sess.GetAuthToken(), token)
	}
}

func TestRenewToken_LogsOutcome(t *testing.T) {
	mockFolio := mocks.NewFolioMockServer()
	defer mockFolio.Close()
	tc := testutil.NewTenantConfig(testutil.WithOkapiURL(mockFolio.GetURL()))
	sess := loginSession(t, zap.NewNop(), tc)
	expireAccessToken(sess)
	mockFolio.SetRejectRefresh(true)

	core, recorded := observer.New(zap.DebugLevel)
	h := NewBaseHandler(zap.New(core), tc)
	_, err := h.renewToken(context.Background(), sess)
	require.NoError(t, err)

	renewals := recorded.FilterMessage("Token renewal").All()
	require.Len(t, renewals, 2)

	refreshLog := renewals[0].ContextMap()
	assert.Equal(t, zap.WarnLevel, renewals[0].Level)
	assert.Equal(t, "refresh", refreshLog["method"])
	assert.Equal(t, "failed", refreshLog["result"])
	assert.EqualValues(t, http.StatusUnauthorized, refreshLog["status_code"])

	reloginLog := renewals[1].ContextMap()
	assert.Equal(t, zap.InfoLevel, renewals[1].Level)
	assert.Equal(t, "relogin", reloginLog["method"])
	assert.Equal(t, "ok", reloginLog["result"])
	assert.Equal(t, sess.ID, reloginLog["session_id"])
	assert.Contains(t, reloginLog, "access_expires_at")
}

// TestTokenValues_NeverLogged drives login, refresh and the re-login fallback with every
// logger at debug level and asserts no logged message or field contains a token value.
func TestTokenValues_NeverLogged(t *testing.T) {
	mockFolio := mocks.NewFolioMockServer()
	defer mockFolio.Close()
	tc := testutil.NewTenantConfig(testutil.WithOkapiURL(mockFolio.GetURL()))

	core, recorded := observer.New(zap.DebugLevel)
	logger := zap.New(core)
	folio.SetAuthLogger(logger)
	folio.SetClientLogger(logger)
	defer folio.SetAuthLogger(nil)
	defer folio.SetClientLogger(nil)

	var seenTokens []string
	remember := func(sess *types.Session) {
		refresh, _ := sess.GetRefreshToken()
		seenTokens = append(seenTokens, sess.GetAuthToken(), refresh)
	}

	sess := loginSession(t, logger, tc)
	remember(sess)

	h := NewBaseHandler(logger, tc)
	expireAccessToken(sess)
	_, err := h.renewToken(context.Background(), sess) // refresh
	require.NoError(t, err)
	remember(sess)

	expireAccessToken(sess)
	mockFolio.SetRejectRefresh(true)
	_, err = h.renewToken(context.Background(), sess) // failed refresh, then re-login
	require.NoError(t, err)
	remember(sess)

	require.NotEmpty(t, recorded.All())
	for _, entry := range recorded.All() {
		for _, token := range seenTokens {
			require.NotEmpty(t, token)
			assert.NotContains(t, entry.Message, token)
			for key, value := range entry.ContextMap() {
				if s, ok := value.(string); ok {
					assert.False(t, strings.Contains(s, token), "log %q field %q contains a token value", entry.Message, key)
				}
			}
		}
	}
}
