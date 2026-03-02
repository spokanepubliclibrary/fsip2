package server

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/spokanepubliclibrary/fsip2/internal/config"
	"github.com/spokanepubliclibrary/fsip2/internal/sip2/parser"
	"github.com/spokanepubliclibrary/fsip2/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// TestServerInitialization tests server creation and initialization
func TestServerInitialization(t *testing.T) {
	cfg := &config.Config{
		Port:     6443,
		OkapiURL: "https://folio.example.com",
		Tenants: map[string]*config.TenantConfig{
			"test": {
				Tenant:   "test",
				OkapiURL: "https://folio.example.com",
			},
		},
	}
	logger, _ := zap.NewDevelopment()

	server, err := NewServer(cfg, logger)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	if server.config != cfg {
		t.Error("Server config not set correctly")
	}

	if server.logger != logger {
		t.Error("Server logger not set correctly")
	}

	if server.isRunning {
		t.Error("New server should not be running")
	}

	if server.handlers == nil {
		t.Error("Handlers map should be initialized")
	}

	if server.tenantService == nil {
		t.Error("Tenant service should be initialized")
	}

	if server.metrics == nil {
		t.Error("Metrics should be initialized")
	}
}

// TestServerIsRunning tests the IsRunning method
func TestServerIsRunning(t *testing.T) {
	cfg := &config.Config{
		Port:     0, // Use random available port
		OkapiURL: "https://folio.example.com",
	}
	logger, _ := zap.NewDevelopment()

	server, err := NewServer(cfg, logger)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Server should not be running initially
	if server.IsRunning() {
		t.Error("New server should not be running")
	}

	// Manually set running state for testing
	server.mu.Lock()
	server.isRunning = true
	server.mu.Unlock()

	if !server.IsRunning() {
		t.Error("Server should be running after setting state")
	}

	// Reset state
	server.mu.Lock()
	server.isRunning = false
	server.mu.Unlock()

	if server.IsRunning() {
		t.Error("Server should not be running after resetting state")
	}
}

// TestRegisterHandler tests handler registration
func TestRegisterHandler(t *testing.T) {
	cfg := &config.Config{
		Port:     0,
		OkapiURL: "https://folio.example.com",
	}
	logger, _ := zap.NewDevelopment()

	server, err := NewServer(cfg, logger)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Create a mock handler
	mockHandler := &MockHandler{}

	// Register handler
	server.RegisterHandler(parser.PatronStatusRequest, mockHandler)

	// Verify handler was registered
	server.mu.RLock()
	handler, ok := server.handlers[parser.PatronStatusRequest]
	server.mu.RUnlock()

	if !ok {
		t.Error("Handler should be registered")
	}

	if handler != mockHandler {
		t.Error("Registered handler should match the one we provided")
	}
}

// TestRegisterAllHandlers tests that all handlers are registered
func TestRegisterAllHandlers(t *testing.T) {
	cfg := &config.Config{
		Port:     0,
		OkapiURL: "https://folio.example.com",
	}
	logger, _ := zap.NewDevelopment()

	server, err := NewServer(cfg, logger)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Register all handlers
	server.RegisterAllHandlers()

	// Expected handlers (14 message types)
	expectedHandlers := []parser.MessageCode{
		parser.LoginRequest,
		parser.SCStatus,
		parser.PatronStatusRequest,
		parser.CheckoutRequest,
		parser.CheckinRequest,
		parser.PatronInformationRequest,
		parser.ItemInformationRequest,
		parser.RenewRequest,
		parser.RenewAllRequest,
		parser.EndPatronSessionRequest,
		parser.FeePaidRequest,
		parser.ItemStatusUpdateRequest,
		parser.RequestSCResend,
		parser.RequestACSResend,
	}

	server.mu.RLock()
	handlerCount := len(server.handlers)
	server.mu.RUnlock()

	if handlerCount != 14 {
		t.Errorf("Expected 14 handlers to be registered, got %d", handlerCount)
	}

	// Verify each expected handler is registered
	for _, code := range expectedHandlers {
		server.mu.RLock()
		_, ok := server.handlers[code]
		server.mu.RUnlock()

		if !ok {
			t.Errorf("Handler for message code %s not registered", code)
		}
	}
}

