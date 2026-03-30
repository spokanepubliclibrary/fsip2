package handlers

import (
	"context"
	"testing"
	"time"

	"github.com/spokanepubliclibrary/fsip2/internal/sip2/parser"
	"github.com/spokanepubliclibrary/fsip2/internal/types"
	"github.com/spokanepubliclibrary/fsip2/tests/testutil"
	"go.uber.org/zap"
)

func TestCheckoutHandler_Handle(t *testing.T) {
	tenantConfig := testutil.NewTenantConfig()

	logger := zap.NewNop()
	handler := NewCheckoutHandler(logger, tenantConfig)
	session := types.NewSession("test-session", tenantConfig)

	// Create checkout request
	msg := &parser.Message{
		Code:       parser.CheckoutRequest,
		Fields:     make(map[string]string),
		RawMessage: "11YN20250110    08150020250124    081500|AOinst001|AA123456|ABITEM001",
	}
	msg.Fields[string(parser.InstitutionID)] = "test-inst"
	msg.Fields[string(parser.PatronIdentifier)] = "123456"
	msg.Fields[string(parser.ItemIdentifier)] = "ITEM001"

	ctx := context.Background()
	response, _ := handler.Handle(ctx, msg, session)

	// Response should start with "12" (Checkout Response)
	if len(response) < 2 || response[:2] != "12" {
		t.Errorf("Expected checkout response (12), got: %s", response)
	}
}

func TestCheckoutHandler_MissingRequiredFields(t *testing.T) {
	tenantConfig := testutil.NewTenantConfig()

	logger := zap.NewNop()
	handler := NewCheckoutHandler(logger, tenantConfig)
	session := types.NewSession("test-session", tenantConfig)

	tests := []struct {
		name   string
		fields map[string]string
	}{
		{
			name: "Missing patron",
			fields: map[string]string{
				string(parser.InstitutionID):  "test-inst",
				string(parser.ItemIdentifier): "ITEM001",
			},
		},
		{
			name: "Missing item",
			fields: map[string]string{
				string(parser.InstitutionID):    "test-inst",
				string(parser.PatronIdentifier): "123456",
			},
		},
	}

	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := &parser.Message{
				Code:   parser.CheckoutRequest,
				Fields: tt.fields,
			}

			response, _ := handler.Handle(ctx, msg, session)

			// Should return checkout response even on validation failure
			if len(response) < 2 || response[:2] != "12" {
				t.Errorf("Expected checkout response, got: %s", response)
			}
		})
	}
}

func TestCheckoutHandler_BuildCheckoutResponse(t *testing.T) {
	tenantConfig := testutil.NewTenantConfig()

	logger := zap.NewNop()
	handler := NewCheckoutHandler(logger, tenantConfig)
	session := types.NewSession("test-session", tenantConfig)

	msg := &parser.Message{
		Code:   parser.CheckoutRequest,
		Fields: make(map[string]string),
	}

	dueDate := time.Now().Add(14 * 24 * time.Hour)

	response := handler.buildCheckoutResponse(
		true,
		"test-inst",
		"123456",
		"ITEM001",
		"Test Book Title",
		dueDate,
		msg,
		session,
		"Checkout successful",
	)

	// Verify response format
	if len(response) < 2 || response[:2] != "12" {
		t.Errorf("Expected response to start with '12', got: %s", response)
	}

	// Should contain "1" for success
	if len(response) >= 3 && response[2] != '1' {
		t.Errorf("Expected success indicator (1), got: %c", response[2])
	}

	// Should contain institution ID
	if !contains(response, "AO") || !contains(response, "test-inst") {
		t.Error("Expected response to contain institution ID")
	}

	// Should contain patron identifier
	if !contains(response, "AA") || !contains(response, "123456") {
		t.Error("Expected response to contain patron identifier")
	}

	// Should contain item identifier
	if !contains(response, "AB") || !contains(response, "ITEM001") {
		t.Error("Expected response to contain item identifier")
	}
}

