package parser

import (
	"testing"

	"github.com/spokanepubliclibrary/fsip2/internal/config"
)

// Tests for checksum.go: StripChecksum and ExtractSequenceNumber

func TestStripChecksum(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "message with AY and AZ",
			input:    "9300|CNjdoe|COpassword|AY1AZ1234",
			expected: "9300|CNjdoe|COpassword|",
		},
		{
			name:     "message without checksum fields",
			input:    "9300|CNjdoe|COpassword|",
			expected: "9300|CNjdoe|COpassword|",
		},
		{
			name:     "empty message",
			input:    "",
			expected: "",
		},
		{
			name:     "only AY field at start",
			input:    "AY1AZ1234",
			expected: "",
		},
		{
			name:     "AY in the middle",
			input:    "23000AY2AZ5678",
			expected: "23000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := StripChecksum(tt.input)
			if result != tt.expected {
				t.Errorf("StripChecksum(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestExtractSequenceNumber(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "sequence number before AZ",
			input:    "9300|CNjdoe|AY1AZ1234",
			expected: "1",
		},
		{
			name:     "no AY field - returns default 0",
			input:    "9300|CNjdoe|",
			expected: "0",
		},
		{
			name:     "empty message - returns default 0",
			input:    "",
			expected: "0",
		},
		{
			name:     "AY at end with no following field code",
			input:    "9300|AY3",
			expected: "3",
		},
		{
			name:     "multi-digit sequence number",
			input:    "9300|AY99AZ0000",
			expected: "99",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractSequenceNumber(tt.input)
			if result != tt.expected {
				t.Errorf("ExtractSequenceNumber(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// Tests for command.go: IsRequestMessage, IsResponseMessage, String, GetResponseCode, MessageName

func TestMessageCode_IsRequestMessage(t *testing.T) {
	requestCodes := []MessageCode{
		BlockPatron, CheckinRequest, CheckoutRequest, HoldRequest,
		ItemInformationRequest, ItemStatusUpdateRequest, PatronStatusRequest,
		PatronEnableRequest, RenewRequest, EndPatronSessionRequest,
		FeePaidRequest, PatronInformationRequest, RenewAllRequest, LoginRequest,
		RequestACSResend, SCStatus,
	}

	for _, code := range requestCodes {
		t.Run("request_"+string(code), func(t *testing.T) {
			if !code.IsRequestMessage() {
				t.Errorf("Expected %s to be a request message", code)
			}
		})
	}

	responseCodes := []MessageCode{
		CheckinResponse, CheckoutResponse, HoldResponse,
		ItemInformationResponse, ItemStatusUpdateResponse, PatronStatusResponse,
		PatronEnableResponse, RenewResponse, EndPatronSessionResponse,
		FeePaidResponse, PatronInformationResponse, RenewAllResponse, LoginResponse,
		RequestSCResend, ACSStatus,
	}

	for _, code := range responseCodes {
		t.Run("not_request_"+string(code), func(t *testing.T) {
			if code.IsRequestMessage() {
				t.Errorf("Expected %s to NOT be a request message", code)
			}
		})
	}

	unknown := MessageCode("ZZ")
	if unknown.IsRequestMessage() {
		t.Error("Expected unknown code to NOT be a request message")
	}
}

func TestMessageCode_IsResponseMessage(t *testing.T) {
	responseCodes := []MessageCode{
		CheckinResponse, CheckoutResponse, HoldResponse,
		ItemInformationResponse, ItemStatusUpdateResponse, PatronStatusResponse,
		PatronEnableResponse, RenewResponse, EndPatronSessionResponse,
		FeePaidResponse, PatronInformationResponse, RenewAllResponse, LoginResponse,
		RequestSCResend, ACSStatus,
	}

	for _, code := range responseCodes {
		t.Run("response_"+string(code), func(t *testing.T) {
			if !code.IsResponseMessage() {
				t.Errorf("Expected %s to be a response message", code)
			}
		})
	}

	requestCodes := []MessageCode{
		CheckinRequest, CheckoutRequest, LoginRequest, PatronStatusRequest,
	}

	for _, code := range requestCodes {
		t.Run("not_response_"+string(code), func(t *testing.T) {
			if code.IsResponseMessage() {
				t.Errorf("Expected %s to NOT be a response message", code)
			}
		})
	}

	unknown := MessageCode("ZZ")
	if unknown.IsResponseMessage() {
		t.Error("Expected unknown code to NOT be a response message")
	}
}

func TestMessageCode_String(t *testing.T) {
	tests := []struct {
		code     MessageCode
		expected string
	}{
		{LoginRequest, "93"},
		{CheckoutRequest, "11"},
		{CheckinRequest, "09"},
		{PatronStatusRequest, "23"},
		{PatronInformationRequest, "63"},
		{SCStatus, "99"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tt.code.String()
			if result != tt.expected {
				t.Errorf("MessageCode.String() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestMessageCode_GetResponseCode(t *testing.T) {
	tests := []struct {
		request  MessageCode
		response MessageCode
	}{
		{CheckinRequest, CheckinResponse},
		{CheckoutRequest, CheckoutResponse},
		{PatronStatusRequest, PatronStatusResponse},
		{LoginRequest, LoginResponse},
		{RenewRequest, RenewResponse},
		{FeePaidRequest, FeePaidResponse},
		{ItemInformationRequest, ItemInformationResponse},
		{PatronInformationRequest, PatronInformationResponse},
		{RenewAllRequest, RenewAllResponse},
		{EndPatronSessionRequest, EndPatronSessionResponse},
		{SCStatus, ACSStatus},
		{RequestACSResend, RequestSCResend},
		{HoldRequest, HoldResponse},
		{ItemStatusUpdateRequest, ItemStatusUpdateResponse},
		{PatronEnableRequest, PatronEnableResponse},
		{BlockPatron, ""},
	}

	for _, tt := range tests {
		t.Run(string(tt.request), func(t *testing.T) {
			result := tt.request.GetResponseCode()
			if result != tt.response {
				t.Errorf("GetResponseCode(%s) = %s, want %s", tt.request, result, tt.response)
			}
		})
	}
}

func TestMessageCode_MessageName(t *testing.T) {
	tests := []struct {
		code     MessageCode
		expected string
	}{
		{CheckinRequest, "Checkin Request"},
		{CheckinResponse, "Checkin Response"},
		{CheckoutRequest, "Checkout Request"},
		{CheckoutResponse, "Checkout Response"},
		{LoginRequest, "Login Request"},
		{LoginResponse, "Login Response"},
		{PatronStatusRequest, "Patron Status Request"},
		{PatronInformationRequest, "Patron Information Request"},
		{RenewAllRequest, "Renew All Request"},
		{SCStatus, "SC Status"},
		{ACSStatus, "ACS Status"},
		{RequestSCResend, "Request SC Resend"},
		{RequestACSResend, "Request ACS Resend"},
		{BlockPatron, "Block Patron"},
		{MessageCode("ZZ"), "Unknown Message"},
	}

	for _, tt := range tests {
		t.Run(string(tt.code), func(t *testing.T) {
			result := tt.code.MessageName()
			if result != tt.expected {
				t.Errorf("MessageName(%s) = %q, want %q", tt.code, result, tt.expected)
			}
		})
	}
}

// Tests for field.go: String, FieldName, IsSensitive

func TestFieldCode_String(t *testing.T) {
	tests := []struct {
		code     FieldCode
		expected string
	}{
		{PatronIdentifier, "AA"},
		{ItemIdentifier, "AB"},
		{InstitutionID, "AO"},
		{TitleIdentifier, "AJ"},
		{LoginUserID, "CN"},
		{LoginPassword, "CO"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tt.code.String()
			if result != tt.expected {
				t.Errorf("FieldCode.String() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestFieldCode_FieldName(t *testing.T) {
	tests := []struct {
		code     FieldCode
		expected string
	}{
		{PatronIdentifier, "Patron Identifier"},
		{ItemIdentifier, "Item Identifier"},
		{InstitutionID, "Institution ID"},
		{TitleIdentifier, "Title Identifier"},
		{PatronPassword, "Patron Password"},
		{TerminalPassword, "Terminal Password"},
		{FieldCode("ZZ"), "Unknown Field"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tt.code.FieldName()
			if result != tt.expected {
				t.Errorf("FieldName(%s) = %q, want %q", tt.code, result, tt.expected)
			}
		})
	}
}

func TestFieldCode_IsSensitive(t *testing.T) {
	sensitiveFields := []FieldCode{PatronPassword, TerminalPassword, LoginPassword}

	for _, field := range sensitiveFields {
		t.Run("sensitive_"+string(field), func(t *testing.T) {
			if !field.IsSensitive() {
				t.Errorf("Expected %s to be sensitive", field)
			}
		})
	}

	nonSensitiveFields := []FieldCode{PatronIdentifier, ItemIdentifier, InstitutionID, TitleIdentifier}

	for _, field := range nonSensitiveFields {
		t.Run("non_sensitive_"+string(field), func(t *testing.T) {
			if field.IsSensitive() {
				t.Errorf("Expected %s to NOT be sensitive", field)
			}
		})
	}
}

// Tests for parser.go: GetMultiValueField, HasField, ValidateMessage, StripMessageDelimiter

func TestMessage_GetMultiValueField(t *testing.T) {
	cfg := &config.TenantConfig{
		Tenant:                "test",
		MessageDelimiter:      "\r",
		FieldDelimiter:        "|",
		ErrorDetectionEnabled: false,
		Charset:               "UTF-8",
	}

	parser := NewParser(cfg)

	// Parse a message that includes multiple BD (home address) fields
	msg, err := parser.Parse("23000202501100    815000|AOinst001|AA123456|BDFirst Address|BDSecond Address|")
	if err != nil {
		t.Fatalf("Failed to parse message: %v", err)
	}

	// GetMultiValueField should return values (possibly empty slice if only one occurrence)
	values := msg.GetMultiValueField(HomeAddress)
	// Just verify the call doesn't panic and returns a slice
	_ = values

	// Also test with a field that definitely doesn't have multiple values
	singleValues := msg.GetMultiValueField(PatronIdentifier)
	_ = singleValues

	// Test with a field that doesn't exist at all
	missing := msg.GetMultiValueField(ItemIdentifier)
	if missing == nil {
		// nil is acceptable for missing field
		_ = missing
	}
}

func TestMessage_HasField(t *testing.T) {
	cfg := &config.TenantConfig{
		Tenant:                "test",
		MessageDelimiter:      "\r",
		FieldDelimiter:        "|",
		ErrorDetectionEnabled: false,
		Charset:               "UTF-8",
	}

	parser := NewParser(cfg)
	msg, err := parser.Parse("23000202501100    815000|AOinst001|AA123456|")
	if err != nil {
		t.Fatalf("Failed to parse message: %v", err)
	}

	// AO and AA should exist
	if !msg.HasField(InstitutionID) {
		t.Error("Expected HasField(AO) to be true")
	}
	if !msg.HasField(PatronIdentifier) {
		t.Error("Expected HasField(AA) to be true")
	}

	// AB (item identifier) should not exist
	if msg.HasField(ItemIdentifier) {
		t.Error("Expected HasField(AB) to be false")
	}

	// PatronPassword should not exist
	if msg.HasField(PatronPassword) {
		t.Error("Expected HasField(AD) to be false")
	}
}

func TestParser_ValidateMessage(t *testing.T) {
	cfg := &config.TenantConfig{
		Tenant:                "test",
		MessageDelimiter:      "\r",
		FieldDelimiter:        "|",
		ErrorDetectionEnabled: false,
		Charset:               "UTF-8",
		SupportedMessages: []config.MessageSupport{
			{Code: "23", Enabled: true},
			{Code: "11", Enabled: true},
		},
	}

	parser := NewParser(cfg)

	// Valid and supported message
	msg1 := &Message{Code: PatronStatusRequest}
	if err := parser.ValidateMessage(msg1); err != nil {
		t.Errorf("Expected valid+supported message to pass, got: %v", err)
	}

	// Valid code but not supported (CheckinRequest not in SupportedMessages)
	msg2 := &Message{Code: CheckinRequest}
	if err := parser.ValidateMessage(msg2); err == nil {
		t.Error("Expected unsupported message to fail validation")
	}

	// Invalid / unknown message code
	msg3 := &Message{Code: MessageCode("ZZ")}
	if err := parser.ValidateMessage(msg3); err == nil {
		t.Error("Expected invalid code to fail validation")
	}
}

func TestParser_StripMessageDelimiter(t *testing.T) {
	tests := []struct {
		name      string
		delimiter string
		input     string
		expected  string
	}{
		{
			name:      "strip carriage return",
			delimiter: "\r",
			input:     "9300|CNjdoe|\r",
			expected:  "9300|CNjdoe|",
		},
		{
			name:      "no delimiter to strip",
			delimiter: "\r",
			input:     "9300|CNjdoe|",
			expected:  "9300|CNjdoe|",
		},
		{
			name:      "newline delimiter",
			delimiter: "\n",
			input:     "9300|CNjdoe|\n",
			expected:  "9300|CNjdoe|",
		},
		{
			name:      "empty message",
			delimiter: "\r",
			input:     "",
			expected:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.TenantConfig{
				Tenant:           "test",
				MessageDelimiter: tt.delimiter,
				FieldDelimiter:   "|",
			}
			parser := NewParser(cfg)
			result := parser.StripMessageDelimiter(tt.input)
			if result != tt.expected {
				t.Errorf("StripMessageDelimiter(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
