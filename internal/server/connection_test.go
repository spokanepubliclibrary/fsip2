package server

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/spokanepubliclibrary/fsip2/internal/config"
	"github.com/spokanepubliclibrary/fsip2/internal/sip2/parser"
	"github.com/spokanepubliclibrary/fsip2/internal/tenant"
	"github.com/spokanepubliclibrary/fsip2/internal/types"
	"go.uber.org/zap"
)

// mockConn is a mock net.Conn for testing
type mockConn struct {
	readBuf  *bytes.Buffer
	writeBuf *bytes.Buffer
	closed   bool
}

func newMockConn(data string) *mockConn {
	return &mockConn{
		readBuf:  bytes.NewBufferString(data),
		writeBuf: &bytes.Buffer{},
		closed:   false,
	}
}

func (m *mockConn) Read(b []byte) (n int, err error) {
	return m.readBuf.Read(b)
}

func (m *mockConn) Write(b []byte) (n int, err error) {
	return m.writeBuf.Write(b)
}

func (m *mockConn) Close() error {
	m.closed = true
	return nil
}

func (m *mockConn) LocalAddr() net.Addr {
	addr, _ := net.ResolveTCPAddr("tcp", "127.0.0.1:6443")
	return addr
}

func (m *mockConn) RemoteAddr() net.Addr {
	addr, _ := net.ResolveTCPAddr("tcp", "127.0.0.1:12345")
	return addr
}

func (m *mockConn) SetDeadline(t time.Time) error {
	return nil
}

func (m *mockConn) SetReadDeadline(t time.Time) error {
	return nil
}

func (m *mockConn) SetWriteDeadline(t time.Time) error {
	return nil
}

func (m *mockConn) GetWritten() string {
	return m.writeBuf.String()
}

// TestReadMessageSingleByteDelimiter tests reading messages with single-byte delimiter
func TestReadMessageSingleByteDelimiter(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		delimiter string
		expected  string
	}{
		{
			name:      "Pipe delimiter",
			input:     "2300019700101    084625AOLIB|AA12345|AC|AD1234|",
			delimiter: "|",
			expected:  "2300019700101    084625AOLIB",
		},
		{
			name:      "Carriage return delimiter",
			input:     "9300CNtest|CO123|\r",
			delimiter: "\r",
			expected:  "9300CNtest|CO123|",
		},
		{
			name:      "Newline delimiter",
			input:     "9900302.00\n",
			delimiter: "\n",
			expected:  "9900302.00",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tenantConfig := &config.TenantConfig{
				Tenant:           "test",
				MessageDelimiter: tt.delimiter,
			}

			session := types.NewSession("test-session", tenantConfig)
			mockConn := newMockConn(tt.input)

			logger, _ := zap.NewDevelopment()
			cfg := &config.Config{OkapiURL: "https://folio.example.com"}
			server, _ := NewServer(cfg, logger)
			tenantService := tenant.NewService(cfg)

			conn := NewConnection(
				mockConn,
				session,
				tenantService,
				make(map[parser.MessageCode]MessageHandler),
				server,
			)

			reader := bufio.NewReader(mockConn.readBuf)
			message, err := conn.readMessage(reader)
			if err != nil {
				t.Fatalf("readMessage() error = %v", err)
			}

			if message != tt.expected {
				t.Errorf("Expected message '%s', got '%s'", tt.expected, message)
			}
		})
	}
}

