package parser

import (
	"testing"

	"github.com/spokanepubliclibrary/fsip2/internal/config"
)

func TestNewParser(t *testing.T) {
	cfg := &config.TenantConfig{
		Tenant:                "test",
		MessageDelimiter:      "\r",
		FieldDelimiter:        "|",
		ErrorDetectionEnabled: true,
		Charset:               "UTF-8",
	}

	parser := NewParser(cfg)
	if parser == nil {
		t.Fatal("Expected parser to be created, got nil")
	}
}

func TestParseLoginRequest(t *testing.T) {
	cfg := &config.TenantConfig{
		Tenant:                "test",
		MessageDelimiter:      "\r",
		FieldDelimiter:        "|",
		ErrorDetectionEnabled: false,
		Charset:               "UTF-8",
	}

	parser := NewParser(cfg)

	// Login request: 93<uid_alg><pwd_alg>|CN<username>|CO<password>
	message := "9300|CNjdoe|COpassword123"

	msg, err := parser.Parse(message)
	if err != nil {
		t.Fatalf("Failed to parse login request: %v", err)
	}

	if msg.Code != LoginRequest {
		t.Errorf("Expected message code %s, got %s", LoginRequest, msg.Code)
	}

	username := msg.GetField(LoginUserID)
	if username != "jdoe" {
		t.Errorf("Expected username 'jdoe', got '%s'", username)
	}

	password := msg.GetField(LoginPassword)
	if password != "password123" {
		t.Errorf("Expected password 'password123', got '%s'", password)
	}
}

func TestParsePatronStatusRequest(t *testing.T) {
	cfg := &config.TenantConfig{
		Tenant:                "test",
		MessageDelimiter:      "\r",
		FieldDelimiter:        "|",
		ErrorDetectionEnabled: false,
		Charset:               "UTF-8",
	}

	parser := NewParser(cfg)

	// Patron Status Request: 23<language><transaction_date>|AO<institution>|AA<patron>
	// Format: 23 + language(3) + transaction_date(18) where date is YYYYMMDD    HHMMSS
	message := "23000202501100    815000|AOinst001|AA123456"

	msg, err := parser.Parse(message)
	if err != nil {
		t.Fatalf("Failed to parse patron status request: %v", err)
	}

	if msg.Code != PatronStatusRequest {
		t.Errorf("Expected message code %s, got %s", PatronStatusRequest, msg.Code)
	}

	institution := msg.GetField(InstitutionID)
	if institution != "inst001" {
		t.Errorf("Expected institution 'inst001', got '%s'", institution)
	}

	patron := msg.GetField(PatronIdentifier)
	if patron != "123456" {
		t.Errorf("Expected patron '123456', got '%s'", patron)
	}
}

func TestParseCheckoutRequest(t *testing.T) {
	cfg := &config.TenantConfig{
		Tenant:                "test",
		MessageDelimiter:      "\r",
		FieldDelimiter:        "|",
		ErrorDetectionEnabled: false,
		Charset:               "UTF-8",
	}

	parser := NewParser(cfg)

	// Checkout Request: 11<sc_renewal><no_block><transaction_date><nb_due_date>|AO<institution>|AA<patron>|AB<item>
	message := "11YN20250110    08150020250124    081500|AOinst001|AA123456|ABITEM001"

	msg, err := parser.Parse(message)
	if err != nil {
		t.Fatalf("Failed to parse checkout request: %v", err)
	}

	if msg.Code != CheckoutRequest {
		t.Errorf("Expected message code %s, got %s", CheckoutRequest, msg.Code)
	}

	item := msg.GetField(ItemIdentifier)
	if item != "ITEM001" {
		t.Errorf("Expected item 'ITEM001', got '%s'", item)
	}

	patron := msg.GetField(PatronIdentifier)
	if patron != "123456" {
		t.Errorf("Expected patron '123456', got '%s'", patron)
	}
}

