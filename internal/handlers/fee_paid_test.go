package handlers

import (
	"context"
	"strings"
	"testing"

	"github.com/spokanepubliclibrary/fsip2/internal/config"
	"github.com/spokanepubliclibrary/fsip2/internal/sip2/parser"
	"github.com/spokanepubliclibrary/fsip2/internal/types"
	"go.uber.org/zap"
)

func TestFeePaidHandler_Handle_MissingInstitutionID(t *testing.T) {
	tenantConfig := &config.TenantConfig{
		Tenant:                "test-library",
		MessageDelimiter:      "\r",
		FieldDelimiter:        "|",
		ErrorDetectionEnabled: true,
		Charset:               "UTF-8",
	}

	logger := zap.NewNop()
	handler := NewFeePaidHandler(logger, tenantConfig)
	session := types.NewSession("test-session", tenantConfig)

	// Create message without institution ID
	msg := &parser.Message{
		Code:           parser.FeePaidRequest,
		Fields:         make(map[string]string),
		SequenceNumber: "0",
	}
	msg.Fields[string(parser.PatronIdentifier)] = "patron-123"
	msg.Fields[string(parser.FeeAmount)] = "10.00"

	ctx := context.Background()
	response, err := handler.Handle(ctx, msg, session)

	// Should not error but return failure response
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Response should start with "38" (Fee Paid Response)
	if !strings.HasPrefix(response, "38") {
		t.Errorf("Expected fee paid response (38), got: %s", response[:10])
	}

	// Response should indicate failure (N)
	if len(response) >= 3 && response[2:3] != "N" {
		t.Errorf("Expected failed response (N), got: %s", response[2:3])
	}

	// Verify contains sequence number (AY)
	if !strings.Contains(response, "AY0") {
		t.Error("Response should contain sequence number field (AY)")
	}

	// Verify contains checksum (AZ)
	if !strings.Contains(response, "AZ") {
		t.Error("Response should contain checksum field (AZ)")
	}
}

func TestFeePaidHandler_Handle_MissingPatronIdentifier(t *testing.T) {
	tenantConfig := &config.TenantConfig{
		Tenant:           "test-library",
		MessageDelimiter: "\r",
		FieldDelimiter:   "|",
	}

	logger := zap.NewNop()
	handler := NewFeePaidHandler(logger, tenantConfig)
	session := types.NewSession("test-session", tenantConfig)

	// Create message without patron identifier
	msg := &parser.Message{
		Code:   parser.FeePaidRequest,
		Fields: make(map[string]string),
	}
	msg.Fields[string(parser.InstitutionID)] = "TEST-INST"
	msg.Fields[string(parser.FeeAmount)] = "10.00"

	ctx := context.Background()
	response, err := handler.Handle(ctx, msg, session)

	// Should not error but return failure response
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Response should indicate failure
	if len(response) >= 3 && response[2:3] != "N" {
		t.Errorf("Expected failed response, got: %s", response)
	}
}

func TestFeePaidHandler_Handle_MissingFeeAmount(t *testing.T) {
	tenantConfig := &config.TenantConfig{
		Tenant:           "test-library",
		MessageDelimiter: "\r",
		FieldDelimiter:   "|",
	}

	logger := zap.NewNop()
	handler := NewFeePaidHandler(logger, tenantConfig)
	session := types.NewSession("test-session", tenantConfig)

	// Create message without fee amount
	msg := &parser.Message{
		Code:   parser.FeePaidRequest,
		Fields: make(map[string]string),
	}
	msg.Fields[string(parser.InstitutionID)] = "TEST-INST"
	msg.Fields[string(parser.PatronIdentifier)] = "patron-123"

	ctx := context.Background()
	response, err := handler.Handle(ctx, msg, session)

	// Should not error but return failure response
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Response should indicate failure
	if len(response) >= 3 && response[2:3] != "N" {
		t.Errorf("Expected failed response, got: %s", response)
	}

	// Should contain validation failed message
	if !strings.Contains(response, "AFValidation failed") {
		t.Errorf("Expected validation failed message, got: %s", response)
	}
}

