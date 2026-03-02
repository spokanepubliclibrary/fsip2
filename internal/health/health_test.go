package health

import (
	"context"
	"io"
	"net/http"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestNewServer(t *testing.T) {
	logger := zap.NewNop()
	port := 18080

	server := NewServer(port, logger)

	if server == nil {
		t.Fatal("Expected server to be created, got nil")
	}

	if server.port != port {
		t.Errorf("Expected port %d, got %d", port, server.port)
	}

	if server.logger == nil {
		t.Error("Expected logger to be set")
	}
}

func TestHealthHandler(t *testing.T) {
	logger := zap.NewNop()
	server := NewServer(18081, logger)

	// Create a test HTTP request
	req, err := http.NewRequest("GET", "/admin/health", nil)
	if err != nil {
		t.Fatal(err)
	}

	// Create a response recorder
	recorder := &testResponseWriter{
		headers:    make(http.Header),
		statusCode: 0,
		body:       []byte{},
	}

	// Call the handler
	server.healthHandler(recorder, req)

	// Check status code
	if recorder.statusCode != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, recorder.statusCode)
	}

	// Check response body
	if string(recorder.body) != "OK" {
		t.Errorf("Expected body 'OK', got %s", string(recorder.body))
	}
}

func TestReadyHandler(t *testing.T) {
	logger := zap.NewNop()
	server := NewServer(18082, logger)

	// Create a test HTTP request
	req, err := http.NewRequest("GET", "/admin/ready", nil)
	if err != nil {
		t.Fatal(err)
	}

	// Create a response recorder
	recorder := &testResponseWriter{
		headers:    make(http.Header),
		statusCode: 0,
		body:       []byte{},
	}

	// Call the handler
	server.readyHandler(recorder, req)

	// Check status code
	if recorder.statusCode != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, recorder.statusCode)
	}

	// Check response body
	if string(recorder.body) != "Ready" {
		t.Errorf("Expected body 'Ready', got %s", string(recorder.body))
	}
}

func TestServerStartStop(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping server start/stop test in short mode")
	}

	logger := zap.NewNop()
	server := NewServerWithHost("127.0.0.1", 18083, logger)

	// Start server in background
	ctx := context.Background()
	errChan := make(chan error, 1)

	go func() {
		errChan <- server.Start(ctx)
	}()

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)

	// Test health endpoint
	resp, err := http.Get("http://127.0.0.1:18083/admin/health")
	if err != nil {
		t.Errorf("Failed to connect to health endpoint: %v", err)
	} else {
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status %d, got %d", http.StatusOK, resp.StatusCode)
		}

		body, _ := io.ReadAll(resp.Body)
		if string(body) != "OK" {
			t.Errorf("Expected body 'OK', got %s", string(body))
		}
	}

	// Test ready endpoint
	resp, err = http.Get("http://127.0.0.1:18083/admin/ready")
	if err != nil {
		t.Errorf("Failed to connect to ready endpoint: %v", err)
	} else {
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status %d, got %d", http.StatusOK, resp.StatusCode)
		}

		body, _ := io.ReadAll(resp.Body)
		if string(body) != "Ready" {
			t.Errorf("Expected body 'Ready', got %s", string(body))
		}
	}

	// Stop server
	stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Stop(stopCtx); err != nil {
		t.Errorf("Failed to stop server: %v", err)
	}
}

func TestStopWithoutStart(t *testing.T) {
	logger := zap.NewNop()
	server := NewServer(18084, logger)

	ctx := context.Background()

	// Should not error when stopping a server that hasn't started
	if err := server.Stop(ctx); err != nil {
		t.Errorf("Expected no error when stopping unstarted server, got: %v", err)
	}
}

// testResponseWriter is a simple implementation of http.ResponseWriter for testing
type testResponseWriter struct {
	headers    http.Header
	body       []byte
	statusCode int
}

func (w *testResponseWriter) Header() http.Header {
	return w.headers
}

func (w *testResponseWriter) Write(data []byte) (int, error) {
	w.body = append(w.body, data...)
	if w.statusCode == 0 {
		w.statusCode = http.StatusOK
	}
	return len(data), nil
}

func (w *testResponseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
}
