package handlers

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/spokanepubliclibrary/fsip2/internal/sip2/parser"
	"github.com/spokanepubliclibrary/fsip2/tests/mocks"
	"github.com/spokanepubliclibrary/fsip2/tests/testutil"
)

func TestLoginHandle_AuthFailure(t *testing.T) {
	mockFolio := mocks.NewFolioMockServer()
	defer mockFolio.Close()
	mockFolio.SetRejectLogins(true)

	tc := testutil.NewTenantConfig(testutil.WithOkapiURL(mockFolio.GetURL()))
	sess := testutil.NewSession(tc)

	h := NewLoginHandler(zap.NewNop(), tc)
	msg := buildTestMsg(parser.LoginRequest, map[parser.FieldCode]string{
		parser.LoginUserID:   "testuser",
		parser.LoginPassword: "testpass",
	})

	resp, err := h.Handle(context.Background(), msg, sess)
	require.NoError(t, err)
	assert.Contains(t, resp, "940") // login ok=0
}

func TestLoginHandle_Success_StoresTokenPair(t *testing.T) {
	// The mock serves /authn/login-with-expiry: tokens as cookies, expirations in the body.
	// Both tokens must end up in the session so renewal can use /authn/refresh.
	mockFolio := mocks.NewFolioMockServer()
	defer mockFolio.Close()

	tc := testutil.NewTenantConfig(testutil.WithOkapiURL(mockFolio.GetURL()))
	sess := testutil.NewSession(tc)

	h := NewLoginHandler(zap.NewNop(), tc)
	msg := buildTestMsg(parser.LoginRequest, map[parser.FieldCode]string{
		parser.LoginUserID:   "testuser",
		parser.LoginPassword: "testpass",
	})

	resp, err := h.Handle(context.Background(), msg, sess)
	require.NoError(t, err)
	assert.Contains(t, resp, "941") // login ok=1
	assert.True(t, sess.IsAuth(), "session should be authenticated after login")
	assert.NotEmpty(t, sess.GetAuthToken())
	refreshToken, refreshExp := sess.GetRefreshToken()
	assert.NotEmpty(t, refreshToken, "refresh token should be stored")
	assert.True(t, refreshExp.After(time.Now()), "refresh expiry should come from the server")
	assert.True(t, sess.GetTokenExpiresAt().After(time.Now()), "access expiry should come from the server")
	assert.Zero(t, mockFolio.GetDeprecatedLoginCount(), "the deprecated /authn/login must never be called")
}

func TestLoginHandle_LocationCodeStoredInSession(t *testing.T) {
	mockFolio := mocks.NewFolioMockServer()
	defer mockFolio.Close()

	tc := testutil.NewTenantConfig(testutil.WithOkapiURL(mockFolio.GetURL()))
	sess := testutil.NewSession(tc)

	h := NewLoginHandler(zap.NewNop(), tc)
	msg := buildTestMsg(parser.LoginRequest, map[parser.FieldCode]string{
		parser.LoginUserID:   "testuser",
		parser.LoginPassword: "testpass",
		parser.LocationCode:  "BRANCH-A",
	})

	resp, err := h.Handle(context.Background(), msg, sess)
	require.NoError(t, err)
	assert.Contains(t, resp, "941") // success
	assert.Equal(t, "BRANCH-A", sess.GetLocationCode())
}

// loginMsg builds a 93 Login request with the given credentials.
func loginMsg(username, password string) *parser.Message {
	return buildTestMsg(parser.LoginRequest, map[parser.FieldCode]string{
		parser.LoginUserID:   username,
		parser.LoginPassword: password,
	})
}

func TestLoginHandle_ShortCircuit_SameCredentials_NoFOLIOCall(t *testing.T) {
	mockFolio := mocks.NewFolioMockServer()
	defer mockFolio.Close()
	tc := testutil.NewTenantConfig(testutil.WithOkapiURL(mockFolio.GetURL()))
	sess := testutil.NewSession(tc)
	h := NewLoginHandler(zap.NewNop(), tc)

	resp, err := h.Handle(context.Background(), loginMsg("testuser", "testpass"), sess)
	require.NoError(t, err)
	require.Contains(t, resp, "941")
	token := sess.GetAuthToken()

	resp, err = h.Handle(context.Background(), loginMsg("testuser", "testpass"), sess)
	require.NoError(t, err)
	assert.Contains(t, resp, "941")
	assert.Equal(t, 1, mockFolio.GetLoginCount(), "same credentials with a valid token must not log in again")
	assert.Equal(t, token, sess.GetAuthToken())
}

func TestLoginHandle_ShortCircuit_WrongPassword_Fails(t *testing.T) {
	mockFolio := mocks.NewFolioMockServer()
	defer mockFolio.Close()
	tc := testutil.NewTenantConfig(testutil.WithOkapiURL(mockFolio.GetURL()))
	sess := testutil.NewSession(tc)
	h := NewLoginHandler(zap.NewNop(), tc)

	_, err := h.Handle(context.Background(), loginMsg("testuser", "testpass"), sess)
	require.NoError(t, err)

	resp, err := h.Handle(context.Background(), loginMsg("testuser", "wrong"), sess)
	require.NoError(t, err)
	assert.Contains(t, resp, "940", "a wrong password must not be accepted via the short-circuit")
	assert.Equal(t, 1, mockFolio.GetLoginCount(), "the failed attempt is not a successful login")

	// A failed 93 leaves the existing session untouched (01b 4.2)
	username, password := sess.GetAuthCredentials()
	assert.Equal(t, "testuser", username)
	assert.Equal(t, "testpass", password)
	assert.True(t, sess.IsAuth())
}

func TestLoginHandle_ShortCircuit_DifferentUser_LogsIn(t *testing.T) {
	mockFolio := mocks.NewFolioMockServer()
	defer mockFolio.Close()
	tc := testutil.NewTenantConfig(testutil.WithOkapiURL(mockFolio.GetURL()))
	sess := testutil.NewSession(tc)
	h := NewLoginHandler(zap.NewNop(), tc)

	// Connection currently authenticated as another kiosk user with a valid token
	sess.SetAuthenticated("otheruser", "", "", "other-token", time.Now().Add(10*time.Minute))
	sess.SetAuthCredentials("otherpass")

	resp, err := h.Handle(context.Background(), loginMsg("testuser", "testpass"), sess)
	require.NoError(t, err)
	assert.Contains(t, resp, "941")
	assert.Equal(t, 1, mockFolio.GetLoginCount(), "a different user must authenticate with FOLIO")
	assert.Equal(t, "testuser", sess.GetUsername())
	assert.NotEqual(t, "other-token", sess.GetAuthToken())
	_, password := sess.GetAuthCredentials()
	assert.Equal(t, "testpass", password)
}
