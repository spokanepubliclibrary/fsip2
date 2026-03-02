// +build integration

package integration

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/spokanepubliclibrary/fsip2/internal/config"
	"github.com/spokanepubliclibrary/fsip2/internal/logging"
	"github.com/spokanepubliclibrary/fsip2/internal/server"
)

// TestServerStartStop tests basic server lifecycle
func TestServerStartStop(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create test configuration
	cfg := &config.Config{
		Port:            16666, // Use non-standard port for testing
		OkapiURL:        "http://localhost:9130",
		LogLevel:        "info",
		HealthCheckPort: 18081,
		Tenants: map[string]*config.TenantConfig{
			"test-tenant": {
				Tenant:           "test-tenant",
				MessageDelimiter: "\r",
				FieldDelimiter:   "|",
				OkapiURL:         "http://localhost:9130",
				Charset:          "UTF-8",
			},
		},
	}

	// Create logger
	logger, err := logging.NewLogger("info", "")
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Sync()

	// Create server
	srv, err := server.NewServer(cfg, logger)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Register handlers
	srv.RegisterAllHandlers()

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Start server in goroutine
	errChan := make(chan error, 1)
	go func() {
		errChan <- srv.Start(ctx)
	}()

	// Wait a bit for server to start
	time.Sleep(100 * time.Millisecond)

	// Verify server is running
	if !srv.IsRunning() {
		t.Error("Expected server to be running")
	}

	// Try to connect to the server
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", cfg.Port), 2*time.Second)
	if err != nil {
		t.Fatalf("Failed to connect to server: %v", err)
	}
	conn.Close()

	// Stop server
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer stopCancel()

	if err := srv.Stop(stopCtx); err != nil {
		t.Errorf("Failed to stop server: %v", err)
	}

	// Verify server stopped
	if srv.IsRunning() {
		t.Error("Expected server to be stopped")
	}
}

// TestServerSCStatusRequest tests SC Status message handling
func TestServerSCStatusRequest(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create test configuration
	cfg := &config.Config{
		Port:            16667,
		OkapiURL:        "http://localhost:9130",
		LogLevel:        "info",
		HealthCheckPort: 18082,
		Tenants: map[string]*config.TenantConfig{
			"test-tenant": {
				Tenant:           "test-tenant",
				MessageDelimiter: "\r",
				FieldDelimiter:   "|",
				OkapiURL:         "http://localhost:9130",
				Charset:          "UTF-8",
			},
		},
	}

	// Create logger
	logger, err := logging.NewLogger("info", "")
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Sync()

	// Create and start server
	srv, err := server.NewServer(cfg, logger)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	srv.RegisterAllHandlers()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	go func() {
		srv.Start(ctx)
	}()

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)

	// Connect to server
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", cfg.Port), 2*time.Second)
	if err != nil {
		t.Fatalf("Failed to connect to server: %v", err)
	}
	defer conn.Close()

	// Send SC Status request (99)
	request := "990302.00\r"
	_, err = conn.Write([]byte(request))
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}

	// Read response with timeout
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		t.Fatalf("Failed to read response: %v", err)
	}

	response := string(buf[:n])

	// Verify response starts with 98 (ACS Status Response)
	if len(response) < 2 || response[:2] != "98" {
		t.Errorf("Expected response to start with '98', got: %s", response)
	}

	// Clean up
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer stopCancel()
	srv.Stop(stopCtx)
}

// TestServerConnectionTracking tests connection metrics
func TestServerConnectionTracking(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create test configuration
	cfg := &config.Config{
		Port:            16668,
		OkapiURL:        "http://localhost:9130",
		LogLevel:        "info",
		HealthCheckPort: 18083,
		Tenants: map[string]*config.TenantConfig{
			"test-tenant": {
				Tenant:           "test-tenant",
				MessageDelimiter: "\r",
				FieldDelimiter:   "|",
				OkapiURL:         "http://localhost:9130",
				Charset:          "UTF-8",
			},
		},
	}

	// Create logger
	logger, err := logging.NewLogger("info", "")
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Sync()

	// Create and start server
	srv, err := server.NewServer(cfg, logger)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	srv.RegisterAllHandlers()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	go func() {
		srv.Start(ctx)
	}()

	time.Sleep(100 * time.Millisecond)

	// Initial connection count should be 0
	if srv.GetActiveConnections() != 0 {
		t.Errorf("Expected 0 active connections, got %d", srv.GetActiveConnections())
	}

	// Create multiple connections
	connections := make([]net.Conn, 3)
	for i := 0; i < 3; i++ {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", cfg.Port), 2*time.Second)
		if err != nil {
			t.Fatalf("Failed to connect: %v", err)
		}
		connections[i] = conn
		time.Sleep(50 * time.Millisecond)
	}

	// Should have 3 active connections
	time.Sleep(100 * time.Millisecond)
	activeConns := srv.GetActiveConnections()
	if activeConns != 3 {
		t.Errorf("Expected 3 active connections, got %d", activeConns)
	}

	// Total connections should be at least 3
	totalConns := srv.GetTotalConnections()
	if totalConns < 3 {
		t.Errorf("Expected at least 3 total connections, got %d", totalConns)
	}

	// Close all connections
	for _, conn := range connections {
		conn.Close()
	}

	// Wait for connections to be cleaned up
	time.Sleep(200 * time.Millisecond)

	// Active connections should be 0
	activeConns = srv.GetActiveConnections()
	if activeConns != 0 {
		t.Errorf("Expected 0 active connections after closing, got %d", activeConns)
	}

	// Clean up
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer stopCancel()
	srv.Stop(stopCtx)
}