// TestReadMessageMultiByteDelimiter tests reading messages with multi-byte delimiters
func TestReadMessageMultiByteDelimiter(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		delimiter string
		expected  string
	}{
		{
			name:      "Double pipe delimiter",
			input:     "2300019700101    084625AO||",
			delimiter: "||",
			expected:  "2300019700101    084625AO",
		},
		{
			name:      "CRLF delimiter",
			input:     "9300CNtest|CO123|\r\n",
			delimiter: "\r\n",
			expected:  "9300CNtest|CO123|",
		},
		{
			name:      "Custom multi-byte delimiter",
			input:     "TEST MESSAGE<END>",
			delimiter: "<END>",
			expected:  "TEST MESSAGE",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tenantConfig := &config.TenantConfig{
				Tenant:           "test",
				MessageDelimiter: tt.delimiter,
			}

			session := types.NewSession("test-session", tenantConfig)
			mockConn := newMockConn(tt.input)

			logger, _ := zap.NewDevelopment()
			cfg := &config.Config{OkapiURL: "https://folio.example.com"}
			server, _ := NewServer(cfg, logger)
			tenantService := tenant.NewService(cfg)

			conn := NewConnection(
				mockConn,
				session,
				tenantService,
				make(map[parser.MessageCode]MessageHandler),
				server,
			)

			reader := bufio.NewReader(mockConn.readBuf)
			message, err := conn.readMessage(reader)
			if err != nil {
				t.Fatalf("readMessage() error = %v", err)
			}

			if message != tt.expected {
				t.Errorf("Expected message '%s', got '%s'", tt.expected, message)
			}
		})
	}
}

// TestSendMessage tests sending messages to client
func TestSendMessage(t *testing.T) {
	tests := []struct {
		name     string
		message  string
		expected string
	}{
		{
			name:     "Simple response",
			message:  "941AY0AZ",
			expected: "941AY0AZ|",
		},
		{
			name:     "Patron status response",
			message:  "24              00019700101    084737AO|AA12345|",
			expected: "24              00019700101    084737AO|AA12345|",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tenantConfig := &config.TenantConfig{
				Tenant:           "test",
				MessageDelimiter: "|",
			}

			session := types.NewSession("test-session", tenantConfig)
			mockConn := newMockConn("")

			logger, _ := zap.NewDevelopment()
			cfg := &config.Config{OkapiURL: "https://folio.example.com"}
			server, _ := NewServer(cfg, logger)
			tenantService := tenant.NewService(cfg)

			conn := NewConnection(
				mockConn,
				session,
				tenantService,
				make(map[parser.MessageCode]MessageHandler),
				server,
			)

			err := conn.sendMessage(tt.message)
			if err != nil {
				t.Fatalf("sendMessage() error = %v", err)
			}

			written := mockConn.GetWritten()
			if written != tt.expected {
				t.Errorf("Expected written message '%s', got '%s'", tt.expected, written)
			}
		})
	}
}

// TestConnectionClose tests connection closure
func TestConnectionClose(t *testing.T) {
	tenantConfig := &config.TenantConfig{
		Tenant: "test",
	}

	session := types.NewSession("test-session", tenantConfig)
	mockConn := newMockConn("")

	logger, _ := zap.NewDevelopment()
	cfg := &config.Config{OkapiURL: "https://folio.example.com"}
	server, _ := NewServer(cfg, logger)
	tenantService := tenant.NewService(cfg)

	conn := NewConnection(
		mockConn,
		session,
		tenantService,
		make(map[parser.MessageCode]MessageHandler),
		server,
	)

	if mockConn.closed {
		t.Error("Connection should not be closed initially")
	}

	err := conn.Close()
	if err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	if !mockConn.closed {
		t.Error("Connection should be closed after Close()")
	}
}

// TestGetRemoteAddr tests getting remote address
func TestGetRemoteAddr(t *testing.T) {
	tenantConfig := &config.TenantConfig{
		Tenant: "test",
	}

	session := types.NewSession("test-session", tenantConfig)
	mockConn := newMockConn("")

	logger, _ := zap.NewDevelopment()
	cfg := &config.Config{OkapiURL: "https://folio.example.com"}
	server, _ := NewServer(cfg, logger)
	tenantService := tenant.NewService(cfg)

	conn := NewConnection(
		mockConn,
		session,
		tenantService,
		make(map[parser.MessageCode]MessageHandler),
		server,
	)

	addr := conn.GetRemoteAddr()
	if addr != "127.0.0.1:12345" {
		t.Errorf("Expected remote addr '127.0.0.1:12345', got '%s'", addr)
	}
}

