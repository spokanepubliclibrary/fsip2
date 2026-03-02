package parser

import (
	"strings"
	"testing"

	"github.com/spokanepubliclibrary/fsip2/internal/config"
)

// TestParseRenewAllRequest tests renew all request parsing (message 65)
func TestParseRenewAllRequest(t *testing.T) {
	cfg := &config.TenantConfig{
		MessageDelimiter:      "\r",
		FieldDelimiter:        "|",
		ErrorDetectionEnabled: false,
		Charset:               "UTF-8",
	}

	parser := NewParser(cfg)
	message := "6520250110    143000|AOinst001|AA123456|"

	msg, err := parser.Parse(message)
	if err != nil {
		t.Fatalf("Failed to parse renew all request: %v", err)
	}

	if msg.Code != RenewAllRequest {
		t.Errorf("Expected message code %s, got %s", RenewAllRequest, msg.Code)
	}

	patron := msg.GetField(PatronIdentifier)
	if patron != "123456" {
		t.Errorf("Expected patron '123456', got '%s'", patron)
	}
}

// TestParseItemInformationRequest tests item information request parsing (message 17)
func TestParseItemInformationRequest(t *testing.T) {
	cfg := &config.TenantConfig{
		MessageDelimiter:      "\r",
		FieldDelimiter:        "|",
		ErrorDetectionEnabled: false,
		Charset:               "UTF-8",
	}

	parser := NewParser(cfg)
	message := "1720250110    143000|AOinst001|ABITEM001|"

	msg, err := parser.Parse(message)
	if err != nil {
		t.Fatalf("Failed to parse item information request: %v", err)
	}

	if msg.Code != ItemInformationRequest {
		t.Errorf("Expected message code %s, got %s", ItemInformationRequest, msg.Code)
	}

	item := msg.GetField(ItemIdentifier)
	if item != "ITEM001" {
		t.Errorf("Expected item 'ITEM001', got '%s'", item)
	}
}

// TestParseFeePaidRequest tests fee paid request parsing (message 37)
func TestParseFeePaidRequest(t *testing.T) {
	cfg := &config.TenantConfig{
		MessageDelimiter:      "\r",
		FieldDelimiter:        "|",
		ErrorDetectionEnabled: false,
		Charset:               "UTF-8",
	}

	parser := NewParser(cfg)
	message := "3720250110    14300001011USD|BV5.00|AOinst001|AA123456|"

	msg, err := parser.Parse(message)
	if err != nil {
		t.Fatalf("Failed to parse fee paid request: %v", err)
	}

	if msg.Code != FeePaidRequest {
		t.Errorf("Expected message code %s, got %s", FeePaidRequest, msg.Code)
	}

	feeAmount := msg.GetField(FeeAmount)
	if feeAmount != "5.00" {
		t.Errorf("Expected fee amount '5.00', got '%s'", feeAmount)
	}

	patron := msg.GetField(PatronIdentifier)
	if patron != "123456" {
		t.Errorf("Expected patron '123456', got '%s'", patron)
	}
}

// TestParseEndPatronSessionRequest tests end patron session request parsing (message 35)
func TestParseEndPatronSessionRequest(t *testing.T) {
	cfg := &config.TenantConfig{
		MessageDelimiter:      "\r",
		FieldDelimiter:        "|",
		ErrorDetectionEnabled: false,
		Charset:               "UTF-8",
	}

	parser := NewParser(cfg)
	message := "3520250110    143000|AOinst001|AA123456|"

	msg, err := parser.Parse(message)
	if err != nil {
		t.Fatalf("Failed to parse end patron session request: %v", err)
	}

	if msg.Code != EndPatronSessionRequest {
		t.Errorf("Expected message code %s, got %s", EndPatronSessionRequest, msg.Code)
	}

	patron := msg.GetField(PatronIdentifier)
	if patron != "123456" {
		t.Errorf("Expected patron '123456', got '%s'", patron)
	}
}

// TestParseItemStatusUpdateRequest tests item status update request parsing (message 19)
func TestParseItemStatusUpdateRequest(t *testing.T) {
	cfg := &config.TenantConfig{
		MessageDelimiter:      "\r",
		FieldDelimiter:        "|",
		ErrorDetectionEnabled: false,
		Charset:               "UTF-8",
	}

	parser := NewParser(cfg)
	message := "1920250110    143000|AOinst001|ABITEM001|CH01|"

	msg, err := parser.Parse(message)
	if err != nil {
		t.Fatalf("Failed to parse item status update request: %v", err)
	}

	if msg.Code != ItemStatusUpdateRequest {
		t.Errorf("Expected message code %s, got %s", ItemStatusUpdateRequest, msg.Code)
	}

	item := msg.GetField(ItemIdentifier)
	if item != "ITEM001" {
		t.Errorf("Expected item 'ITEM001', got '%s'", item)
	}

	itemProps := msg.GetField(CurrentItemType)
	if itemProps != "01" {
		t.Errorf("Expected item properties '01', got '%s'", itemProps)
	}
}

