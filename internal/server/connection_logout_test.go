package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/spokanepubliclibrary/fsip2/internal/config"
	"github.com/spokanepubliclibrary/fsip2/internal/folio"
	"github.com/spokanepubliclibrary/fsip2/internal/handlers"
	"github.com/spokanepubliclibrary/fsip2/internal/tenant"
	"github.com/spokanepubliclibrary/fsip2/internal/types"
	"github.com/spokanepubliclibrary/fsip2/tests/mocks"
)

// timeoutConn behaves like an idle connection whose read deadline has expired.
type timeoutConn struct{ *mockConn }

func (c *timeoutConn) Read(b []byte) (int, error) { return 0, os.ErrDeadlineExceeded }

type recordedClose struct {
	reason       string
	socketClosed bool
}

// withLogoutRecorder replaces the server's logout with a recorder.
func withLogoutRecorder(conn *Connection, mc *mockConn) *[]recordedClose {
	var calls []recordedClose
	conn.server.sessionLogout = func(session *types.Session, reason string) {
		calls = append(calls, recordedClose{reason: reason, socketClosed: mc.closed})
	}
	return &calls
}

func TestCloseReason(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"clean EOF", nil, handlers.LogoutReasonClientClosed},
		{"read deadline", fmt.Errorf("failed to read message: %w", os.ErrDeadlineExceeded), handlers.LogoutReasonReadTimeout},
		{"context cancelled", context.Canceled, handlers.LogoutReasonContextCancelled},
		{"wrapped context deadline", fmt.Errorf("x: %w", context.DeadlineExceeded), handlers.LogoutReasonContextCancelled},
		{"connection reset", fmt.Errorf("failed to read message: %w", errors.New("connection reset by peer")), handlers.LogoutReasonError},
		{"unexpected EOF", fmt.Errorf("failed to read message: %w", io.ErrUnexpectedEOF), handlers.LogoutReasonError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, closeReason(tt.err))
		})
	}
}

func TestConnectionClose_ClientClosed_LogsOutOnceAfterSocketClose(t *testing.T) {
	conn, mc := newHandleTestConn(t, "", nil)
	calls := withLogoutRecorder(conn, mc)

	require.NoError(t, conn.Handle(context.Background()))

	require.Len(t, *calls, 1)
	assert.Equal(t, handlers.LogoutReasonClientClosed, (*calls)[0].reason)
	assert.True(t, (*calls)[0].socketClosed, "the socket must be closed before the logout runs")
}

func TestConnectionClose_ReadTimeout(t *testing.T) {
	conn, mc := newHandleTestConn(t, "", nil)
	conn.conn = &timeoutConn{mc}
	calls := withLogoutRecorder(conn, mc)

	require.Error(t, conn.Handle(context.Background()))

	require.Len(t, *calls, 1)
	assert.Equal(t, handlers.LogoutReasonReadTimeout, (*calls)[0].reason)
}

func TestConnectionClose_ContextCancelled(t *testing.T) {
	conn, mc := newHandleTestConn(t, "", nil)
	calls := withLogoutRecorder(conn, mc)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	require.Error(t, conn.Handle(ctx))

	require.Len(t, *calls, 1)
	assert.Equal(t, handlers.LogoutReasonContextCancelled, (*calls)[0].reason)
}

func TestConnectionClose_NilLogout_NoPanic(t *testing.T) {
	conn, _ := newHandleTestConn(t, "", nil)
	conn.server.sessionLogout = nil
	assert.NoError(t, conn.Handle(context.Background()))
}

// TestConnectionClose_EndsKeycloakSession wires the real logout (NewServer default) to a mock FOLIO.
func TestConnectionClose_EndsKeycloakSession(t *testing.T) {
	mockFolio := mocks.NewFolioMockServer()
	defer mockFolio.Close()

	tc := &config.TenantConfig{Tenant: "test", OkapiURL: mockFolio.GetURL(), MessageDelimiter: "\r", FieldDelimiter: "|"}
	sess := types.NewSession("test-session", tc)
	resp, err := folio.NewAuthClient(tc.OkapiURL, tc.OkapiTenant).Login(context.Background(), "testuser", "testpass")
	require.NoError(t, err)
	sess.SetAuthenticated("testuser", "", "", resp.OkapiToken, resp.ExpiresAt)
	sess.UpdateTokens(resp.OkapiToken, resp.ExpiresAt, resp.RefreshToken, resp.RefreshTokenExpiration)

	cfg := &config.Config{Tenants: map[string]*config.TenantConfig{tc.Tenant: tc}}
	srv, _ := NewServer(cfg, zap.NewNop())
	conn := NewConnection(newMockConn(""), sess, tenant.NewService(cfg), nil, srv, 6443, "127.0.0.1")

	require.NoError(t, conn.Handle(context.Background()))

	assert.Equal(t, 1, mockFolio.GetLogoutCount())
	assert.Equal(t, 0, mockFolio.GetTrapTriggeredCount())
	assert.False(t, mockFolio.IsRefreshTokenAlive(resp.RefreshToken))
	assert.True(t, sess.TakeTokens().IsEmpty())
}

// TestConnectionClose_NoTokens_NoRequest: a connection that never logged in sends nothing.
func TestConnectionClose_NoTokens_NoRequest(t *testing.T) {
	mockFolio := mocks.NewFolioMockServer()
	defer mockFolio.Close()

	tc := &config.TenantConfig{Tenant: "test", OkapiURL: mockFolio.GetURL(), MessageDelimiter: "\r", FieldDelimiter: "|"}
	cfg := &config.Config{Tenants: map[string]*config.TenantConfig{tc.Tenant: tc}}
	srv, _ := NewServer(cfg, zap.NewNop())
	conn := NewConnection(newMockConn(""), types.NewSession("s", tc), tenant.NewService(cfg), nil, srv, 6443, "127.0.0.1")

	require.NoError(t, conn.Handle(context.Background()))
	assert.Empty(t, mockFolio.GetLogoutTenants())
}
