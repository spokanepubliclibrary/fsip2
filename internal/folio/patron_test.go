package folio

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/spokanepubliclibrary/fsip2/internal/folio/models"
)

func TestUpdateUserExpiration_Success(t *testing.T) {
	// Create test user
	expirationDate := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	testUser := &models.User{
		ID:       "user-123",
		Username: "testuser",
		Barcode:  "123456",
		Active:   true,
		Personal: models.PersonalInfo{
			FirstName: "Test",
			LastName:  "User",
		},
		ExpirationDate: &expirationDate,
	}

	// Track whether PUT was called
	putCalled := false
	var capturedUser models.User

	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Handle GET /users/{id}
		if r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/users/") {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(testUser)
			return
		}

		// Handle PUT /users/{id}
		if r.Method == http.MethodPut && strings.HasPrefix(r.URL.Path, "/users/") {
			putCalled = true

			// Verify Accept header is text/plain (FOLIO requirement)
			acceptHeader := r.Header.Get("Accept")
			if acceptHeader != "text/plain" {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte(`Accept header must be ["text/plain"] for this request`))
				return
			}

			// Decode the request body to verify the update
			if err := json.NewDecoder(r.Body).Decode(&capturedUser); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			w.WriteHeader(http.StatusNoContent)
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	// Create patron client
	client := NewPatronClient(server.URL, "test-tenant")
	ctx := context.Background()

	// Test updating expiration date
	newExpiration := "2026-06-01T00:00:00.000+00:00"
	err := client.UpdateUserExpiration(ctx, "test-token", "user-123", newExpiration, false)

	if err != nil {
		t.Fatalf("UpdateUserExpiration failed: %v", err)
	}

	if !putCalled {
		t.Error("Expected PUT request to be made")
	}

	// Verify the expiration date was updated
	if capturedUser.ExpirationDate == nil {
		t.Fatal("Expected expiration date to be set")
	}

	expectedDate := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	if !capturedUser.ExpirationDate.Equal(expectedDate) {
		t.Errorf("Expected expiration date %v, got %v", expectedDate, capturedUser.ExpirationDate)
	}

	// Verify other fields were preserved
	if capturedUser.Username != testUser.Username {
		t.Errorf("Username was not preserved: expected %s, got %s", testUser.Username, capturedUser.Username)
	}
}

func TestUpdateUserExpiration_UserNotFound(t *testing.T) {
	// Create mock server that returns 404
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "User not found"})
	}))
	defer server.Close()

	client := NewPatronClient(server.URL, "test-tenant")
	ctx := context.Background()

	err := client.UpdateUserExpiration(ctx, "test-token", "nonexistent-user", "2026-01-01T00:00:00.000+00:00", false)

	if err == nil {
		t.Error("Expected error when user not found")
	}

	if !strings.Contains(err.Error(), "failed to fetch user") {
		t.Errorf("Expected 'failed to fetch user' error, got: %v", err)
	}
}

func TestUpdateUserExpiration_PermissionDenied(t *testing.T) {
	// Create test user
	expirationDate := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	testUser := &models.User{
		ID:             "user-123",
		ExpirationDate: &expirationDate,
	}

	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Handle GET - success
		if r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/users/") {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(testUser)
			return
		}

		// Handle PUT - forbidden
		if r.Method == http.MethodPut && strings.HasPrefix(r.URL.Path, "/users/") {
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(map[string]string{
				"message": "Access requires permission: users.item.put",
				"type":    "error",
			})
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := NewPatronClient(server.URL, "test-tenant")
	ctx := context.Background()

	err := client.UpdateUserExpiration(ctx, "test-token", "user-123", "2026-01-01T00:00:00.000+00:00", false)

	if err == nil {
		t.Fatal("Expected permission error")
	}

	// Check if it's a PermissionError
	if !IsPermissionError(err) {
		t.Errorf("Expected PermissionError, got: %T - %v", err, err)
	}

	if !strings.Contains(err.Error(), "permission denied") {
		t.Errorf("Expected 'permission denied' in error message, got: %v", err)
	}
}

func TestUpdateUserExpiration_InvalidDateFormat(t *testing.T) {
	// Create test user
	expirationDate := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	testUser := &models.User{
		ID:             "user-123",
		ExpirationDate: &expirationDate,
	}

	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/users/") {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(testUser)
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := NewPatronClient(server.URL, "test-tenant")
	ctx := context.Background()

	// Test with invalid date format
	err := client.UpdateUserExpiration(ctx, "test-token", "user-123", "invalid-date-format", false)

	if err == nil {
		t.Fatal("Expected error for invalid date format")
	}

	if !strings.Contains(err.Error(), "failed to parse expiration date") {
		t.Errorf("Expected parse error, got: %v", err)
	}
}