// TestGetLocalAddr tests getting local address
func TestGetLocalAddr(t *testing.T) {
	tenantConfig := &config.TenantConfig{
		Tenant: "test",
	}

	session := types.NewSession("test-session", tenantConfig)
	mockConn := newMockConn("")

	logger, _ := zap.NewDevelopment()
	cfg := &config.Config{OkapiURL: "https://folio.example.com"}
	server, _ := NewServer(cfg, logger)
	tenantService := tenant.NewService(cfg)

	conn := NewConnection(
		mockConn,
		session,
		tenantService,
		make(map[parser.MessageCode]MessageHandler),
		server,
	)

	addr := conn.GetLocalAddr()
	if addr != "127.0.0.1:6443" {
		t.Errorf("Expected local addr '127.0.0.1:6443', got '%s'", addr)
	}
}

// TestGetClientIP tests extracting client IP
func TestGetClientIP(t *testing.T) {
	tenantConfig := &config.TenantConfig{
		Tenant: "test",
	}

	session := types.NewSession("test-session", tenantConfig)
	mockConn := newMockConn("")

	logger, _ := zap.NewDevelopment()
	cfg := &config.Config{OkapiURL: "https://folio.example.com"}
	server, _ := NewServer(cfg, logger)
	tenantService := tenant.NewService(cfg)

	conn := NewConnection(
		mockConn,
		session,
		tenantService,
		make(map[parser.MessageCode]MessageHandler),
		server,
	)

	ip, err := conn.GetClientIP()
	if err != nil {
		t.Fatalf("GetClientIP() error = %v", err)
	}

	if ip != "127.0.0.1" {
		t.Errorf("Expected client IP '127.0.0.1', got '%s'", ip)
	}
}

// TestGetClientPort tests extracting client port
func TestGetClientPort(t *testing.T) {
	tenantConfig := &config.TenantConfig{
		Tenant: "test",
	}

	session := types.NewSession("test-session", tenantConfig)
	mockConn := newMockConn("")

	logger, _ := zap.NewDevelopment()
	cfg := &config.Config{OkapiURL: "https://folio.example.com"}
	server, _ := NewServer(cfg, logger)
	tenantService := tenant.NewService(cfg)

	conn := NewConnection(
		mockConn,
		session,
		tenantService,
		make(map[parser.MessageCode]MessageHandler),
		server,
	)

	port, err := conn.GetClientPort()
	if err != nil {
		t.Fatalf("GetClientPort() error = %v", err)
	}

	if port != 12345 {
		t.Errorf("Expected client port 12345, got %d", port)
	}
}

// TestGetServerPort tests extracting server port
func TestGetServerPort(t *testing.T) {
	tenantConfig := &config.TenantConfig{
		Tenant: "test",
	}

	session := types.NewSession("test-session", tenantConfig)
	mockConn := newMockConn("")

	logger, _ := zap.NewDevelopment()
	cfg := &config.Config{OkapiURL: "https://folio.example.com"}
	server, _ := NewServer(cfg, logger)
	tenantService := tenant.NewService(cfg)

	conn := NewConnection(
		mockConn,
		session,
		tenantService,
		make(map[parser.MessageCode]MessageHandler),
		server,
	)

	port, err := conn.GetServerPort()
	if err != nil {
		t.Fatalf("GetServerPort() error = %v", err)
	}

	if port != 6443 {
		t.Errorf("Expected server port 6443, got %d", port)
	}
}

// TestProcessMessageWithHandler tests message processing with a mock handler
func TestProcessMessageWithHandler(t *testing.T) {
	tenantConfig := &config.TenantConfig{
		Tenant:           "test",
		MessageDelimiter: "|",
		SupportedMessages: []config.MessageSupport{
			{Code: "99", Enabled: true}, // SC Status
		},
	}

	session := types.NewSession("test-session", tenantConfig)
	mockConn := newMockConn("")

	logger, _ := zap.NewDevelopment()
	cfg := &config.Config{OkapiURL: "https://folio.example.com"}
	server, _ := NewServer(cfg, logger)
	tenantService := tenant.NewService(cfg)

	mockHandler := &MockHandler{}
	handlers := map[parser.MessageCode]MessageHandler{
		parser.SCStatus: mockHandler,
	}

	conn := NewConnection(
		mockConn,
		session,
		tenantService,
		handlers,
		server,
	)

	// SC Status request message
	rawMessage := "9900302.00"

	response, err := conn.processMessage(context.Background(), rawMessage)
	if err != nil {
		t.Fatalf("processMessage() error = %v", err)
	}

	if response != "OK" {
		t.Errorf("Expected response 'OK', got '%s'", response)
	}

	if !mockHandler.WasCalled() {
		t.Error("Handler should have been called")
	}

	if mockHandler.GetCallCount() != 1 {
		t.Errorf("Expected handler to be called once, got %d", mockHandler.GetCallCount())
	}
}

