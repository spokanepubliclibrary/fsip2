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

func TestItemStatusUpdateHandler_Handle_MissingInstitutionID(t *testing.T) {
	tenantConfig := &config.TenantConfig{
		Tenant:           "test-library",
		MessageDelimiter: "\r",
		FieldDelimiter:   "|",
	}

	logger := zap.NewNop()
	handler := NewItemStatusUpdateHandler(logger, tenantConfig)
	session := types.NewSession("test-session", tenantConfig)

	// Create message without institution ID
	msg := &parser.Message{
		Code:   parser.ItemStatusUpdateRequest,
		Fields: make(map[string]string),
	}
	msg.Fields[string(parser.ItemIdentifier)] = "item-123"

	ctx := context.Background()
	response, err := handler.Handle(ctx, msg, session)

	// Should not error but return failure response
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Response should start with "20" (Item Status Update Response)
	if !strings.HasPrefix(response, "20") {
		t.Errorf("Expected response to start with '20', got: %s", response[:10])
	}

	// Response should indicate failure (0)
	if len(response) >= 3 && response[2:3] != "0" {
		t.Errorf("Expected failed response (0), got: %s", response[2:3])
	}
}

func TestItemStatusUpdateHandler_Handle_MissingItemIdentifier(t *testing.T) {
	tenantConfig := &config.TenantConfig{
		Tenant:           "test-library",
		MessageDelimiter: "\r",
		FieldDelimiter:   "|",
	}

	logger := zap.NewNop()
	handler := NewItemStatusUpdateHandler(logger, tenantConfig)
	session := types.NewSession("test-session", tenantConfig)

	// Create message without item identifier
	msg := &parser.Message{
		Code:   parser.ItemStatusUpdateRequest,
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
	if len(response) >= 3 && response[2:3] != "0" {
		t.Errorf("Expected failed response, got: %s", response)
	}
}

func TestItemStatusUpdateHandler_Handle_AllFieldsMissing(t *testing.T) {
	tenantConfig := &config.TenantConfig{
		Tenant:           "test-library",
		MessageDelimiter: "\r",
		FieldDelimiter:   "|",
	}

	logger := zap.NewNop()
	handler := NewItemStatusUpdateHandler(logger, tenantConfig)
	session := types.NewSession("test-session", tenantConfig)

	// Create message with no fields
	msg := &parser.Message{
		Code:   parser.ItemStatusUpdateRequest,
		Fields: make(map[string]string),
	}

	ctx := context.Background()
	response, err := handler.Handle(ctx, msg, session)

	// Should not error but return failure response
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Response should start with "20" and indicate failure "0"
	if !strings.HasPrefix(response, "200") {
		t.Errorf("Expected response to start with '200' (failure), got: %s", response[:10])
	}
}

func TestItemStatusUpdateHandler_BuildItemStatusUpdateResponse_Success(t *testing.T) {
	tenantConfig := &config.TenantConfig{
		Tenant:           "test-library",
		MessageDelimiter: "\r",
		FieldDelimiter:   "|",
	}

	logger := zap.NewNop()
	handler := NewItemStatusUpdateHandler(logger, tenantConfig)

	msg := &parser.Message{
		Code:   parser.ItemStatusUpdateRequest,
		Fields: make(map[string]string),
	}

	response := handler.buildItemStatusUpdateResponse(true, "TEST-INST", "item-123", msg)

	// Verify response format
	if !strings.HasPrefix(response, "201") {
		t.Errorf("Expected response to start with '201' (success), got: %s", response[:10])
	}

	// Verify contains item identifier (AB field)
	if !strings.Contains(response, "ABitem-123") {
		t.Error("Response should contain item identifier field")
	}

	// Verify contains institution ID (AO field)
	if !strings.Contains(response, "AOTEST-INST") {
		t.Error("Response should contain institution ID field")
	}

	// Verify contains title (AJ field)
	if !strings.Contains(response, "AJ") {
		t.Error("Response should contain title field")
	}

	// Verify contains success message
	if !strings.Contains(response, "AFItem properties updated") {
		t.Error("Response should contain success message")
	}
}

func TestItemStatusUpdateHandler_BuildItemStatusUpdateResponse_Failure(t *testing.T) {
	tenantConfig := &config.TenantConfig{
		Tenant:           "test-library",
		MessageDelimiter: "\r",
		FieldDelimiter:   "|",
	}

	logger := zap.NewNop()
	handler := NewItemStatusUpdateHandler(logger, tenantConfig)

	msg := &parser.Message{
		Code:   parser.ItemStatusUpdateRequest,
		Fields: make(map[string]string),
	}

	response := handler.buildItemStatusUpdateResponse(false, "TEST-INST", "item-456", msg)

	// Verify response format
	if !strings.HasPrefix(response, "200") {
		t.Errorf("Expected response to start with '200' (failure), got: %s", response[:10])
	}

	// Verify contains failure message
	if !strings.Contains(response, "AFItem properties update failed") {
		t.Error("Response should contain failure message")
	}
}

func TestItemStatusUpdateHandler_ResponseFormat(t *testing.T) {
	tenantConfig := &config.TenantConfig{
		Tenant:           "test-library",
		MessageDelimiter: "\r",
		FieldDelimiter:   "|",
	}

	logger := zap.NewNop()
	handler := NewItemStatusUpdateHandler(logger, tenantConfig)
	session := types.NewSession("test-session", tenantConfig)

	// Create minimal valid message (will fail due to no FOLIO connection)
	msg := &parser.Message{
		Code:   parser.ItemStatusUpdateRequest,
		Fields: make(map[string]string),
	}
	msg.Fields[string(parser.InstitutionID)] = "TEST-INST"
	msg.Fields[string(parser.ItemIdentifier)] = "item-123"

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

	// Should start with "20" (Item Status Update Response code)
	if !strings.HasPrefix(response, "20") {
		t.Errorf("Response should start with '20', got: %s", response)
	}

	// Should contain pipe delimiters for variable fields
	if !strings.Contains(response, "|") {
		t.Errorf("Response should contain field delimiters, got: %s", response)
	}
}

func TestItemStatusUpdateHandler_StatusIndicator(t *testing.T) {
	tenantConfig := &config.TenantConfig{
		Tenant:           "test-library",
		MessageDelimiter: "\r",
		FieldDelimiter:   "|",
	}

	logger := zap.NewNop()
	handler := NewItemStatusUpdateHandler(logger, tenantConfig)

	msg := &parser.Message{
		Code:   parser.ItemStatusUpdateRequest,
		Fields: make(map[string]string),
	}

	testCases := []struct {
		name              string
		ok                bool
		expectedIndicator string
	}{
		{
			name:              "Success indicator",
			ok:                true,
			expectedIndicator: "201",
		},
		{
			name:              "Failure indicator",
			ok:                false,
			expectedIndicator: "200",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			response := handler.buildItemStatusUpdateResponse(tc.ok, "TEST-INST", "item-123", msg)

			if !strings.HasPrefix(response, tc.expectedIndicator) {
				t.Errorf("Expected response to start with '%s', got: %s", tc.expectedIndicator, response[:10])
			}
		})
	}
}

func TestItemStatusUpdateHandler_ContainsRequiredFields(t *testing.T) {
	tenantConfig := &config.TenantConfig{
		Tenant:           "test-library",
		MessageDelimiter: "\r",
		FieldDelimiter:   "|",
	}

	logger := zap.NewNop()
	handler := NewItemStatusUpdateHandler(logger, tenantConfig)

	msg := &parser.Message{
		Code:   parser.ItemStatusUpdateRequest,
		Fields: make(map[string]string),
	}

	response := handler.buildItemStatusUpdateResponse(true, "TEST-INST", "item-123", msg)

	// Check for required SIP2 fields
	requiredFields := []string{
		"AB", // Item identifier
		"AO", // Institution ID
		"AJ", // Title identifier
		"AF", // Screen message
	}

	for _, field := range requiredFields {
		if !strings.Contains(response, field) {
			t.Errorf("Response should contain field '%s', got: %s", field, response)
		}
	}
}