func TestFeePaidHandler_Handle_InvalidFeeAmount(t *testing.T) {
	tenantConfig := &config.TenantConfig{
		Tenant:           "test-library",
		MessageDelimiter: "\r",
		FieldDelimiter:   "|",
	}

	logger := zap.NewNop()
	handler := NewFeePaidHandler(logger, tenantConfig)
	session := types.NewSession("test-session", tenantConfig)

	// Create message with invalid fee amount
	msg := &parser.Message{
		Code:   parser.FeePaidRequest,
		Fields: make(map[string]string),
	}
	msg.Fields[string(parser.InstitutionID)] = "TEST-INST"
	msg.Fields[string(parser.PatronIdentifier)] = "patron-123"
	msg.Fields[string(parser.FeeAmount)] = "invalid"

	ctx := context.Background()
	response, err := handler.Handle(ctx, msg, session)

	// Should not error but return failure response
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Response should indicate failure
	if len(response) >= 3 && response[2:3] != "N" {
		t.Errorf("Expected failed response, got: %s", response)
	}

	// Should contain invalid fee amount message
	if !strings.Contains(response, "AFInvalid fee amount") {
		t.Errorf("Expected invalid fee amount message, got: %s", response)
	}
}

func TestFeePaidHandler_Handle_NegativeFeeAmount(t *testing.T) {
	tenantConfig := &config.TenantConfig{
		Tenant:           "test-library",
		MessageDelimiter: "\r",
		FieldDelimiter:   "|",
	}

	logger := zap.NewNop()
	handler := NewFeePaidHandler(logger, tenantConfig)
	session := types.NewSession("test-session", tenantConfig)

	// Create message with negative fee amount
	msg := &parser.Message{
		Code:   parser.FeePaidRequest,
		Fields: make(map[string]string),
	}
	msg.Fields[string(parser.InstitutionID)] = "TEST-INST"
	msg.Fields[string(parser.PatronIdentifier)] = "patron-123"
	msg.Fields[string(parser.FeeAmount)] = "-10.00"

	ctx := context.Background()
	response, err := handler.Handle(ctx, msg, session)

	// Should not error but return failure response
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Response should indicate failure
	if len(response) >= 3 && response[2:3] != "N" {
		t.Errorf("Expected failed response for negative amount, got: %s", response)
	}
}

func TestFeePaidHandler_Handle_ZeroFeeAmount(t *testing.T) {
	tenantConfig := &config.TenantConfig{
		Tenant:           "test-library",
		MessageDelimiter: "\r",
		FieldDelimiter:   "|",
	}

	logger := zap.NewNop()
	handler := NewFeePaidHandler(logger, tenantConfig)
	session := types.NewSession("test-session", tenantConfig)

	// Create message with zero fee amount
	msg := &parser.Message{
		Code:   parser.FeePaidRequest,
		Fields: make(map[string]string),
	}
	msg.Fields[string(parser.InstitutionID)] = "TEST-INST"
	msg.Fields[string(parser.PatronIdentifier)] = "patron-123"
	msg.Fields[string(parser.FeeAmount)] = "0.00"

	ctx := context.Background()
	response, err := handler.Handle(ctx, msg, session)

	// Should not error but return failure response
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Response should indicate failure
	if len(response) >= 3 && response[2:3] != "N" {
		t.Errorf("Expected failed response for zero amount, got: %s", response)
	}
}

func TestFeePaidHandler_BuildErrorResponse(t *testing.T) {
	tenantConfig := &config.TenantConfig{
		Tenant:                "test-library",
		MessageDelimiter:      "\r",
		FieldDelimiter:        "|",
		ErrorDetectionEnabled: true,
		Charset:               "UTF-8",
	}

	logger := zap.NewNop()
	handler := NewFeePaidHandler(logger, tenantConfig)
	session := types.NewSession("test-session", tenantConfig)

	msg := &parser.Message{
		Code:           parser.FeePaidRequest,
		Fields:         make(map[string]string),
		SequenceNumber: "0",
	}

	response := handler.buildErrorResponse("TEST-INST", "patron-123", "Test error", msg, session)

	// Verify response format
	if !strings.HasPrefix(response, "38N") {
		t.Errorf("Expected response to start with '38N', got: %s", response[:10])
	}

	// Verify contains institution ID
	if !strings.Contains(response, "AOTEST-INST") {
		t.Error("Response should contain institution ID field")
	}

	// Verify contains patron identifier
	if !strings.Contains(response, "AApatron-123") {
		t.Error("Response should contain patron identifier field")
	}

	// Verify contains error message
	if !strings.Contains(response, "AFTest error") {
		t.Error("Response should contain error message")
	}

	// Verify contains sequence number (AY)
	if !strings.Contains(response, "AY0") {
		t.Error("Response should contain sequence number field (AY)")
	}

	// Verify contains checksum (AZ)
	if !strings.Contains(response, "AZ") {
		t.Error("Response should contain checksum field (AZ)")
	}
}