// TestProcessMessageNoHandler tests message processing with no registered handler
func TestProcessMessageNoHandler(t *testing.T) {
	tenantConfig := &config.TenantConfig{
		Tenant:           "test",
		MessageDelimiter: "|",
		SupportedMessages: []config.MessageSupport{
			{Code: "99", Enabled: true}, // SC Status
		},
	}

	session := types.NewSession("test-session", tenantConfig)
	mockConn := newMockConn("")

	logger, _ := zap.NewDevelopment()
	cfg := &config.Config{OkapiURL: "https://folio.example.com"}
	server, _ := NewServer(cfg, logger)
	tenantService := tenant.NewService(cfg)

	// No handlers registered
	handlers := map[parser.MessageCode]MessageHandler{}

	conn := NewConnection(
		mockConn,
		session,
		tenantService,
		handlers,
		server,
	)

	// SC Status request message
	rawMessage := "9900302.00"

	_, err := conn.processMessage(context.Background(), rawMessage)
	if err == nil {
		t.Error("processMessage() should return error when no handler is registered")
	}

	if !strings.Contains(err.Error(), "no handler") {
		t.Errorf("Error should mention no handler, got: %v", err)
	}
}

// TestProcessMessageInvalidMessage tests processing of invalid messages
func TestProcessMessageInvalidMessage(t *testing.T) {
	tenantConfig := &config.TenantConfig{
		Tenant:           "test",
		MessageDelimiter: "|",
	}

	session := types.NewSession("test-session", tenantConfig)
	mockConn := newMockConn("")

	logger, _ := zap.NewDevelopment()
	cfg := &config.Config{OkapiURL: "https://folio.example.com"}
	server, _ := NewServer(cfg, logger)
	tenantService := tenant.NewService(cfg)

	conn := NewConnection(
		mockConn,
		session,
		tenantService,
		make(map[parser.MessageCode]MessageHandler),
		server,
	)

	// Invalid message (too short)
	rawMessage := "99"

	_, err := conn.processMessage(context.Background(), rawMessage)
	if err == nil {
		t.Error("processMessage() should return error for invalid message")
	}
}

// TestConnectionSessionActivityUpdate tests that processing updates session activity
func TestConnectionSessionActivityUpdate(t *testing.T) {
	tenantConfig := &config.TenantConfig{
		Tenant:           "test",
		MessageDelimiter: "|",
		SupportedMessages: []config.MessageSupport{
			{Code: "99", Enabled: true}, // SC Status
		},
	}

	session := types.NewSession("test-session", tenantConfig)
	mockConn := newMockConn("")

	logger, _ := zap.NewDevelopment()
	cfg := &config.Config{OkapiURL: "https://folio.example.com"}
	server, _ := NewServer(cfg, logger)
	tenantService := tenant.NewService(cfg)

	mockHandler := &MockHandler{}
	handlers := map[parser.MessageCode]MessageHandler{
		parser.SCStatus: mockHandler,
	}

	conn := NewConnection(
		mockConn,
		session,
		tenantService,
		handlers,
		server,
	)

	// Wait a bit to establish some idle time
	time.Sleep(50 * time.Millisecond)
	initialIdleTime := session.GetIdleTime()

	// Process a message - should update activity
	rawMessage := "9900302.00"
	_, _ = conn.processMessage(context.Background(), rawMessage)

	// Check that idle time was reset
	newIdleTime := session.GetIdleTime()
	if newIdleTime >= initialIdleTime {
		t.Error("Session activity should have been updated during message processing")
	}
}

