package folio

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

// Tests for HTTPError methods

func TestHTTPError_IsNotFound(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		expected   bool
	}{
		{"404 is not found", 404, true},
		{"200 is not not-found", 200, false},
		{"401 is not not-found", 401, false},
		{"500 is not not-found", 500, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &HTTPError{StatusCode: tt.statusCode}
			if err.IsNotFound() != tt.expected {
				t.Errorf("IsNotFound() = %v, want %v", err.IsNotFound(), tt.expected)
			}
		})
	}
}

func TestHTTPError_IsBadRequest(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		expected   bool
	}{
		{"400 is bad request", 400, true},
		{"200 is not bad request", 200, false},
		{"404 is not bad request", 404, false},
		{"500 is not bad request", 500, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &HTTPError{StatusCode: tt.statusCode}
			if err.IsBadRequest() != tt.expected {
				t.Errorf("IsBadRequest() = %v, want %v", err.IsBadRequest(), tt.expected)
			}
		})
	}
}

func TestHTTPError_IsServerError(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		expected   bool
	}{
		{"500 is server error", 500, true},
		{"503 is server error", 503, true},
		{"599 is server error", 599, true},
		{"200 is not server error", 200, false},
		{"404 is not server error", 404, false},
		{"400 is not server error", 400, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &HTTPError{StatusCode: tt.statusCode}
			if err.IsServerError() != tt.expected {
				t.Errorf("IsServerError() = %v, want %v", err.IsServerError(), tt.expected)
			}
		})
	}
}

func TestHTTPError_ParseErrorMessage_EmptyBody(t *testing.T) {
	err := &HTTPError{StatusCode: 500, Body: ""}
	msg := err.ParseErrorMessage()
	if msg != "Unknown error" {
		t.Errorf("Expected 'Unknown error', got %q", msg)
	}
}

func TestHTTPError_ParseErrorMessage_FolioFormat(t *testing.T) {
	body := `{"message": "Item not found", "type": "error", "code": "NOT_FOUND"}`
	err := &HTTPError{StatusCode: 404, Body: body}
	msg := err.ParseErrorMessage()
	if msg != "Item not found" {
		t.Errorf("Expected 'Item not found', got %q", msg)
	}
}

func TestHTTPError_ParseErrorMessage_ErrorsArrayFormat(t *testing.T) {
	body := `{"errors": [{"message": "Validation failed", "type": "error"}]}`
	err := &HTTPError{StatusCode: 422, Body: body}
	msg := err.ParseErrorMessage()
	if msg != "Validation failed" {
		t.Errorf("Expected 'Validation failed', got %q", msg)
	}
}

func TestHTTPError_ParseErrorMessage_RawBody(t *testing.T) {
	body := "Internal server error occurred"
	err := &HTTPError{StatusCode: 500, Body: body}
	msg := err.ParseErrorMessage()
	if msg != body {
		t.Errorf("Expected raw body %q, got %q", body, msg)
	}
}

func TestHTTPError_ParseErrorMessage_LongBodyTruncated(t *testing.T) {
	// Create a body longer than 200 characters
	longBody := ""
	for i := 0; i < 250; i++ {
		longBody += "x"
	}
	err := &HTTPError{StatusCode: 500, Body: longBody}
	msg := err.ParseErrorMessage()
	if len(msg) != 203 { // 200 chars + "..."
		t.Errorf("Expected truncated message of length 203, got %d: %q", len(msg), msg)
	}
}

func TestHTTPError_ParseErrorMessage_InvalidJSON(t *testing.T) {
	body := "not json at all"
	err := &HTTPError{StatusCode: 500, Body: body}
	msg := err.ParseErrorMessage()
	if msg != body {
		t.Errorf("Expected raw body for invalid JSON, got %q", msg)
	}
}

// Tests for Client methods

func TestClient_SetTimeout(t *testing.T) {
	client := NewClient("http://example.com", "test-tenant")
	// Initial timeout should be 30 seconds
	if client.timeout != 30*time.Second {
		t.Errorf("Expected initial timeout 30s, got %v", client.timeout)
	}

	// Set custom timeout
	client.SetTimeout(5 * time.Second)
	if client.timeout != 5*time.Second {
		t.Errorf("Expected timeout 5s after SetTimeout, got %v", client.timeout)
	}

	// Set zero timeout
	client.SetTimeout(0)
	if client.timeout != 0 {
		t.Errorf("Expected timeout 0 after SetTimeout(0), got %v", client.timeout)
	}
}

func TestClient_Delete_Success(t *testing.T) {
	deleteCalled := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			deleteCalled = true
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusMethodNotAllowed)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-tenant")
	err := client.Delete(context.Background(), "/some/resource/123", "test-token")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if !deleteCalled {
		t.Error("Expected DELETE request to be made")
	}
}