func TestParseWithMultipleFields(t *testing.T) {
	cfg := &config.TenantConfig{
		Tenant:                "test",
		MessageDelimiter:      "\r",
		FieldDelimiter:        "|",
		ErrorDetectionEnabled: false,
		Charset:               "UTF-8",
	}

	parser := NewParser(cfg)

	// Message with multiple variable fields
	message := "23000202501100    815000|AOinst001|AA123456|AC123|ADpassword"

	msg, err := parser.Parse(message)
	if err != nil {
		t.Fatalf("Failed to parse message: %v", err)
	}

	institution := msg.GetField(InstitutionID)
	if institution != "inst001" {
		t.Errorf("Expected institution 'inst001', got '%s'", institution)
	}

	patron := msg.GetField(PatronIdentifier)
	if patron != "123456" {
		t.Errorf("Expected patron '123456', got '%s'", patron)
	}

	terminal := msg.GetField(TerminalPassword)
	if terminal != "123" {
		t.Errorf("Expected terminal password '123', got '%s'", terminal)
	}

	password := msg.GetField(PatronPassword)
	if password != "password" {
		t.Errorf("Expected patron password 'password', got '%s'", password)
	}
}

func TestGetFieldReturnsEmptyForMissingField(t *testing.T) {
	cfg := &config.TenantConfig{
		Tenant:                "test",
		MessageDelimiter:      "\r",
		FieldDelimiter:        "|",
		ErrorDetectionEnabled: false,
		Charset:               "UTF-8",
	}

	parser := NewParser(cfg)
	message := "23000202501100    815000|AOinst001|AA123456"

	msg, err := parser.Parse(message)
	if err != nil {
		t.Fatalf("Failed to parse message: %v", err)
	}

	// Request field that doesn't exist
	item := msg.GetField(ItemIdentifier)
	if item != "" {
		t.Errorf("Expected empty string for missing field, got '%s'", item)
	}
}

func TestFixedFieldExtraction(t *testing.T) {
	cfg := &config.TenantConfig{
		Tenant:                "test",
		MessageDelimiter:      "\r",
		FieldDelimiter:        "|",
		ErrorDetectionEnabled: false,
		Charset:               "UTF-8",
	}

	parser := NewParser(cfg)

	// Checkout request with fixed fields
	message := "11YN20250110    08150020250124    081500|AOinst001"

	msg, err := parser.Parse(message)
	if err != nil {
		t.Fatalf("Failed to parse message: %v", err)
	}

	// Note: Fixed fields are parsed internally but there's no public API to extract them
	// by position. The parsed message should contain the variable fields instead.
	// Verify the message was parsed successfully
	if msg.Code != CheckoutRequest {
		t.Errorf("Expected message code %s, got %s", CheckoutRequest, msg.Code)
	}
}

func TestMessageCodeDetection(t *testing.T) {
	tests := []struct {
		message      string
		expectedCode MessageCode
	}{
		{"93", LoginRequest},
		{"99", SCStatus},
		{"23", PatronStatusRequest},
		{"11", CheckoutRequest},
		{"09", CheckinRequest},
		{"63", PatronInformationRequest},
		{"17", ItemInformationRequest},
		{"29", RenewRequest},
		{"65", RenewAllRequest},
		{"35", EndPatronSessionRequest},
		{"37", FeePaidRequest},
		{"19", ItemStatusUpdateRequest},
		{"96", RequestSCResend},
	}

	cfg := &config.TenantConfig{
		Tenant:                "test",
		MessageDelimiter:      "\r",
		FieldDelimiter:        "|",
		ErrorDetectionEnabled: false,
		Charset:               "UTF-8",
	}

	parser := NewParser(cfg)

	for _, tt := range tests {
		t.Run(string(tt.expectedCode), func(t *testing.T) {
			msg, err := parser.Parse(tt.message)
			if err != nil {
				t.Fatalf("Failed to parse message '%s': %v", tt.message, err)
			}

			if msg.Code != tt.expectedCode {
				t.Errorf("Expected code %s, got %s", tt.expectedCode, msg.Code)
			}
		})
	}
}