// mockHandlerFunc wraps a function literal as a MessageHandler.
type mockHandlerFunc struct {
	fn func(context.Context, *parser.Message, *types.Session) (string, error)
}

func (m *mockHandlerFunc) Handle(ctx context.Context, msg *parser.Message, s *types.Session) (string, error) {
	return m.fn(ctx, msg, s)
}

// newHandleTestConn creates a Connection backed by a mockConn for Handle() tests.
func newHandleTestConn(t *testing.T, data string, handlers map[parser.MessageCode]MessageHandler) (*Connection, *mockConn) {
	t.Helper()
	tc := &config.TenantConfig{
		Tenant:           "test-tenant",
		MessageDelimiter: "\r",
		FieldDelimiter:   "|",
		Charset:          "UTF-8",
		SupportedMessages: []config.MessageSupport{
			{Code: "99", Enabled: true}, // SCStatus
		},
	}
	mc := newMockConn(data)
	sess := types.NewSession("test-session", tc)
	logger, _ := zap.NewDevelopment()
	cfg := &config.Config{Tenants: map[string]*config.TenantConfig{tc.Tenant: tc}}
	srv, _ := NewServer(cfg, logger)
	ts := tenant.NewService(cfg)
	return NewConnection(mc, sess, ts, handlers, srv), mc
}

// TestConnectionHandle_EOF — empty connection closes cleanly (nil error).
func TestConnectionHandle_EOF(t *testing.T) {
	conn, _ := newHandleTestConn(t, "", nil)
	err := conn.Handle(context.Background())
	assert.NoError(t, err)
}

// TestConnectionHandle_SingleMessage — one valid SIP2 message is dispatched
// and the response is written before EOF causes a clean exit.
func TestConnectionHandle_SingleMessage(t *testing.T) {
	called := false
	handlers := map[parser.MessageCode]MessageHandler{
		parser.SCStatus: &mockHandlerFunc{fn: func(ctx context.Context, msg *parser.Message, s *types.Session) (string, error) {
			called = true
			return "9800302.00|AOTEST\r", nil
		}},
	}
	conn, mc := newHandleTestConn(t, "9900302.00\r", handlers)
	err := conn.Handle(context.Background())

	assert.NoError(t, err)
	assert.True(t, called, "handler should have been invoked")
	assert.NotEmpty(t, mc.GetWritten(), "response should have been sent")
}