func TestClient_Delete_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "not found"})
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-tenant")
	err := client.Delete(context.Background(), "/some/resource/nonexistent", "test-token")
	if err == nil {
		t.Error("Expected error for 404 response")
	}

	if httpErr, ok := err.(*HTTPError); ok {
		if !httpErr.IsNotFound() {
			t.Errorf("Expected IsNotFound to be true, status=%d", httpErr.StatusCode)
		}
	}
}

func TestClient_PostWithTextPlainAccept_Success(t *testing.T) {
	acceptHeader := ""

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		acceptHeader = r.Header.Get("Accept")
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusMethodNotAllowed)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-tenant")
	body := map[string]string{"key": "value"}
	err := client.PostWithTextPlainAccept(context.Background(), "/test/endpoint", "token", body)
	if err != nil {
		t.Fatalf("PostWithTextPlainAccept failed: %v", err)
	}
	if acceptHeader != "text/plain" {
		t.Errorf("Expected Accept: text/plain, got %q", acceptHeader)
	}
}

func TestClient_PostWithTextPlainAccept_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("bad request"))
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-tenant")
	err := client.PostWithTextPlainAccept(context.Background(), "/test/endpoint", "token", nil)
	if err == nil {
		t.Error("Expected error for 400 response")
	}
	if httpErr, ok := err.(*HTTPError); ok {
		if !httpErr.IsBadRequest() {
			t.Errorf("Expected IsBadRequest to be true, status=%d", httpErr.StatusCode)
		}
	}
}

func TestHTTPError_Error(t *testing.T) {
	err := &HTTPError{
		StatusCode: 404,
		Status:     "404 Not Found",
		Body:       "not found",
		URL:        "http://example.com/test",
		Method:     "GET",
	}
	msg := err.Error()
	if msg == "" {
		t.Error("Expected non-empty error message")
	}
}

// newObservedClient returns a Client whose logger is backed by an observer core,
// allowing tests to inspect emitted log entries.
func newObservedClient(baseURL string, logger *zap.Logger) *Client {
	initSharedHTTPClient()
	return &Client{
		httpClient: sharedHTTPClient,
		baseURL:    baseURL,
		tenant:     "test-tenant",
		timeout:    0,
		logger:     logger,
	}
}

func TestClient_DebugLogs_SuccessfulRequest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer srv.Close()

	core, recorded := observer.New(zap.DebugLevel)
	client := newObservedClient(srv.URL, zap.New(core))

	var result map[string]string
	err := client.Get(context.Background(), "/test", "tok", &result)
	require.NoError(t, err)

	entries := recorded.All()
	require.Len(t, entries, 2, "expected folio_request and folio_response debug entries")

	reqEntry := entries[0]
	assert.Equal(t, zap.DebugLevel, reqEntry.Level)
	assert.Equal(t, "FOLIO API request", reqEntry.Message)
	assert.Equal(t, "folio_request", reqEntry.ContextMap()["type"])
	assert.Equal(t, "GET", reqEntry.ContextMap()["method"])
	assert.Equal(t, "test-tenant", reqEntry.ContextMap()["tenant"])

	respEntry := entries[1]
	assert.Equal(t, zap.DebugLevel, respEntry.Level)
	assert.Equal(t, "FOLIO API response", respEntry.Message)
	assert.Equal(t, "folio_response", respEntry.ContextMap()["type"])
	assert.Equal(t, "GET", respEntry.ContextMap()["method"])
	assert.EqualValues(t, http.StatusOK, respEntry.ContextMap()["status_code"])
}

func TestClient_DebugLogs_HTTPErrorResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer srv.Close()

	core, recorded := observer.New(zap.DebugLevel)
	client := newObservedClient(srv.URL, zap.New(core))

	err := client.Get(context.Background(), "/missing", "tok", nil)
	require.Error(t, err)

	entries := recorded.All()
	require.Len(t, entries, 2, "expected folio_request and folio_response debug entries on error")

	assert.Equal(t, "folio_request", entries[0].ContextMap()["type"])

	respEntry := entries[1]
	assert.Equal(t, "FOLIO API response", respEntry.Message)
	assert.Equal(t, "folio_response", respEntry.ContextMap()["type"])
	assert.EqualValues(t, http.StatusNotFound, respEntry.ContextMap()["status_code"])
}

func TestClient_DebugLogs_NilLoggerNoPanic(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	// nil logger — must not panic
	client := newObservedClient(srv.URL, nil)
	assert.NotPanics(t, func() {
		_ = client.Get(context.Background(), "/test", "tok", nil)
	})
}

func TestSetClientLogger_SetsPackageVar(t *testing.T) {
	original := clientLogger
	defer func() { clientLogger = original }()

	core, _ := observer.New(zap.DebugLevel)
	logger := zap.New(core)
	SetClientLogger(logger)
	assert.Equal(t, logger, clientLogger)
}