// TestServerStartStop tests server start and stop
func TestServerStartStop(t *testing.T) {
	cfg := &config.Config{
		Host:     "127.0.0.1",
		Port:     0, // Use random available port
		OkapiURL: "https://folio.example.com",
	}
	logger, _ := zap.NewDevelopment()

	server, err := NewServer(cfg, logger)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	server.RegisterAllHandlers()

	// Start server in a goroutine
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errChan := make(chan error, 1)
	go func() {
		errChan <- server.Start(ctx)
	}()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Verify server is running
	if !server.IsRunning() {
		t.Error("Server should be running after Start()")
	}

	// Verify listener is created
	if server.listener == nil {
		t.Error("Server listener should be created")
	}

	// Stop server
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer stopCancel()

	if err := server.Stop(stopCtx); err != nil {
		t.Errorf("Failed to stop server: %v", err)
	}

	// Verify server is stopped
	if server.IsRunning() {
		t.Error("Server should not be running after Stop()")
	}

	// Cancel context to exit Start()
	cancel()

	// Wait for Start() to complete
	select {
	case err := <-errChan:
		if err != nil && err != context.Canceled {
			t.Errorf("Server Start() returned unexpected error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Error("Timeout waiting for server to stop")
	}
}

// TestServerAlreadyRunning tests that starting an already running server returns an error
func TestServerAlreadyRunning(t *testing.T) {
	cfg := &config.Config{
		Port:     0,
		OkapiURL: "https://folio.example.com",
	}
	logger, _ := zap.NewDevelopment()

	server, err := NewServer(cfg, logger)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Manually set running state
	server.mu.Lock()
	server.isRunning = true
	server.mu.Unlock()

	// Try to start server
	err = server.Start(context.Background())
	if err == nil {
		t.Error("Starting an already running server should return an error")
	}

	expectedError := "server is already running"
	if err.Error() != expectedError {
		t.Errorf("Expected error '%s', got '%s'", expectedError, err.Error())
	}
}

// TestServerConnectionCounters tests connection tracking
func TestServerConnectionCounters(t *testing.T) {
	cfg := &config.Config{
		Port:     0,
		OkapiURL: "https://folio.example.com",
	}
	logger, _ := zap.NewDevelopment()

	server, err := NewServer(cfg, logger)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Initial counters should be zero
	if server.GetActiveConnections() != 0 {
		t.Errorf("Expected 0 active connections, got %d", server.GetActiveConnections())
	}

	if server.GetTotalConnections() != 0 {
		t.Errorf("Expected 0 total connections, got %d", server.GetTotalConnections())
	}

	// Simulate connection increment
	server.activeConnections = 5
	server.totalConnections = 10

	if server.GetActiveConnections() != 5 {
		t.Errorf("Expected 5 active connections, got %d", server.GetActiveConnections())
	}

	if server.GetTotalConnections() != 10 {
		t.Errorf("Expected 10 total connections, got %d", server.GetTotalConnections())
	}
}

// TestServerGracefulShutdown tests graceful shutdown with active connections
func TestServerGracefulShutdown(t *testing.T) {
	cfg := &config.Config{
		Host:     "127.0.0.1",
		Port:     0,
		OkapiURL: "https://folio.example.com",
	}
	logger, _ := zap.NewDevelopment()

	server, err := NewServer(cfg, logger)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	server.RegisterAllHandlers()

	// Start server
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = server.Start(ctx)
	}()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Simulate active connection by incrementing WaitGroup
	server.wg.Add(1)
	activeConnDone := make(chan bool)
	go func() {
		// Simulate a slow connection that takes 500ms to complete
		time.Sleep(500 * time.Millisecond)
		server.wg.Done()
		activeConnDone <- true
	}()

	// Stop server - should wait for the active connection
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer stopCancel()

	stopStart := time.Now()
	if err := server.Stop(stopCtx); err != nil {
		t.Errorf("Failed to stop server: %v", err)
	}
	stopDuration := time.Since(stopStart)

	// Verify it waited for the connection
	if stopDuration < 500*time.Millisecond {
		t.Errorf("Server should have waited for active connection, but stopped too quickly: %v", stopDuration)
	}

	// Verify the simulated connection completed
	select {
	case <-activeConnDone:
		// Connection completed as expected
	case <-time.After(1 * time.Second):
		t.Error("Active connection did not complete")
	}

	// Cancel context
	cancel()
}

// TestServerShutdownTimeout tests that shutdown respects timeout
func TestServerShutdownTimeout(t *testing.T) {
	cfg := &config.Config{
		Port:     0,
		OkapiURL: "https://folio.example.com",
	}
	logger, _ := zap.NewDevelopment()

	server, err := NewServer(cfg, logger)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Simulate a connection that never completes
	server.wg.Add(1)

	// Stop server with timeout - should return after 30s internal timeout
	// We'll use a 1 second timeout for testing
	stopStart := time.Now()

	// Create a mock listener to avoid nil pointer
	listener, _ := net.Listen("tcp", "127.0.0.1:0")
	server.listener = listener
	defer listener.Close()

	// Manually set running state
	server.mu.Lock()
	server.isRunning = true
	server.mu.Unlock()

	// Stop will wait up to 30 seconds for connections to close
	// We can't actually wait that long in tests, so this test verifies
	// the shutdown process starts and doesn't hang indefinitely
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer stopCancel()

	// Stop should complete even with hanging connection
	_ = server.Stop(stopCtx)
	stopDuration := time.Since(stopStart)

	// The Stop should have attempted to wait but we're just verifying it doesn't panic
	// In real scenarios, the 30s timeout would trigger
	if stopDuration > 5*time.Second {
		t.Error("Stop() took too long even with timeout")
	}

	// Clean up the hanging WaitGroup for test cleanup
	server.wg.Done()
}

// TestServerGetters tests the various getter methods
func TestServerGetters(t *testing.T) {
	cfg := &config.Config{
		Port:     6443,
		OkapiURL: "https://folio.example.com",
	}
	logger, _ := zap.NewDevelopment()

	server, err := NewServer(cfg, logger)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Test GetConfig
	if server.GetConfig() != cfg {
		t.Error("GetConfig should return the server config")
	}

	// Test GetLogger
	if server.GetLogger() != logger {
		t.Error("GetLogger should return the server logger")
	}

	// Test GetTenantService
	if server.GetTenantService() == nil {
		t.Error("GetTenantService should return the tenant service")
	}

	// Test GetMetrics
	if server.GetMetrics() == nil {
		t.Error("GetMetrics should return the metrics")
	}
}

// TestServerConcurrentConnections tests handling multiple concurrent connections
func TestServerConcurrentConnections(t *testing.T) {
	cfg := &config.Config{
		Port:     0,
		OkapiURL: "https://folio.example.com",
	}
	logger, _ := zap.NewDevelopment()

	server, err := NewServer(cfg, logger)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Simulate multiple concurrent connections
	var wg sync.WaitGroup
	connectionCount := 10

	for i := 0; i < connectionCount; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			// Simulate connection handling
			server.activeConnections++
			server.totalConnections++
			time.Sleep(10 * time.Millisecond)
			server.activeConnections--
		}(i)
	}

	wg.Wait()

	// Verify total connections
	if server.GetTotalConnections() != int64(connectionCount) {
		t.Errorf("Expected %d total connections, got %d", connectionCount, server.GetTotalConnections())
	}

	// Verify all connections closed (active should be 0)
	if server.GetActiveConnections() != 0 {
		t.Errorf("Expected 0 active connections after all completed, got %d", server.GetActiveConnections())
	}
}

