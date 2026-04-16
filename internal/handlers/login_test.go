package handlers

import (
	"context"
	"testing"
	"time"

	"github.com/spokanepubliclibrary/fsip2/internal/config"
	"github.com/spokanepubliclibrary/fsip2/internal/sip2/parser"
	"github.com/spokanepubliclibrary/fsip2/internal/types"
	"go.uber.org/zap"
)

func TestLoginHandler_Handle_Success(t *testing.T) {
	// Create test configuration
	tenantConfig := &config.TenantConfig{
		Tenant:           "test-tenant",
		MessageDelimiter: "\r",
		FieldDelimiter:   "|",
		OkapiURL:         "http://localhost:9130",
	}

	// Create logger
	logger := zap.NewNop()

	// Create handler
	handler := NewLoginHandler(logger, tenantConfig)

	// Create session
	session := types.NewSession("test-session", tenantConfig)

	// Create login request message
	msg := &parser.Message{
		Code:   parser.LoginRequest,
		Fields: make(map[string]string),
	}
	msg.Fields[string(parser.LoginUserID)] = "testuser"
	msg.Fields[string(parser.LoginPassword)] = "testpass"

	// Note: This will fail without a real FOLIO server
	// In a real test, you'd mock the FOLIO client
	ctx := context.Background()
	response, _ := handler.Handle(ctx, msg, session)

	// Should return a login response even on failure
	if response == "" {
		t.Error("Expected non-empty response")
	}

	// Response should start with "94"
	if len(response) < 2 || response[:2] != "94" {
		t.Errorf("Expected response to start with '94', got: %s", response)
	}
}

func TestLoginHandler_Handle_MissingUsername(t *testing.T) {
	tenantConfig := &config.TenantConfig{
		Tenant:           "test-tenant",
		MessageDelimiter: "\r",
		FieldDelimiter:   "|",
	}

	logger := zap.NewNop()
	handler := NewLoginHandler(logger, tenantConfig)
	session := types.NewSession("test-session", tenantConfig)

	// Create message without username
	msg := &parser.Message{
		Code:   parser.LoginRequest,
		Fields: make(map[string]string),
	}
	msg.Fields[string(parser.LoginPassword)] = "testpass"

	ctx := context.Background()
	response, _ := handler.Handle(ctx, msg, session)

	// Should return login failed response
	if len(response) < 2 || response[:2] != "94" {
		t.Errorf("Expected login response, got: %s", response)
	}

	// Should indicate failure (940...)
	if len(response) >= 3 && response[2] != '0' {
		t.Errorf("Expected login failure (0), got: %s", response)
	}
}

// TestLogin_SkipsFOLIO_WhenSessionTokenValid verifies the early-exit path: when the
// session already holds a valid, non-expired token the handler must return a success
// response (ok-bit == '1') without ever reaching the FOLIO network call.
func TestLogin_SkipsFOLIO_WhenSessionTokenValid(t *testing.T) {
	tenantConfig := &config.TenantConfig{
		Tenant:           "test-tenant",
		MessageDelimiter: "\r",
		FieldDelimiter:   "|",
		OkapiURL:         "http://127.0.0.1:0", // unreachable — proves no HTTP call was made
		OkapiTenant:      "test-tenant",
	}

	logger := zap.NewNop()
	handler := NewLoginHandler(logger, tenantConfig)
	session := types.NewSession("test-session", tenantConfig)

	// Seed a valid token that expires well beyond the 90 s buffer.
	session.SetAuthenticated("testuser", "", "", "valid-token", time.Now().Add(10*time.Minute))

	msg := &parser.Message{
		Code:   parser.LoginRequest,
		Fields: make(map[string]string),
	}
	msg.Fields[string(parser.LoginUserID)] = "testuser"
	msg.Fields[string(parser.LoginPassword)] = "testpass"

	ctx := context.Background()
	response, err := handler.Handle(ctx, msg, session)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(response) < 3 {
		t.Fatalf("response too short: %q", response)
	}
	if response[:2] != "94" {
		t.Errorf("expected response to start with '94', got: %s", response)
	}
	// ok-bit must be '1': early-exit returned success without calling FOLIO.
	if response[2] != '1' {
		t.Errorf("expected ok-bit '1' (FOLIO skipped, session valid), got: %c in %s", response[2], response)
	}
}