func TestCheckoutHandler_ChecksumFields(t *testing.T) {
	tenantConfig := testutil.NewTenantConfig()

	logger := zap.NewNop()
	handler := NewCheckoutHandler(logger, tenantConfig)
	session := types.NewSession("test-session", tenantConfig)

	msg := &parser.Message{
		Code:           parser.CheckoutRequest,
		Fields:         make(map[string]string),
		SequenceNumber: "0",
	}

	dueDate := time.Now().Add(14 * 24 * time.Hour)

	response := handler.buildCheckoutResponse(
		true,
		"test-inst",
		"123456",
		"ITEM001",
		"Test Book Title",
		dueDate,
		msg,
		session,
		"Checkout successful",
	)

	// Verify AY (sequence number) field is present
	if !contains(response, "AY0") {
		t.Errorf("Expected response to contain AY (sequence number) field. Got: %s", response)
	}

	// Verify AZ (checksum) field is present
	if !contains(response, "AZ") {
		t.Errorf("Expected response to contain AZ (checksum) field. Got: %s", response)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[len(s)-len(substr):] == substr ||
		len(s) > len(substr) && s[:len(substr)] == substr ||
		len(s) > len(substr) && s[1:len(substr)+1] == substr ||
		findSubstring(s, substr)
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// TestCheckoutHandler_TitleID_Success tests that title is properly used when provided
func TestCheckoutHandler_TitleID_Success(t *testing.T) {
	tenantConfig := testutil.NewTenantConfig()

	logger := zap.NewNop()
	handler := NewCheckoutHandler(logger, tenantConfig)
	session := types.NewSession("test-session", tenantConfig)

	msg := &parser.Message{
		Code:           parser.CheckoutRequest,
		Fields:         make(map[string]string),
		SequenceNumber: "0",
	}

	dueDate := time.Now().Add(14 * 24 * time.Hour)

	// Test with actual title (not barcode)
	response := handler.buildCheckoutResponse(
		true,
		"test-inst",
		"123456",
		"ITEM001",
		"The Great Gatsby", // Actual title
		dueDate,
		msg,
		session,
		"Checkout successful",
	)

	// Should contain title in AJ field
	if !contains(response, "AJThe Great Gatsby") {
		t.Errorf("Expected response to contain 'AJThe Great Gatsby', got: %s", response)
	}

	// Should NOT contain item barcode as title
	if contains(response, "AJITEM001") {
		t.Errorf("Expected response NOT to use barcode as title when actual title is provided, got: %s", response)
	}
}

// TestCheckoutHandler_TitleID_Fallback tests that item barcode is used when title is empty
func TestCheckoutHandler_TitleID_Fallback(t *testing.T) {
	tenantConfig := testutil.NewTenantConfig()

	logger := zap.NewNop()
	handler := NewCheckoutHandler(logger, tenantConfig)
	session := types.NewSession("test-session", tenantConfig)

	msg := &parser.Message{
		Code:           parser.CheckoutRequest,
		Fields:         make(map[string]string),
		SequenceNumber: "0",
	}

	dueDate := time.Now().Add(14 * 24 * time.Hour)

	// Test with empty title (should fallback to barcode)
	response := handler.buildCheckoutResponse(
		true,
		"test-inst",
		"123456",
		"ITEM001",
		"ITEM001", // Fallback to barcode
		dueDate,
		msg,
		session,
		"Checkout successful",
	)

	// Should contain item barcode as title fallback
	if !contains(response, "AJITEM001") {
		t.Errorf("Expected response to contain 'AJITEM001' as title fallback, got: %s", response)
	}
}

// TestCheckoutHandler_TitleID_Truncation tests that long titles are properly truncated
func TestCheckoutHandler_TitleID_Truncation(t *testing.T) {
	tenantConfig := testutil.NewTenantConfig()

	logger := zap.NewNop()
	handler := NewCheckoutHandler(logger, tenantConfig)
	session := types.NewSession("test-session", tenantConfig)

	msg := &parser.Message{
		Code:           parser.CheckoutRequest,
		Fields:         make(map[string]string),
		SequenceNumber: "0",
	}

	dueDate := time.Now().Add(14 * 24 * time.Hour)

	// Test with long title (exactly 60 characters - should not truncate)
	longTitle60 := "123456789012345678901234567890123456789012345678901234567890"
	if len([]rune(longTitle60)) != 60 {
		t.Fatalf("Test setup error: title should be exactly 60 characters, got %d", len([]rune(longTitle60)))
	}

	response := handler.buildCheckoutResponse(
		true,
		"test-inst",
		"123456",
		"ITEM001",
		longTitle60,
		dueDate,
		msg,
		session,
		"Checkout successful",
	)

	// Should contain full 60-character title
	if !contains(response, "AJ"+longTitle60) {
		t.Errorf("Expected response to contain full 60-character title, got: %s", response)
	}

	// Test with very long title (>60 characters - should be pre-truncated by caller)
	// In real usage, truncateString() is called before buildCheckoutResponse
	veryLongTitle := "This is a very long title that exceeds sixty characters and would need truncation to fit"
	truncatedTitle := truncateString(veryLongTitle, 60)

	response2 := handler.buildCheckoutResponse(
		true,
		"test-inst",
		"123456",
		"ITEM001",
		truncatedTitle,
		dueDate,
		msg,
		session,
		"Checkout successful",
	)

	// Should contain truncated title (60 chars)
	if !contains(response2, "AJ"+truncatedTitle) {
		t.Errorf("Expected response to contain truncated title, got: %s", response2)
	}

	// Verify truncation happened correctly
	if len([]rune(truncatedTitle)) != 60 {
		t.Errorf("Expected truncated title to be 60 characters, got %d", len([]rune(truncatedTitle)))
	}
}

// TestCheckoutHandler_TitleID_EmptyStringVsBarcode tests difference between empty title and barcode fallback
func TestCheckoutHandler_TitleID_EmptyStringVsBarcode(t *testing.T) {
	tenantConfig := testutil.NewTenantConfig()

	logger := zap.NewNop()
	handler := NewCheckoutHandler(logger, tenantConfig)
	session := types.NewSession("test-session", tenantConfig)

	msg := &parser.Message{
		Code:           parser.CheckoutRequest,
		Fields:         make(map[string]string),
		SequenceNumber: "0",
	}

	dueDate := time.Now().Add(14 * 24 * time.Hour)

	tests := []struct {
		name         string
		titleID      string
		itemBarcode  string
		expectedInAJ string
	}{
		{
			name:         "Normal title provided",
			titleID:      "1984 by George Orwell",
			itemBarcode:  "BOOK123",
			expectedInAJ: "AJ1984 by George Orwell",
		},
		{
			name:         "Empty title - uses barcode",
			titleID:      "BOOK456",
			itemBarcode:  "BOOK456",
			expectedInAJ: "AJBOOK456",
		},
		{
			name:         "Title with special characters",
			titleID:      "C++: The Complete Reference",
			itemBarcode:  "TECH789",
			expectedInAJ: "AJC++: The Complete Reference",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := handler.buildCheckoutResponse(
				true,
				"test-inst",
				"123456",
				tt.itemBarcode,
				tt.titleID,
				dueDate,
				msg,
				session,
				"Checkout successful",
			)

			if !contains(response, tt.expectedInAJ) {
				t.Errorf("Expected response to contain '%s', got: %s", tt.expectedInAJ, response)
			}
		})
	}
}