// TestConnectionHandle_ContextCancelled — context cancel causes Handle to exit.
func TestConnectionHandle_ContextCancelled(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	tc := &config.TenantConfig{Tenant: "test", MessageDelimiter: "\r", FieldDelimiter: "|"}
	sess := types.NewSession("test-session", tc)
	logger := zap.NewNop()
	cfg := &config.Config{}
	srv, _ := NewServer(cfg, logger)
	ts := tenant.NewService(cfg)
	conn := NewConnection(serverConn, sess, ts, nil, srv)

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() { errCh <- conn.Handle(ctx) }()

	cancel()
	serverConn.Close() // unblocks the blocked read so Handle can exit

	select {
	case err := <-errCh:
		assert.Error(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("Handle() did not return after context cancel")
	}
}

// TestConnectionHandle_ProcessError — handler error causes error response,
// loop continues, then EOF causes clean exit.
func TestConnectionHandle_ProcessError(t *testing.T) {
	handlers := map[parser.MessageCode]MessageHandler{
		parser.SCStatus: &mockHandlerFunc{fn: func(ctx context.Context, msg *parser.Message, s *types.Session) (string, error) {
			return "", fmt.Errorf("simulated handler error")
		}},
	}
	conn, mc := newHandleTestConn(t, "9900302.00\r", handlers)
	err := conn.Handle(context.Background())

	assert.NoError(t, err)
	_ = mc.GetWritten()
}

// newBareConnection creates a minimal Connection (no data, no handlers) for unit testing.
func newBareConnection(t *testing.T) *Connection {
	t.Helper()
	tc := &config.TenantConfig{Tenant: "test", MessageDelimiter: "\r", FieldDelimiter: "|"}
	mc := newMockConn("")
	sess := types.NewSession("test-session", tc)
	logger, _ := zap.NewDevelopment()
	cfg := &config.Config{}
	srv, _ := NewServer(cfg, logger)
	ts := tenant.NewService(cfg)
	return NewConnection(mc, sess, ts, nil, srv)
}

func TestBuildErrorResponse_CheckoutMessage(t *testing.T) {
	conn := newBareConnection(t)
	raw := "11YN20250110    08150020250110    081500|AOtest|AA123456|ABitem001"
	resp := conn.buildErrorResponse(raw, fmt.Errorf("checkout failed"))
	assert.Contains(t, resp, "12") // CheckoutResponse code
	assert.Contains(t, resp, "|AFcheckout failed")
}

func TestBuildErrorResponse_CheckinMessage(t *testing.T) {
	conn := newBareConnection(t)
	raw := "09N20250110    08150020250110    081500|AOtest|ABitem001"
	resp := conn.buildErrorResponse(raw, fmt.Errorf("checkin failed"))
	assert.Contains(t, resp, "10")
}

func TestBuildErrorResponse_PatronStatusMessage(t *testing.T) {
	conn := newBareConnection(t)
	raw := "2300020250110    081500|AOtest|AA123456"
	resp := conn.buildErrorResponse(raw, fmt.Errorf("patron error"))
	assert.Contains(t, resp, "24")
	assert.Contains(t, resp, "YYYYYYYYYYYYYY") // all-blocked status
}

func TestBuildErrorResponse_DefaultMessage(t *testing.T) {
	conn := newBareConnection(t)
	raw := "9900302.00" // SCStatus — maps to ACSStatus response (98)
	resp := conn.buildErrorResponse(raw, fmt.Errorf("sc error"))
	assert.Contains(t, resp, "98")
}

func TestBuildErrorResponse_UnparsableMessage(t *testing.T) {
	conn := newBareConnection(t)
	resp := conn.buildErrorResponse("GARBAGE_NOT_SIP2", fmt.Errorf("error"))
	assert.Empty(t, resp) // can't parse → returns ""
}

func TestBuildErrorResponse_LongError(t *testing.T) {
	conn := newBareConnection(t)
	raw := "2300020250110    081500|AOtest|AA123456"
	long := strings.Repeat("x", 300)
	resp := conn.buildErrorResponse(raw, fmt.Errorf("%s", long))
	// Error is truncated to 255 chars in AF field
	assert.NotEmpty(t, resp)
}

// TestHandleLoginTenantResolution_NoChange — single tenant, no login resolvers:
// ResolveAtLogin returns the current tenant unchanged.
func TestHandleLoginTenantResolution_NoChange(t *testing.T) {
	tc := &config.TenantConfig{Tenant: "test", MessageDelimiter: "\r", FieldDelimiter: "|"}
	mc := newMockConn("")
	sess := types.NewSession("test-session", tc)
	logger, _ := zap.NewDevelopment()
	cfg := &config.Config{Tenants: map[string]*config.TenantConfig{tc.Tenant: tc}}
	srv, _ := NewServer(cfg, logger)
	ts := tenant.NewService(cfg)
	conn := NewConnection(mc, sess, ts, nil, srv)

	msg := &parser.Message{
		Code: parser.LoginRequest,
		Fields: map[string]string{
			string(parser.PatronIdentifier): "testuser",
			string(parser.InstitutionID):    "test",
		},
		MultiValueFields: map[string][]string{},
	}

	err := conn.handleLoginTenantResolution(context.Background(), msg)
	assert.NoError(t, err)
	assert.Equal(t, "test", conn.session.TenantConfig.Tenant)
}

// TestHandleLoginTenantResolution_TenantSwitch — username prefix resolver fires,
// session tenant is updated to the matched sub-tenant.
func TestHandleLoginTenantResolution_TenantSwitch(t *testing.T) {
	defaultTc := &config.TenantConfig{
		Tenant:           "default",
		MessageDelimiter: "\r",
		FieldDelimiter:   "|",
	}
	branchTc := &config.TenantConfig{
		Tenant:           "branch",
		MessageDelimiter: "\r",
		FieldDelimiter:   "|",
	}
	cfg := &config.Config{
		Tenants: map[string]*config.TenantConfig{
			"default": defaultTc,
			"branch":  branchTc,
		},
		SCTenants: []config.SCTenantConfig{
			{Tenant: "branch", UsernamePrefixes: []string{"BRANCH-"}},
		},
	}
	logger, _ := zap.NewDevelopment()
	srv, _ := NewServer(cfg, logger)
	ts := tenant.NewService(cfg)
	mc := newMockConn("")
	sess := types.NewSession("test-session", defaultTc)
	conn := NewConnection(mc, sess, ts, nil, srv)

	msg := &parser.Message{
		Code: parser.LoginRequest,
		Fields: map[string]string{
			string(parser.LoginUserID): "BRANCH-user123",
		},
		MultiValueFields: map[string][]string{},
	}

	err := conn.handleLoginTenantResolution(context.Background(), msg)
	assert.NoError(t, err)
	assert.Equal(t, "branch", conn.session.TenantConfig.Tenant)
}

// TestReadMessageDelimiterDetection tests delimiter detection algorithm
func TestReadMessageDelimiterDetection(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		delimiter  string
		wantOutput string
		wantError  bool
	}{
		{
			name:       "Single byte delimiter at end",
			input:      "Hello World|",
			delimiter:  "|",
			wantOutput: "Hello World",
			wantError:  false,
		},
		{
			name:       "Multi-byte delimiter at end",
			input:      "Hello World||",
			delimiter:  "||",
			wantOutput: "Hello World",
			wantError:  false,
		},
		{
			name:       "Delimiter appears in message body",
			input:      "Field1|Field2|Field3|",
			delimiter:  "|",
			wantOutput: "Field1",
			wantError:  false,
		},
		{
			name:       "CRLF delimiter",
			input:      "Test Message\r\n",
			delimiter:  "\r\n",
			wantOutput: "Test Message",
			wantError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tenantConfig := &config.TenantConfig{
				Tenant:           "test",
				MessageDelimiter: tt.delimiter,
			}

			session := types.NewSession("test-session", tenantConfig)
			mockConn := newMockConn(tt.input)

			logger, _ := zap.NewDevelopment()
			cfg := &config.Config{OkapiURL: "https://folio.example.com"}
			server, _ := NewServer(cfg, logger)
			tenantService := tenant.NewService(cfg)

			conn := NewConnection(
				mockConn,
				session,
				tenantService,
				make(map[parser.MessageCode]MessageHandler),
				server,
			)

			reader := bufio.NewReader(mockConn.readBuf)
			message, err := conn.readMessage(reader)

			if (err != nil) != tt.wantError {
				t.Errorf("readMessage() error = %v, wantError %v", err, tt.wantError)
				return
			}

			if !tt.wantError && message != tt.wantOutput {
				t.Errorf("Expected message '%s', got '%s'", tt.wantOutput, message)
			}
		})
	}
}