// TestServerErrorHandlingPortInUse tests error when port is already in use
func TestServerErrorHandlingPortInUse(t *testing.T) {
	// Create a listener to occupy a port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create test listener: %v", err)
	}
	defer listener.Close()

	// Get the port that's now in use
	addr := listener.Addr().(*net.TCPAddr)
	port := addr.Port

	// Try to start server on the same port
	cfg := &config.Config{
		Host:     "127.0.0.1",
		Port:     port,
		OkapiURL: "https://folio.example.com",
	}
	logger, _ := zap.NewDevelopment()

	server, err := NewServer(cfg, logger)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	server.RegisterAllHandlers()

	// Start should fail because port is in use
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err = server.Start(ctx)
	if err == nil {
		t.Error("Starting server on occupied port should return an error")
	}

	// Error message should mention the failure
	if err != nil && err.Error() == "" {
		t.Error("Error should have a descriptive message")
	}
}

// MockHandler is a mock implementation of MessageHandler for testing
type MockHandler struct {
	called    bool
	callCount int
	mu        sync.Mutex
}

func (h *MockHandler) Handle(ctx context.Context, msg *parser.Message, session *types.Session) (string, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.called = true
	h.callCount++
	return "OK", nil
}

func (h *MockHandler) WasCalled() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.called
}

func (h *MockHandler) GetCallCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.callCount
}

// TestHandleConnection_CleanDisconnect tests that handleConnection exits cleanly
// when the client disconnects immediately (EOF on first read).
func TestHandleConnection_CleanDisconnect(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	tc := &config.TenantConfig{
		Tenant:           "default",
		MessageDelimiter: "\r",
		FieldDelimiter:   "|",
	}
	cfg := &config.Config{
		Tenants: map[string]*config.TenantConfig{
			"default": tc,
		},
	}
	srv, err := NewServer(cfg, logger)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	mc := newMockConn("") // EOF immediately
	err = srv.handleConnection(context.Background(), mc)
	if err != nil {
		t.Errorf("handleConnection() returned unexpected error: %v", err)
	}
}