func TestUpdateUserExpiration_ServerError(t *testing.T) {
	// Create test user
	expirationDate := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	testUser := &models.User{
		ID:             "user-123",
		ExpirationDate: &expirationDate,
	}

	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Handle GET - success
		if r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/users/") {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(testUser)
			return
		}

		// Handle PUT - server error
		if r.Method == http.MethodPut && strings.HasPrefix(r.URL.Path, "/users/") {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Internal server error"})
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := NewPatronClient(server.URL, "test-tenant")
	ctx := context.Background()

	err := client.UpdateUserExpiration(ctx, "test-token", "user-123", "2026-01-01T00:00:00.000+00:00", false)

	if err == nil {
		t.Fatal("Expected error for server error")
	}

	if !strings.Contains(err.Error(), "failed to update user expiration") {
		t.Errorf("Expected update error, got: %v", err)
	}
}

func TestParseExpirationDate(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectError bool
		expectedDay int
	}{
		{
			name:        "FOLIO format with timezone offset",
			input:       "2026-06-15T00:00:00.000+00:00",
			expectError: false,
			expectedDay: 15,
		},
		{
			name:        "FOLIO format with Z",
			input:       "2026-06-15T00:00:00.000Z",
			expectError: false,
			expectedDay: 15,
		},
		{
			name:        "ISO 8601 with timezone",
			input:       "2026-06-15T12:30:45-05:00",
			expectError: false,
			expectedDay: 15,
		},
		{
			name:        "ISO 8601 with Z",
			input:       "2026-06-15T12:30:45Z",
			expectError: false,
			expectedDay: 15,
		},
		{
			name:        "RFC3339 format",
			input:       "2026-06-15T00:00:00Z",
			expectError: false,
			expectedDay: 15,
		},
		{
			name:        "Invalid format",
			input:       "15-06-2026",
			expectError: true,
		},
		{
			name:        "Empty string",
			input:       "",
			expectError: true,
		},
		{
			name:        "Random text",
			input:       "not a date",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseExpirationDate(tt.input)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if result.Day() != tt.expectedDay {
					t.Errorf("Expected day %d, got %d", tt.expectedDay, result.Day())
				}
			}
		})
	}
}

func TestUpdateUserExpiration_PreservesExactJSONStructure(t *testing.T) {
	// This test verifies that UpdateUserExpiration preserves the exact JSON structure
	// from the GET response and doesn't add missing fields with default values

	// Create a minimal user JSON response (missing many fields like "active", "type", etc.)
	minimalUserJSON := `{
		"id": "user-456",
		"username": "testuser",
		"barcode": "123456",
		"expirationDate": "2025-06-01T00:00:00.000+00:00",
		"personal": {
			"lastName": "Doe",
			"firstName": "John"
		}
	}`

	var capturedPUTBody []byte
	putCalled := false

	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Handle GET /users/{id} - return minimal user
		if r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/users/") {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(minimalUserJSON))
			return
		}

		// Handle PUT /users/{id} - capture raw body
		if r.Method == http.MethodPut && strings.HasPrefix(r.URL.Path, "/users/") {
			putCalled = true

			// Verify Accept header
			acceptHeader := r.Header.Get("Accept")
			if acceptHeader != "text/plain" {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte(`Accept header must be ["text/plain"] for this request`))
				return
			}

			// Capture the raw PUT body
			bodyBytes, err := io.ReadAll(r.Body)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			capturedPUTBody = bodyBytes

			w.WriteHeader(http.StatusNoContent)
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	// Create patron client
	client := NewPatronClient(server.URL, "test-tenant")
	ctx := context.Background()

	// Test updating expiration date (without reactivation)
	newExpiration := "2026-12-31T00:00:00.000+00:00"
	err := client.UpdateUserExpiration(ctx, "test-token", "user-456", newExpiration, false)

	if err != nil {
		t.Fatalf("UpdateUserExpiration failed: %v", err)
	}

	if !putCalled {
		t.Fatal("Expected PUT request to be made")
	}

	// Parse the captured PUT body to verify structure
	var putData map[string]interface{}
	if err := json.Unmarshal(capturedPUTBody, &putData); err != nil {
		t.Fatalf("Failed to parse PUT body: %v", err)
	}

	// Verify that only expected fields are present
	expectedFields := []string{"id", "username", "barcode", "expirationDate", "personal"}
	for _, field := range expectedFields {
		if _, exists := putData[field]; !exists {
			t.Errorf("Expected field %s to be present in PUT body", field)
		}
	}

	// Verify that fields NOT in the GET response are NOT added to PUT
	// These fields have default values in Go structs but should NOT be in JSON
	unexpectedFields := []string{"active", "type", "enrollmentDate", "departments", "proxyFor"}
	for _, field := range unexpectedFields {
		if _, exists := putData[field]; exists {
			t.Errorf("Field %s should NOT be present in PUT body (was not in GET response), but it was added with value: %v", field, putData[field])
		}
	}

	// Verify expirationDate was updated correctly
	if expirationDate, ok := putData["expirationDate"].(string); !ok {
		t.Error("expirationDate should be a string")
	} else if !strings.HasPrefix(expirationDate, "2026-12-31") {
		t.Errorf("Expected expirationDate to start with '2026-12-31', got: %s", expirationDate)
	}

	// Verify the total number of top-level fields matches what we expect (5 fields)
	if len(putData) != 5 {
		t.Errorf("Expected exactly 5 top-level fields in PUT body, got %d: %v", len(putData), putData)
	}
}