// TestLogin_CallsFOLIO_WhenTokenExpired verifies that a session with an expired token
// does NOT trigger the early-exit and instead attempts the FOLIO call.  With no real
// FOLIO server available the call fails and the ok-bit must be '0'.
func TestLogin_CallsFOLIO_WhenTokenExpired(t *testing.T) {
	tenantConfig := &config.TenantConfig{
		Tenant:           "test-tenant",
		MessageDelimiter: "\r",
		FieldDelimiter:   "|",
		OkapiURL:         "http://127.0.0.1:0", // unreachable — FOLIO call will fail
		OkapiTenant:      "test-tenant",
	}

	logger := zap.NewNop()
	handler := NewLoginHandler(logger, tenantConfig)
	session := types.NewSession("test-session", tenantConfig)

	// Seed a token that is already expired (in the past, well inside the 90 s buffer).
	session.SetAuthenticated("testuser", "", "", "old-token", time.Now().Add(-5*time.Minute))

	msg := &parser.Message{
		Code:   parser.LoginRequest,
		Fields: make(map[string]string),
	}
	msg.Fields[string(parser.LoginUserID)] = "testuser"
	msg.Fields[string(parser.LoginPassword)] = "testpass"

	ctx := context.Background()
	response, err := handler.Handle(ctx, msg, session)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(response) < 3 {
		t.Fatalf("response too short: %q", response)
	}
	if response[:2] != "94" {
		t.Errorf("expected response to start with '94', got: %s", response)
	}
	// ok-bit must be '0': FOLIO was attempted and failed (unreachable server).
	if response[2] != '0' {
		t.Errorf("expected ok-bit '0' (FOLIO called, auth failed), got: %c in %s", response[2], response)
	}
}

// TestLogin_CallsFOLIO_WhenNotAuthenticated verifies that an unauthenticated session
// does NOT trigger the early-exit and instead attempts the FOLIO call.  With no real
// FOLIO server available the call fails and the ok-bit must be '0'.
func TestLogin_CallsFOLIO_WhenNotAuthenticated(t *testing.T) {
	tenantConfig := &config.TenantConfig{
		Tenant:           "test-tenant",
		MessageDelimiter: "\r",
		FieldDelimiter:   "|",
		OkapiURL:         "http://127.0.0.1:0", // unreachable — FOLIO call will fail
		OkapiTenant:      "test-tenant",
	}

	logger := zap.NewNop()
	handler := NewLoginHandler(logger, tenantConfig)
	// NewSession leaves IsAuthenticated = false — no SetAuthenticated call needed.
	session := types.NewSession("test-session", tenantConfig)

	msg := &parser.Message{
		Code:   parser.LoginRequest,
		Fields: make(map[string]string),
	}
	msg.Fields[string(parser.LoginUserID)] = "testuser"
	msg.Fields[string(parser.LoginPassword)] = "testpass"

	ctx := context.Background()
	response, err := handler.Handle(ctx, msg, session)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(response) < 3 {
		t.Fatalf("response too short: %q", response)
	}
	if response[:2] != "94" {
		t.Errorf("expected response to start with '94', got: %s", response)
	}
	// ok-bit must be '0': FOLIO was attempted and failed (unreachable server).
	if response[2] != '0' {
		t.Errorf("expected ok-bit '0' (FOLIO called, auth failed), got: %c in %s", response[2], response)
	}
}

func TestLoginHandler_BuildLoginResponse(t *testing.T) {
	tenantConfig := &config.TenantConfig{
		Tenant:           "test-tenant",
		MessageDelimiter: "\r",
		FieldDelimiter:   "|",
	}

	logger := zap.NewNop()
	handler := NewLoginHandler(logger, tenantConfig)
	session := types.NewSession("test-session", tenantConfig)

	tests := []struct {
		name           string
		ok             bool
		sequenceNumber string
	}{
		{"Success", true, "0"},
		{"Failure", false, "1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := handler.buildLoginResponse(tt.ok, tt.sequenceNumber, session)
			// Response should start with "94"
			if len(response) < 2 || response[:2] != "94" {
				t.Errorf("Expected response to start with '94', got: %s", response)
			}
			// Check ok status
			if len(response) >= 3 {
				if tt.ok && response[2] != '1' {
					t.Errorf("Expected success (1), got: %c", response[2])
				} else if !tt.ok && response[2] != '0' {
					t.Errorf("Expected failure (0), got: %c", response[2])
				}
			}
		})
	}
}
