// +build e2e

package e2e

import (
	"context"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/spokanepubliclibrary/fsip2/internal/config"
	"github.com/spokanepubliclibrary/fsip2/internal/logging"
	"github.com/spokanepubliclibrary/fsip2/internal/server"
	"github.com/spokanepubliclibrary/fsip2/tests/mocks"
	"github.com/spokanepubliclibrary/fsip2/tests/testutil"
)

// E2ESetup encapsulates the full test environment for a SIP2 e2e test.
// Always call setup.Close(t) in a defer or t.Cleanup.
type E2ESetup struct {
	MockFolio *mocks.FolioMockServer
	Server    *server.Server
	Config    *config.Config
	Port      int
	cancel    context.CancelFunc
}

// NewE2ESetup creates a mock FOLIO server, builds config, starts the SIP2 server,
// and waits until the port is accepting connections.
func NewE2ESetup(t *testing.T) *E2ESetup {
	t.Helper()
	mockFolio := mocks.NewFolioMockServer()
	cfg := buildE2EConfig(t, mockFolio.GetURL())
	logger, _ := logging.NewLogger("warn", "")
	srv, err := server.NewServer(cfg, logger)
	require.NoError(t, err)
	srv.RegisterAllHandlers()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	go srv.Start(ctx)
	waitForServerReady(t, cfg.Port)

	return &E2ESetup{
		MockFolio: mockFolio,
		Server:    srv,
		Config:    cfg,
		Port:      cfg.Port,
		cancel:    cancel,
	}
}

// Close shuts down the server and mock FOLIO server.
func (s *E2ESetup) Close(t *testing.T) {
	t.Helper()
	s.cancel()
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer stopCancel()
	s.Server.Stop(stopCtx)
	s.MockFolio.Close()
}

// Connect opens a new TCP connection to the SIP2 server.
func (s *E2ESetup) Connect(t *testing.T) net.Conn {
	t.Helper()
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", s.Port), 2*time.Second)
	require.NoError(t, err)
	return conn
}

// Exchange sends a SIP2 message and returns the response line.
func (s *E2ESetup) Exchange(t *testing.T, conn net.Conn, message string) string {
	t.Helper()
	t.Logf("Sending: %s", message)
	_, err := conn.Write([]byte(message))
	require.NoError(t, err, "failed to send message")

	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	require.NoError(t, err, "failed to read response")
	response := string(buf[:n])
	t.Logf("Received: %s", response)
	return response
}

// Login sends SIP2 login (93) and asserts it succeeds (941 response).
// CP is set to testServicePointUUID so checkin can resolve the service point.
func (s *E2ESetup) Login(t *testing.T, conn net.Conn) {
	t.Helper()
	resp := s.Exchange(t, conn, testutil.NewLoginMessage("testuser", "testpass", testServicePointUUID))
	require.True(t, strings.HasPrefix(resp, "94"), "login response must start with 94, got: %s", resp)
	require.Contains(t, resp, "941", "login must succeed")
}

// testServicePointUUID is the CP value used for all E2E sessions. It must match
// whatever service point UUID the mock FOLIO server accepts.
const testServicePointUUID = "test-sp-uuid"

// =============================================================================
// Server lifecycle helpers
// =============================================================================

// buildE2EConfig creates a test config using dynamically allocated ports.
func buildE2EConfig(t *testing.T, folioURL string) *config.Config {
	t.Helper()
	return buildE2EConfigWithOptions(t, folioURL, false)
}

// buildE2EConfigWithOptions creates a test config with optional bulk-payment support.
func buildE2EConfigWithOptions(t *testing.T, folioURL string, acceptBulkPayment bool) *config.Config {
	t.Helper()
	port, err := getFreePort()
	require.NoError(t, err, "failed to allocate free SIP2 port")
	healthPort, err := getFreePort()
	require.NoError(t, err, "failed to allocate free health port")
	return &config.Config{
		Port:            port,
		OkapiURL:        folioURL,
		LogLevel:        "info",
		HealthCheckPort: healthPort,
		Tenants: map[string]*config.TenantConfig{
			"test-inst": {
				Tenant:             "test-tenant",
				MessageDelimiter:   "\r",
				FieldDelimiter:     "|",
				OkapiURL:           folioURL,
				Charset:            "UTF-8",
				AcceptBulkPayment:  acceptBulkPayment,
				SupportedMessages: []config.MessageSupport{
					{Code: "23", Enabled: true}, // Patron Status
					{Code: "11", Enabled: true}, // Checkout
					{Code: "09", Enabled: true}, // Checkin
					{Code: "63", Enabled: true}, // Patron Information
					{Code: "17", Enabled: true}, // Item Information
					{Code: "29", Enabled: true}, // Renew
					{Code: "65", Enabled: true}, // Renew All
					{Code: "35", Enabled: true}, // End Patron Session
					{Code: "37", Enabled: true}, // Fee Paid
					{Code: "19", Enabled: true}, // Item Status Update
					{Code: "99", Enabled: true}, // SC Status
					{Code: "93", Enabled: true}, // Login
					{Code: "97", Enabled: true}, // Request ACS Resend
				},
			},
		},
	}
}

// NewE2ESetupBulk creates an E2ESetup with AcceptBulkPayment=true for the test-inst tenant.
func NewE2ESetupBulk(t *testing.T) *E2ESetup {
	t.Helper()
	mockFolio := mocks.NewFolioMockServer()
	cfg := buildE2EConfigWithOptions(t, mockFolio.GetURL(), true)
	logger, _ := logging.NewLogger("warn", "")
	srv, err := server.NewServer(cfg, logger)
	require.NoError(t, err)
	srv.RegisterAllHandlers()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	go srv.Start(ctx)
	waitForServerReady(t, cfg.Port)

	return &E2ESetup{
		MockFolio: mockFolio,
		Server:    srv,
		Config:    cfg,
		Port:      cfg.Port,
		cancel:    cancel,
	}
}

// waitForServerReady polls until the TCP port accepts connections.
// Replaces all time.Sleep(200ms) calls — deterministic, no wasted time.
func waitForServerReady(t *testing.T, port int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 50*time.Millisecond)
		if err == nil {
			conn.Close()
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("SIP2 server on port %d did not become ready within 5s", port)
}

// getFreePort returns an available TCP port on localhost.
func getFreePort() (int, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port, nil
}

// =============================================================================
// SIP2 field parsing
// =============================================================================

// assertSIP2Field extracts and asserts the value of a SIP2 field in a response.
// fieldCode is the 2-char code, e.g. "AO", "AA", "AB".
func assertSIP2Field(t *testing.T, response, fieldCode, expected string) {
	t.Helper()
	value := findSIP2Field(response, fieldCode)
	assert.Equal(t, expected, value, "SIP2 field %s", fieldCode)
}

// findSIP2Field returns the value of a named SIP2 field, or "" if not found.
func findSIP2Field(response, fieldCode string) string {
	idx := findSubstringIndex(response, fieldCode)
	if idx == -1 {
		return ""
	}
	start := idx + len(fieldCode)
	end := findNextDelimiter(response, start)
	return response[start:end]
}

// =============================================================================
// String utilities
// =============================================================================

func findSubstringIndex(s, substr string) int {
	if len(substr) == 0 || len(s) < len(substr) {
		return -1
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

func findFieldIndex(s, fieldCode string) int {
	return findSubstringIndex(s, fieldCode)
}

func findNextDelimiter(s string, startIndex int) int {
	for i := startIndex; i < len(s); i++ {
		if s[i] == '|' || s[i] == '\r' {
			return i
		}
	}
	return len(s)
}