func TestUpdateUserExpiration_WithReactivation(t *testing.T) {
	// This test verifies that when reactivate=true, the active field is set to true
	// This is used for rolling renewals with extendExpired=true

	// Create an inactive user
	expirationDate := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC) // Expired
	testUser := &models.User{
		ID:             "user-789",
		Username:       "inactiveuser",
		Barcode:        "789012",
		Active:         false, // Inactive
		ExpirationDate: &expirationDate,
		Personal: models.PersonalInfo{
			FirstName: "Inactive",
			LastName:  "User",
		},
	}

	var capturedPUTBody []byte
	putCalled := false

	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Handle GET /users/{id}
		if r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/users/") {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(testUser)
			return
		}

		// Handle PUT /users/{id}
		if r.Method == http.MethodPut && strings.HasPrefix(r.URL.Path, "/users/") {
			putCalled = true

			// Verify Accept header
			acceptHeader := r.Header.Get("Accept")
			if acceptHeader != "text/plain" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			// Capture the raw PUT body
			bodyBytes, err := io.ReadAll(r.Body)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			capturedPUTBody = bodyBytes

			w.WriteHeader(http.StatusNoContent)
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	// Create patron client
	client := NewPatronClient(server.URL, "test-tenant")
	ctx := context.Background()

	// Test updating expiration date with reactivation
	newExpiration := "2026-12-31T00:00:00.000+00:00"
	err := client.UpdateUserExpiration(ctx, "test-token", "user-789", newExpiration, true)

	if err != nil {
		t.Fatalf("UpdateUserExpiration failed: %v", err)
	}

	if !putCalled {
		t.Fatal("Expected PUT request to be made")
	}

	// Parse the captured PUT body
	var putData map[string]interface{}
	if err := json.Unmarshal(capturedPUTBody, &putData); err != nil {
		t.Fatalf("Failed to parse PUT body: %v", err)
	}

	// Verify active field is set to true
	activeValue, exists := putData["active"]
	if !exists {
		t.Fatal("Expected 'active' field to be present in PUT body when reactivate=true")
	}

	activeBool, ok := activeValue.(bool)
	if !ok {
		t.Fatalf("Expected 'active' to be a boolean, got %T", activeValue)
	}

	if !activeBool {
		t.Error("Expected 'active' to be true when reactivate=true, got false")
	}

	// Verify expirationDate was updated
	expirationValue, exists := putData["expirationDate"]
	if !exists {
		t.Fatal("Expected 'expirationDate' field to be present in PUT body")
	}

	expirationStr, ok := expirationValue.(string)
	if !ok {
		t.Fatalf("Expected 'expirationDate' to be a string, got %T", expirationValue)
	}

	if !strings.HasPrefix(expirationStr, "2026-12-31") {
		t.Errorf("Expected expirationDate to start with '2026-12-31', got: %s", expirationStr)
	}
}

func TestIsPermissionError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name: "PermissionError",
			err: &PermissionError{
				Operation: "test",
				UserID:    "user-1",
				Err:       &HTTPError{StatusCode: 403},
			},
			expected: true,
		},
		{
			name:     "Regular error",
			err:      &HTTPError{StatusCode: 404},
			expected: false,
		},
		{
			name:     "Nil error",
			err:      nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsPermissionError(tt.err)
			if result != tt.expected {
				t.Errorf("IsPermissionError() = %v, expected %v", result, tt.expected)
			}
		})
	}
}