// TestServerStart_TLSEnabled tests that Start() creates a TLS listener and runs successfully.
// It verifies the TLS path by starting the server, confirming the listener exists, then stopping it.
func TestServerStart_TLSEnabled(t *testing.T) {
	certFile, keyFile := generateTestCertFiles(t)

	cfg := &config.Config{
		Host: "127.0.0.1",
		Port: 0,
		TLS: &config.TLSConfig{
			Enabled:  true,
			CertFile: certFile,
			KeyFile:  keyFile,
		},
	}
	logger, _ := zap.NewDevelopment()
	srv, err := NewServer(cfg, logger)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Start(ctx)
	}()

	// Give server time to start and create the TLS listener
	time.Sleep(100 * time.Millisecond)
	require.True(t, srv.IsRunning(), "server should be running after Start()")
	require.NotNil(t, srv.listener, "TLS listener should be created")

	stopCtx, stopCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer stopCancel()
	require.NoError(t, srv.Stop(stopCtx))
	cancel()

	select {
	case startErr := <-errCh:
		assert.True(t, startErr == nil || errors.Is(startErr, context.Canceled),
			"unexpected Start() error: %v", startErr)
	case <-time.After(2 * time.Second):
		t.Fatal("Start() did not return after Stop()")
	}
}

// TestServerStart_TLSMissingCert tests that Start() returns an error when TLS cert files are missing.
func TestServerStart_TLSMissingCert(t *testing.T) {
	cfg := &config.Config{
		Host: "127.0.0.1",
		Port: 0,
		TLS: &config.TLSConfig{
			Enabled:  true,
			CertFile: "/nonexistent/cert.pem",
			KeyFile:  "/nonexistent/key.pem",
		},
	}
	logger, _ := zap.NewDevelopment()
	srv, err := NewServer(cfg, logger)
	require.NoError(t, err)

	err = srv.Start(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load TLS config")
}

// TestServerStart_AcceptsConnection exercises the connection-accepting goroutine in Start()
// (lines 161-184 of server.go) by dialing the running server with a real TCP client.
func TestServerStart_AcceptsConnection(t *testing.T) {
	cfg := &config.Config{
		Host: "127.0.0.1",
		Port: 0, // OS assigns
		Tenants: map[string]*config.TenantConfig{
			"default": {Tenant: "default", MessageDelimiter: "\r", FieldDelimiter: "|"},
		},
	}
	logger, _ := zap.NewDevelopment()
	srv, err := NewServer(cfg, logger)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	go func() { errCh <- srv.Start(ctx) }()

	// Wait for server to be running AND listener to be assigned
	require.Eventually(t, func() bool {
		return srv.IsRunning() && srv.listener != nil
	}, 2*time.Second, 10*time.Millisecond, "server did not start in time")

	// Dial the server — exercises the connection-accepting goroutine
	addr := srv.listener.Addr().String()
	conn, dialErr := net.Dial("tcp", addr)
	require.NoError(t, dialErr)
	conn.Close()

	// Give the connection goroutine time to process
	time.Sleep(100 * time.Millisecond)

	stopCtx, stopCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer stopCancel()
	require.NoError(t, srv.Stop(stopCtx))
	cancel()

	select {
	case startErr := <-errCh:
		assert.True(t, startErr == nil || errors.Is(startErr, context.Canceled),
			"unexpected Start() error: %v", startErr)
	case <-time.After(3 * time.Second):
		t.Fatal("Start() did not return after Stop()")
	}
}

// TestServerHandlerRegistrationRace tests concurrent handler registration
func TestServerHandlerRegistrationRace(t *testing.T) {
	cfg := &config.Config{
		Port:     0,
		OkapiURL: "https://folio.example.com",
	}
	logger, _ := zap.NewDevelopment()

	server, err := NewServer(cfg, logger)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	var wg sync.WaitGroup
	handlers := 100

	// Register multiple handlers concurrently
	for i := 0; i < handlers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			mockHandler := &MockHandler{}
			// Use a valid message code for registration
			code := parser.MessageCode(fmt.Sprintf("%02d", id%13+10))
			server.RegisterHandler(code, mockHandler)
		}(i)
	}

	wg.Wait()

	// Verify handlers were registered (should have at least some handlers)
	server.mu.RLock()
	handlerCount := len(server.handlers)
	server.mu.RUnlock()

	if handlerCount == 0 {
		t.Error("No handlers were registered")
	}
}