// TestReadMessage_MaxSizeGuard tests the maxSIP2MessageBytes size guard in readMessage.
// The guard fires when len(message) > maxSIP2MessageBytes (65536), so exactly 65536
// bytes of payload (plus delimiter) must succeed, while 65537+ bytes with no delimiter
// must return the "exceeded maximum size" error.
func TestReadMessage_MaxSizeGuard(t *testing.T) {
	t.Parallel()

	const delimiter = "|"
	const maxBytes = 64 * 1024 // must match maxSIP2MessageBytes in connection.go

	buildConn := func(t *testing.T, input []byte) *Connection {
		t.Helper()
		tc := &config.TenantConfig{
			Tenant:           "test",
			MessageDelimiter: delimiter,
		}
		mc := &mockConn{
			readBuf:  bytes.NewBuffer(input),
			writeBuf: &bytes.Buffer{},
		}
		sess := types.NewSession("test-session", tc)
		logger, _ := zap.NewDevelopment()
		cfg := &config.Config{OkapiURL: "https://folio.example.com"}
		srv, _ := NewServer(cfg, logger)
		ts := tenant.NewService(cfg)
		return NewConnection(mc, sess, ts, make(map[parser.MessageCode]MessageHandler), srv)
	}

	tests := []struct {
		name        string
		buildInput  func() []byte
		wantErr     bool
		errContains string
		wantLen     int // expected payload byte length (ignored when wantErr is true)
	}{
		{
			name: "WithinLimit — 1000-byte payload plus delimiter succeeds",
			buildInput: func() []byte {
				payload := bytes.Repeat([]byte("A"), 1000)
				return append(payload, []byte(delimiter)...)
			},
			wantErr: false,
			wantLen: 1000,
		},
		{
			name: "ExactlyAtLimit — 65536-byte payload plus delimiter succeeds",
			buildInput: func() []byte {
				// 65536 bytes == maxBytes; guard fires only when len > maxBytes,
				// so this must return successfully.
				payload := bytes.Repeat([]byte("B"), maxBytes)
				return append(payload, []byte(delimiter)...)
			},
			wantErr: false,
			wantLen: maxBytes,
		},
		{
			name: "ExceedsLimit — 65537 bytes with no delimiter returns error",
			buildInput: func() []byte {
				// One byte over the limit, no delimiter — the guard triggers before
				// EOF is reached.
				return bytes.Repeat([]byte("C"), maxBytes+1)
			},
			wantErr:     true,
			errContains: "exceeded maximum size",
		},
	}

	for _, tt := range tests {
		tt := tt // capture range variable
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			input := tt.buildInput()
			conn := buildConn(t, input)
			reader := bufio.NewReader(bytes.NewReader(input))

			msg, err := conn.readMessage(reader)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("readMessage() expected error containing %q, got nil error (message len=%d)", tt.errContains, len(msg))
				}
				if !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("readMessage() error = %q, want it to contain %q", err.Error(), tt.errContains)
				}
				return
			}

			if err != nil {
				t.Fatalf("readMessage() unexpected error: %v", err)
			}
			if len(msg) != tt.wantLen {
				t.Errorf("readMessage() returned %d bytes, want %d", len(msg), tt.wantLen)
			}
		})
	}
}