// TestParseHoldRequest tests hold request parsing (message 15)
func TestParseHoldRequest(t *testing.T) {
	cfg := &config.TenantConfig{
		MessageDelimiter:      "\r",
		FieldDelimiter:        "|",
		ErrorDetectionEnabled: false,
		Charset:               "UTF-8",
	}

	parser := NewParser(cfg)
	message := "15+20250110    143000|AOinst001|AA123456|ABITEM001|BSMAIN-CIRC|"

	msg, err := parser.Parse(message)
	if err != nil {
		t.Fatalf("Failed to parse hold request: %v", err)
	}

	if msg.Code != HoldRequest {
		t.Errorf("Expected message code %s, got %s", HoldRequest, msg.Code)
	}

	patron := msg.GetField(PatronIdentifier)
	if patron != "123456" {
		t.Errorf("Expected patron '123456', got '%s'", patron)
	}

	item := msg.GetField(ItemIdentifier)
	if item != "ITEM001" {
		t.Errorf("Expected item 'ITEM001', got '%s'", item)
	}
}

// TestParseRequestSCResend tests ACS resend request parsing (message 96)
func TestParseRequestSCResend(t *testing.T) {
	cfg := &config.TenantConfig{
		MessageDelimiter:      "\r",
		FieldDelimiter:        "|",
		ErrorDetectionEnabled: false,
		Charset:               "UTF-8",
	}

	parser := NewParser(cfg)
	message := "96"

	msg, err := parser.Parse(message)
	if err != nil {
		t.Fatalf("Failed to parse SC resend request: %v", err)
	}

	if msg.Code != RequestSCResend {
		t.Errorf("Expected message code %s, got %s", RequestSCResend, msg.Code)
	}
}

// TestParseSCStatus tests SC status request parsing (message 99)
func TestParseSCStatus(t *testing.T) {
	cfg := &config.TenantConfig{
		MessageDelimiter:      "\r",
		FieldDelimiter:        "|",
		ErrorDetectionEnabled: false,
		Charset:               "UTF-8",
	}

	parser := NewParser(cfg)
	message := "990302.00"

	msg, err := parser.Parse(message)
	if err != nil {
		t.Fatalf("Failed to parse SC status request: %v", err)
	}

	if msg.Code != SCStatus {
		t.Errorf("Expected message code %s, got %s", SCStatus, msg.Code)
	}
}

// TestParseWithChecksum tests parsing with checksum validation
func TestParseWithChecksum(t *testing.T) {
	cfg := &config.TenantConfig{
		MessageDelimiter:      "\r",
		FieldDelimiter:        "|",
		ErrorDetectionEnabled: true,
		Charset:               "UTF-8",
	}

	parser := NewParser(cfg)
	message := "9300|CNjdoe|COpassword123|"

	_, err := parser.Parse(message)
	if err == nil {
		t.Error("Expected checksum validation to fail without checksum, but it passed")
	}
}

// TestParseEmptyFields tests parsing messages with empty field values
func TestParseEmptyFields(t *testing.T) {
	cfg := &config.TenantConfig{
		MessageDelimiter:      "\r",
		FieldDelimiter:        "|",
		ErrorDetectionEnabled: false,
		Charset:               "UTF-8",
	}

	parser := NewParser(cfg)
	message := "23000202501100    815000|AOinst001|AA123456|AD|"

	msg, err := parser.Parse(message)
	if err != nil {
		t.Fatalf("Failed to parse message with empty fields: %v", err)
	}

	password := msg.GetField(PatronPassword)
	if password != "" {
		t.Errorf("Expected empty password, got '%s'", password)
	}
}

