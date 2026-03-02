package handlers

import (
	"context"
	"testing"

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

func TestLoginHandle_Success_AccessTokenFallback(t *testing.T) {
	// The FolioMockServer returns AccessToken (not OkapiToken) in its JSON response.
	// This means every successful login via the mock already exercises the
	// AccessToken fallback branch in login.go (token := authResp.OkapiToken → ""
	// → token = authResp.AccessToken). The test below drives this path explicitly.
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