// TestHandleLoginTenantResolution_UsernamePrefix — CN field (LoginUserID) drives tenant
// resolution via UsernamePrefixes. The AA field (PatronIdentifier) is intentionally absent
// so the test fails if the implementation reads PatronIdentifier instead of LoginUserID.
func TestHandleLoginTenantResolution_UsernamePrefix(t *testing.T) {
	defaultTc := &config.TenantConfig{
		Tenant:           "default",
		MessageDelimiter: "\r",
		FieldDelimiter:   "|",
	}
	lib4Tc := &config.TenantConfig{
		Tenant:           "lib4",
		MessageDelimiter: "\r",
		FieldDelimiter:   "|",
	}
	cfg := &config.Config{
		Tenants: map[string]*config.TenantConfig{
			"default": defaultTc,
			"lib4":    lib4Tc,
		},
		SCTenants: []config.SCTenantConfig{
			{Tenant: "lib4", UsernamePrefixes: []string{"lib4_"}},
		},
	}
	logger, _ := zap.NewDevelopment()
	srv, _ := NewServer(cfg, logger)
	ts := tenant.NewService(cfg)
	mc := newMockConn("")
	sess := types.NewSession("test-session", defaultTc)
	conn := NewConnection(mc, sess, ts, nil, srv)

	// CN = LoginUserID set to a lib4-prefixed username.
	// AA = PatronIdentifier is deliberately absent (or set to a non-matching value)
	// to prove it is CN — not AA — that drives resolution.
	msg := &parser.Message{
		Code: parser.LoginRequest,
		Fields: map[string]string{
			string(parser.LoginUserID):   "lib4_sip1",
			string(parser.LoginPassword): "secret",
		},
		MultiValueFields: map[string][]string{},
	}

	err := conn.handleLoginTenantResolution(context.Background(), msg)
	assert.NoError(t, err)
	assert.Equal(t, "lib4", conn.session.TenantConfig.Tenant,
		"tenant should be resolved to 'lib4' via LoginUserID prefix, not 'default'")
}