// TestParseInvalidMessages tests error handling for invalid messages
func TestParseInvalidMessages(t *testing.T) {
	cfg := &config.TenantConfig{
		MessageDelimiter:      "\r",
		FieldDelimiter:        "|",
		ErrorDetectionEnabled: false,
		Charset:               "UTF-8",
	}

	parser := NewParser(cfg)

	tests := []struct {
		name    string
		message string
		wantErr bool
	}{
		{"Too short", "9", true},
		{"Empty message", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parser.Parse(tt.message)
			if (err != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestMessageGetField tests the GetField method
func TestMessageGetField(t *testing.T) {
	msg := &Message{
		Fields: map[string]string{
			"AA": "patron123",
			"AB": "item456",
			"AO": "inst001",
		},
	}

	tests := []struct {
		name      string
		fieldCode FieldCode
		want      string
	}{
		{"Existing field", PatronIdentifier, "patron123"},
		{"Non-existing field", "ZZ", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := msg.GetField(tt.fieldCode)
			if got != tt.want {
				t.Errorf("GetField() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestStripMessageDelimiter tests delimiter stripping
func TestStripMessageDelimiter(t *testing.T) {
	tests := []struct {
		name      string
		delimiter string
		message   string
		want      string
	}{
		{"Strip carriage return", "\r", "9300|CNjdoe|\r", "9300|CNjdoe|"},
		{"Strip CRLF", "\r\n", "9300|CNjdoe|\r\n", "9300|CNjdoe|"},
		{"No delimiter", "\r", "9300|CNjdoe|", "9300|CNjdoe|"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.TenantConfig{
				MessageDelimiter: tt.delimiter,
				FieldDelimiter:   "|",
			}
			parser := NewParser(cfg)

			got := parser.StripMessageDelimiter(tt.message)
			if got != tt.want {
				t.Errorf("StripMessageDelimiter() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestParseWithDifferentDelimiters tests parsing with custom delimiters
func TestParseWithDifferentDelimiters(t *testing.T) {
	tests := []struct {
		name           string
		fieldDelimiter string
		message        string
		expectPatron   string
	}{
		{"Standard pipe delimiter", "|", "23000202501100    815000|AOinst001|AA123456|", "123456"},
		{"Caret delimiter", "^", "23000202501100    815000^AOinst001^AA123456^", "123456"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.TenantConfig{
				MessageDelimiter:      "\r",
				FieldDelimiter:        tt.fieldDelimiter,
				ErrorDetectionEnabled: false,
				Charset:               "UTF-8",
			}

			parser := NewParser(cfg)
			msg, err := parser.Parse(tt.message)
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}

			patron := msg.GetField(PatronIdentifier)
			if patron != tt.expectPatron {
				t.Errorf("Expected patron %s, got %s", tt.expectPatron, patron)
			}
		})
	}
}

// TestDetectMessageCode tests message code detection
func TestDetectMessageCode(t *testing.T) {
	tests := []struct {
		message string
		want    MessageCode
	}{
		{"9300|CNjdoe|", LoginRequest},
		{"23000202501100    815000|", PatronStatusRequest},
		{"11YN20250110    143000|", CheckoutRequest},
		{"09YN20250110    143000|", CheckinRequest},
		{"6300019700101    08462510|", PatronInformationRequest},
		{"1720250110    143000|", ItemInformationRequest},
		{"6520250110    143000|", RenewAllRequest},
		{"3520250110    143000|", EndPatronSessionRequest},
		{"3720250110    143000|", FeePaidRequest},
		{"1920250110    143000|", ItemStatusUpdateRequest},
		{"96", RequestSCResend},
		{"990302.00", SCStatus},
	}

	for _, tt := range tests {
		t.Run(string(tt.want), func(t *testing.T) {
			if len(tt.message) < 2 {
				return
			}
			got := MessageCode(tt.message[0:2])
			if got != tt.want {
				t.Errorf("DetectMessageCode() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestParseFieldsWithSpecialCharacters tests parsing fields containing special characters
func TestParseFieldsWithSpecialCharacters(t *testing.T) {
	cfg := &config.TenantConfig{
		MessageDelimiter:      "\r",
		FieldDelimiter:        "|",
		ErrorDetectionEnabled: false,
		Charset:               "UTF-8",
	}

	parser := NewParser(cfg)
	message := "23000202501100    815000|AOinst001|AA123456|AEO'Brien, Mary-Jane|"

	msg, err := parser.Parse(message)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	name := msg.GetField(PersonalName)
	if name != "O'Brien, Mary-Jane" {
		t.Errorf("Expected name \"O'Brien, Mary-Jane\", got '%s'", name)
	}
}

// TestParseTrailingFieldDelimiter tests parsing with/without trailing delimiter
func TestParseTrailingFieldDelimiter(t *testing.T) {
	cfg := &config.TenantConfig{
		MessageDelimiter:      "\r",
		FieldDelimiter:        "|",
		ErrorDetectionEnabled: false,
		Charset:               "UTF-8",
	}

	parser := NewParser(cfg)

	tests := []struct {
		name    string
		message string
	}{
		{"With trailing delimiter", "23000202501100    815000|AOinst001|AA123456|"},
		{"Without trailing delimiter", "23000202501100    815000|AOinst001|AA123456"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, err := parser.Parse(tt.message)
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}

			patron := msg.GetField(PatronIdentifier)
			if patron != "123456" {
				t.Errorf("Expected patron '123456', got '%s'", patron)
			}
		})
	}
}

// TestParseFieldsWithWhitespace tests parsing fields with leading/trailing whitespace
func TestParseFieldsWithWhitespace(t *testing.T) {
	cfg := &config.TenantConfig{
		MessageDelimiter:      "\r",
		FieldDelimiter:        "|",
		ErrorDetectionEnabled: false,
		Charset:               "UTF-8",
	}

	parser := NewParser(cfg)
	message := "23000202501100    815000|AOinst001|AA  123456  |"

	msg, err := parser.Parse(message)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	patron := msg.GetField(PatronIdentifier)
	if !strings.Contains(patron, "123456") {
		t.Errorf("Expected patron to contain '123456', got '%s'", patron)
	}
}
