package handlers

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/spokanepubliclibrary/fsip2/internal/config"
	"github.com/spokanepubliclibrary/fsip2/internal/sip2/parser"
	"github.com/spokanepubliclibrary/fsip2/internal/types"
	"go.uber.org/zap"
)

func TestEndSessionHandler_Handle_Success(t *testing.T) {
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
	handler := NewEndSessionHandler(logger, tenantConfig)

	// Create session with authentication
	session := types.NewSession("test-session-123", tenantConfig)
	expiresAt := time.Now().Add(10 * time.Minute)
	session.SetAuthenticated("testuser", "patron-123", "barcode-456", "test-token", expiresAt)

	// Create end session request message
	msg := &parser.Message{
		Code:   parser.EndPatronSessionRequest,
		Fields: make(map[string]string),
	}
	msg.Fields[string(parser.InstitutionID)] = "TEST-INST"
	msg.Fields[string(parser.PatronIdentifier)] = "patron-barcode"

	ctx := context.Background()
	response, err := handler.Handle(ctx, msg, session)

	// Verify no error
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Verify response
	if response == "" {
		t.Error("Expected non-empty response")
	}

	// Response should start with "36" (End Session Response)
	if len(response) < 2 || response[:2] != "36" {
		t.Errorf("Expected response to start with '36', got: %s", response)
	}

	// Response should indicate success (Y)
	if len(response) < 3 || response[2:3] != "Y" {
		t.Errorf("Expected successful end session (Y), got: %s", response)
	}

	// Verify response contains institution ID
	if !strings.Contains(response, "AOTEST-INST") {
		t.Errorf("Response should contain institution ID, got: %s", response)
	}

	// Verify response contains patron identifier
	if !strings.Contains(response, "AApatron-barcode") {
		t.Errorf("Response should contain patron identifier, got: %s", response)
	}

	// Verify success message
	if !strings.Contains(response, "AFSession ended successfully") {
		t.Errorf("Response should contain success message, got: %s", response)
	}

	// Verify session was cleared
	if session.IsAuth() {
		t.Error("Expected session to be cleared after end session")
	}

	if session.GetAuthToken() != "" {
		t.Error("Expected auth token to be cleared")
	}

	if session.GetPatronID() != "" {
		t.Error("Expected patron ID to be cleared")
	}
}


func TestEndSessionHandler_Handle_MissingPatronIdentifier(t *testing.T) {
	tenantConfig := &config.TenantConfig{
		Tenant:           "test-tenant",
		MessageDelimiter: "\r",
		FieldDelimiter:   "|",
	}

	logger := zap.NewNop()
	handler := NewEndSessionHandler(logger, tenantConfig)
	session := types.NewSession("test-session", tenantConfig)

	// Create message without patron identifier
	msg := &parser.Message{
		Code:   parser.EndPatronSessionRequest,
		Fields: make(map[string]string),
	}
	msg.Fields[string(parser.InstitutionID)] = "TEST-INST"

	ctx := context.Background()
	response, err := handler.Handle(ctx, msg, session)

	// Should not error but return failure response
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Response should indicate failure
	if len(response) < 3 || response[2:3] != "N" {
		t.Errorf("Expected failed end session, got: %s", response)
	}
}

func TestEndSessionHandler_Handle_MissingAllFields(t *testing.T) {
	tenantConfig := &config.TenantConfig{
		Tenant:           "test-tenant",
		MessageDelimiter: "\r",
		FieldDelimiter:   "|",
	}

	logger := zap.NewNop()
	handler := NewEndSessionHandler(logger, tenantConfig)
	session := types.NewSession("test-session", tenantConfig)

	// Create message with no fields
	msg := &parser.Message{
		Code:   parser.EndPatronSessionRequest,
		Fields: make(map[string]string),
	}

	ctx := context.Background()
	response, err := handler.Handle(ctx, msg, session)

	// Should not error but return failure response
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Response should start with "36" and indicate failure
	if len(response) < 3 || response[:2] != "36" || response[2:3] != "N" {
		t.Errorf("Expected failed end session response (36N), got: %s", response)
	}
}

func TestEndSessionHandler_BuildEndSessionResponse_Success(t *testing.T) {
	tenantConfig := &config.TenantConfig{
		Tenant:           "test-tenant",
		MessageDelimiter: "\r",
		FieldDelimiter:   "|",
	}

	logger := zap.NewNop()
	handler := NewEndSessionHandler(logger, tenantConfig)

	msg := &parser.Message{
		Code:   parser.EndPatronSessionRequest,
		Fields: make(map[string]string),
	}

	response := handler.buildEndSessionResponse(true, "TEST-INST", "patron-123", msg)

	// Verify response format
	if !strings.HasPrefix(response, "36Y") {
		t.Errorf("Expected response to start with '36Y', got: %s", response)
	}

	// Verify contains required fields
	if !strings.Contains(response, "AOTEST-INST") {
		t.Error("Response should contain institution ID field")
	}

	if !strings.Contains(response, "AApatron-123") {
		t.Error("Response should contain patron identifier field")
	}

	if !strings.Contains(response, "AFSession ended successfully") {
		t.Error("Response should contain success message")
	}
}

func TestEndSessionHandler_BuildEndSessionResponse_Failure(t *testing.T) {
	tenantConfig := &config.TenantConfig{
		Tenant:           "test-tenant",
		MessageDelimiter: "\r",
		FieldDelimiter:   "|",
	}

	logger := zap.NewNop()
	handler := NewEndSessionHandler(logger, tenantConfig)

	msg := &parser.Message{
		Code:   parser.EndPatronSessionRequest,
		Fields: make(map[string]string),
	}

	response := handler.buildEndSessionResponse(false, "TEST-INST", "", msg)

	// Verify response format
	if !strings.HasPrefix(response, "36N") {
		t.Errorf("Expected response to start with '36N', got: %s", response)
	}

	// Verify contains failure message
	if !strings.Contains(response, "AFFailed to end session") {
		t.Error("Response should contain failure message")
	}
}