func TestFeePaidHandler_BuildErrorResponse_WithTransactionID(t *testing.T) {
	tenantConfig := &config.TenantConfig{
		Tenant:                "test-library",
		MessageDelimiter:      "\r",
		FieldDelimiter:        "|",
		ErrorDetectionEnabled: true,
		Charset:               "UTF-8",
	}

	logger := zap.NewNop()
	handler := NewFeePaidHandler(logger, tenantConfig)
	session := types.NewSession("test-session", tenantConfig)

	msg := &parser.Message{
		Code:           parser.FeePaidRequest,
		Fields:         make(map[string]string),
		SequenceNumber: "1",
	}
	msg.Fields[string(parser.TransactionID)] = "TXN-12345"

	response := handler.buildErrorResponse("TEST-INST", "patron-123", "Test error", msg, session)

	// Verify contains transaction ID
	if !strings.Contains(response, "BKTXN-12345") {
		t.Error("Response should contain transaction ID when provided")
	}

	// Verify contains sequence number (AY)
	if !strings.Contains(response, "AY1") {
		t.Error("Response should contain sequence number field (AY)")
	}

	// Verify contains checksum (AZ)
	if !strings.Contains(response, "AZ") {
		t.Error("Response should contain checksum field (AZ)")
	}
}

func TestFeePaidHandler_ResponseFormat(t *testing.T) {
	tenantConfig := &config.TenantConfig{
		Tenant:           "test-library",
		MessageDelimiter: "\r",
		FieldDelimiter:   "|",
	}

	logger := zap.NewNop()
	handler := NewFeePaidHandler(logger, tenantConfig)
	session := types.NewSession("test-session", tenantConfig)

	// Create minimal valid message (will fail due to no FOLIO connection)
	msg := &parser.Message{
		Code:   parser.FeePaidRequest,
		Fields: make(map[string]string),
	}
	msg.Fields[string(parser.InstitutionID)] = "TEST-INST"
	msg.Fields[string(parser.PatronIdentifier)] = "patron-123"
	msg.Fields[string(parser.FeeAmount)] = "10.00"

	ctx := context.Background()
	response, err := handler.Handle(ctx, msg, session)

	// Should not error but return failure response (no FOLIO connection)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Response should be properly formatted
	if len(response) < 3 {
		t.Errorf("Response too short: %s", response)
	}

	// Should start with "38" (Fee Paid Response code)
	if !strings.HasPrefix(response, "38") {
		t.Errorf("Response should start with '38', got: %s", response)
	}

	// Should contain pipe delimiters for variable fields
	if !strings.Contains(response, "|") {
		t.Errorf("Response should contain field delimiters, got: %s", response)
	}
}

func TestFeePaidHandler_ValidatesAllRequiredFields(t *testing.T) {
	tenantConfig := &config.TenantConfig{
		Tenant:           "test-library",
		MessageDelimiter: "\r",
		FieldDelimiter:   "|",
	}

	logger := zap.NewNop()
	handler := NewFeePaidHandler(logger, tenantConfig)
	session := types.NewSession("test-session", tenantConfig)

	testCases := []struct {
		name          string
		fields        map[string]string
		expectFailure bool
	}{
		{
			name: "All fields present",
			fields: map[string]string{
				string(parser.InstitutionID):    "TEST-INST",
				string(parser.PatronIdentifier): "patron-123",
				string(parser.FeeAmount):        "10.00",
			},
			expectFailure: false, // Will fail for other reasons (no FOLIO), but passes validation
		},
		{
			name: "Missing institution ID",
			fields: map[string]string{
				string(parser.PatronIdentifier): "patron-123",
				string(parser.FeeAmount):        "10.00",
			},
			expectFailure: true,
		},
		{
			name: "Missing patron identifier",
			fields: map[string]string{
				string(parser.InstitutionID): "TEST-INST",
				string(parser.FeeAmount):     "10.00",
			},
			expectFailure: true,
		},
		{
			name: "Missing fee amount",
			fields: map[string]string{
				string(parser.InstitutionID):    "TEST-INST",
				string(parser.PatronIdentifier): "patron-123",
			},
			expectFailure: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			msg := &parser.Message{
				Code:   parser.FeePaidRequest,
				Fields: tc.fields,
			}

			ctx := context.Background()
			response, err := handler.Handle(ctx, msg, session)

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			// Check if validation failed
			hasValidationError := strings.Contains(response, "AFValidation failed")

			if tc.expectFailure && !hasValidationError {
				t.Errorf("Expected validation failure, but got: %s", response)
			}

			if !tc.expectFailure && hasValidationError {
				t.Errorf("Did not expect validation failure, but got: %s", response)
			}
		})
	}
}
