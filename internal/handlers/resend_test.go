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

func TestResendHandler_Handle_Success(t *testing.T) {
	// Create test configuration
	tenantConfig := &config.TenantConfig{
		Tenant:           "test-library",
		MessageDelimiter: "\r",
		FieldDelimiter:   "|",
	}

	// Create logger
	logger := zap.NewNop()

	// Create handler
	handler := NewResendHandler(logger, tenantConfig)

	// Create session
	session := types.NewSession("test-session-123", tenantConfig)

	// Create Resend request message (97)
	msg := &parser.Message{
		Code:   parser.RequestSCResend,
		Fields: make(map[string]string),
	}

	ctx := context.Background()
	response, err := handler.Handle(ctx, msg, session)

	// Verify no error
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Verify response
	if response != "96" {
		t.Errorf("Expected response '96', got: %s", response)
	}
}

func TestResendHandler_Handle_MultipleRequests(t *testing.T) {
	tenantConfig := &config.TenantConfig{
		Tenant:           "test-library",
		MessageDelimiter: "\r",
		FieldDelimiter:   "|",
	}

	logger := zap.NewNop()
	handler := NewResendHandler(logger, tenantConfig)

	session := types.NewSession("test-session", tenantConfig)

	msg := &parser.Message{
		Code:   parser.RequestSCResend,
		Fields: make(map[string]string),
	}

	ctx := context.Background()

	// Send multiple resend requests - all should return the same response
	for i := 0; i < 5; i++ {
		response, err := handler.Handle(ctx, msg, session)

		if err != nil {
			t.Errorf("Request %d: Unexpected error: %v", i, err)
		}

		if response != "96" {
			t.Errorf("Request %d: Expected response '96', got: %s", i, response)
		}
	}
}

func TestResendHandler_BuildResendResponse(t *testing.T) {
	tenantConfig := &config.TenantConfig{
		Tenant:           "test-library",
		MessageDelimiter: "\r",
		FieldDelimiter:   "|",
	}

	logger := zap.NewNop()
	handler := NewResendHandler(logger, tenantConfig)

	response := handler.buildResendResponse()

	// Verify response is exactly "96"
	if response != "96" {
		t.Errorf("Expected response '96', got: %s", response)
	}

	// Verify response length
	if len(response) != 2 {
		t.Errorf("Expected response length 2, got: %d", len(response))
	}
}

func TestResendHandler_Handle_WithAuthenticated(t *testing.T) {
	tenantConfig := &config.TenantConfig{
		Tenant:           "test-library",
		MessageDelimiter: "\r",
		FieldDelimiter:   "|",
	}

	logger := zap.NewNop()
	handler := NewResendHandler(logger, tenantConfig)

	session := types.NewSession("test-session", tenantConfig)
	expiresAt := time.Now().Add(10 * time.Minute)
	session.SetAuthenticated("testuser", "patron-123", "barcode-456", "test-token", expiresAt)

	msg := &parser.Message{
		Code:   parser.RequestSCResend,
		Fields: make(map[string]string),
	}

	ctx := context.Background()
	response, err := handler.Handle(ctx, msg, session)

	// Resend handler doesn't care about authentication status
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if response != "96" {
		t.Errorf("Expected response '96', got: %s", response)
	}

	// Verify session wasn't modified
	if !session.IsAuth() {
		t.Error("Session authentication state should not be modified")
	}
}

func TestResendHandler_Handle_WithoutAuthentication(t *testing.T) {
	tenantConfig := &config.TenantConfig{
		Tenant:           "test-library",
		MessageDelimiter: "\r",
		FieldDelimiter:   "|",
	}

	logger := zap.NewNop()
	handler := NewResendHandler(logger, tenantConfig)

	session := types.NewSession("test-session", tenantConfig)
	// Session is not authenticated

	msg := &parser.Message{
		Code:   parser.RequestSCResend,
		Fields: make(map[string]string),
	}

	ctx := context.Background()
	response, err := handler.Handle(ctx, msg, session)

	// Resend handler works regardless of authentication
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if response != "96" {
		t.Errorf("Expected response '96', got: %s", response)
	}
}

func TestResendHandler_ResponseFormat(t *testing.T) {
	tenantConfig := &config.TenantConfig{
		Tenant:           "test-library",
		MessageDelimiter: "\r",
		FieldDelimiter:   "|",
	}

	logger := zap.NewNop()
	handler := NewResendHandler(logger, tenantConfig)

	session := types.NewSession("test-session", tenantConfig)

	msg := &parser.Message{
		Code:   parser.RequestSCResend,
		Fields: make(map[string]string),
	}

	ctx := context.Background()
	response, err := handler.Handle(ctx, msg, session)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Response should be exactly "96" - no variable fields, no delimiters
	if response != "96" {
		t.Errorf("Expected exact response '96', got: %s", response)
	}

	// Should not contain any delimiters
	if len(response) > 2 {
		t.Errorf("Response should not contain any additional data, got: %s", response)
	}
}
