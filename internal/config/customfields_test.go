package config

import (
	"strings"
	"testing"
)

func TestPatronCustomFieldsConfig_Validate_Disabled(t *testing.T) {
	cfg := &PatronCustomFieldsConfig{
		Enabled: false,
		Fields: []CustomFieldMapping{
			{Code: "ZZ", Source: "invalid", Type: "invalid"}, // Invalid but disabled
		},
	}

	err := cfg.Validate()
	if err != nil {
		t.Errorf("Expected no error when disabled, got: %v", err)
	}
}

func TestPatronCustomFieldsConfig_Validate_ValidFields(t *testing.T) {
	cfg := &PatronCustomFieldsConfig{
		Enabled: true,
		Fields: []CustomFieldMapping{
			{Code: "SA", Source: "field1", Type: "string"},
			{Code: "SB", Source: "field2", Type: "boolean"},
			{Code: "SC", Source: "field3", Type: "array", ArrayDelimiter: ","},
		},
	}

	err := cfg.Validate()
	if err != nil {
		t.Errorf("Expected no error for valid config, got: %v", err)
	}

	// Check defaults were set
	if cfg.Fields[0].MaxLength != 60 {
		t.Errorf("Expected default MaxLength 60, got %d", cfg.Fields[0].MaxLength)
	}
	if cfg.Fields[2].ArrayDelimiter != "," {
		t.Errorf("Expected ArrayDelimiter ',', got %q", cfg.Fields[2].ArrayDelimiter)
	}
}

func TestPatronCustomFieldsConfig_Validate_InvalidFieldCode(t *testing.T) {
	testCases := []struct {
		name string
		code string
	}{
		{"Too short", "S"},
		{"Too long", "SAA"},
		{"Wrong prefix", "AA"},
		{"Number", "S1"},
		{"Special char", "S@"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &PatronCustomFieldsConfig{
				Enabled: true,
				Fields: []CustomFieldMapping{
					{Code: tc.code, Source: "field1", Type: "string"},
				},
			}

			err := cfg.Validate()
			if err == nil {
				t.Errorf("Expected error for invalid field code %q, got nil", tc.code)
			}
			if !strings.Contains(err.Error(), "invalid field code") {
				t.Errorf("Expected 'invalid field code' error, got: %v", err)
			}
		})
	}
}

func TestPatronCustomFieldsConfig_Validate_ValidFieldCodes(t *testing.T) {
	testCases := []string{
		"SA", "SB", "SC", "SD", "SE", "SF", "SG", "SH", "SI", "SJ",
		"SK", "SL", "SM", "SN", "SO", "SP", "SQ", "SR", "SS", "ST",
		"SU", "SV", "SW", "SX", "SY", "SZ",
		"sa", "sb", "sz", // lowercase should work
	}

	for _, code := range testCases {
		t.Run(code, func(t *testing.T) {
			cfg := &PatronCustomFieldsConfig{
				Enabled: true,
				Fields: []CustomFieldMapping{
					{Code: code, Source: "field1", Type: "string"},
				},
			}

			err := cfg.Validate()
			if err != nil {
				t.Errorf("Expected no error for valid field code %q, got: %v", code, err)
			}
		})
	}
}

func TestPatronCustomFieldsConfig_Validate_DuplicateFieldCodes(t *testing.T) {
	cfg := &PatronCustomFieldsConfig{
		Enabled: true,
		Fields: []CustomFieldMapping{
			{Code: "SA", Source: "field1", Type: "string"},
			{Code: "SA", Source: "field2", Type: "string"}, // Duplicate
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("Expected error for duplicate field codes, got nil")
	}
	if !strings.Contains(err.Error(), "duplicate field code") {
		t.Errorf("Expected 'duplicate field code' error, got: %v", err)
	}
}

func TestPatronCustomFieldsConfig_Validate_DuplicateFieldCodes_CaseInsensitive(t *testing.T) {
	cfg := &PatronCustomFieldsConfig{
		Enabled: true,
		Fields: []CustomFieldMapping{
			{Code: "SA", Source: "field1", Type: "string"},
			{Code: "sa", Source: "field2", Type: "string"}, // Duplicate (case-insensitive)
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("Expected error for duplicate field codes (case-insensitive), got nil")
	}
	if !strings.Contains(err.Error(), "duplicate field code") {
		t.Errorf("Expected 'duplicate field code' error, got: %v", err)
	}
}

func TestPatronCustomFieldsConfig_Validate_EmptySource(t *testing.T) {
	cfg := &PatronCustomFieldsConfig{
		Enabled: true,
		Fields: []CustomFieldMapping{
			{Code: "SA", Source: "", Type: "string"}, // Empty source
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("Expected error for empty source, got nil")
	}
	if !strings.Contains(err.Error(), "source field is required") {
		t.Errorf("Expected 'source field is required' error, got: %v", err)
	}
}

func TestPatronCustomFieldsConfig_Validate_InvalidType(t *testing.T) {
	testCases := []string{"invalid", "int", "number", ""}

	for _, invalidType := range testCases {
		t.Run(invalidType, func(t *testing.T) {
			cfg := &PatronCustomFieldsConfig{
				Enabled: true,
				Fields: []CustomFieldMapping{
					{Code: "SA", Source: "field1", Type: invalidType},
				},
			}

			err := cfg.Validate()
			if err == nil {
				t.Errorf("Expected error for invalid type %q, got nil", invalidType)
			}
			if !strings.Contains(err.Error(), "invalid type") {
				t.Errorf("Expected 'invalid type' error, got: %v", err)
			}
		})
	}
}

func TestPatronCustomFieldsConfig_Validate_ValidTypes(t *testing.T) {
	testCases := []string{"string", "boolean", "array"}

	for _, validType := range testCases {
		t.Run(validType, func(t *testing.T) {
			cfg := &PatronCustomFieldsConfig{
				Enabled: true,
				Fields: []CustomFieldMapping{
					{Code: "SA", Source: "field1", Type: validType},
				},
			}

			err := cfg.Validate()
			if err != nil {
				t.Errorf("Expected no error for valid type %q, got: %v", validType, err)
			}
		})
	}
}

func TestPatronCustomFieldsConfig_Validate_ArrayDelimiterDefault(t *testing.T) {
	cfg := &PatronCustomFieldsConfig{
		Enabled: true,
		Fields: []CustomFieldMapping{
			{Code: "SA", Source: "field1", Type: "array"}, // No delimiter specified
		},
	}

	err := cfg.Validate()
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// Should set default delimiter
	if cfg.Fields[0].ArrayDelimiter != "," {
		t.Errorf("Expected default ArrayDelimiter ',', got %q", cfg.Fields[0].ArrayDelimiter)
	}
}

func TestPatronCustomFieldsConfig_Validate_MaxLengthDefault(t *testing.T) {
	cfg := &PatronCustomFieldsConfig{
		Enabled: true,
		Fields: []CustomFieldMapping{
			{Code: "SA", Source: "field1", Type: "string"}, // No MaxLength specified
		},
	}

	err := cfg.Validate()
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// Should set default MaxLength
	if cfg.Fields[0].MaxLength != 60 {
		t.Errorf("Expected default MaxLength 60, got %d", cfg.Fields[0].MaxLength)
	}
}

func TestPatronCustomFieldsConfig_Validate_EmptyFieldsEnabled(t *testing.T) {
	cfg := &PatronCustomFieldsConfig{
		Enabled: true,
		Fields:  []CustomFieldMapping{}, // No fields
	}

	err := cfg.Validate()
	if err != nil {
		t.Errorf("Expected no error for enabled with no fields (no-op), got: %v", err)
	}
}

func TestIsValidCustomFieldCode(t *testing.T) {
	testCases := []struct {
		code     string
		expected bool
	}{
		{"SA", true},
		{"SZ", true},
		{"sa", true},
		{"sz", true},
		{"S", false},
		{"SAA", false},
		{"AA", false},
		{"A", false},
		{"S1", false},
		{"S@", false},
		{"", false},
	}

	for _, tc := range testCases {
		t.Run(tc.code, func(t *testing.T) {
			result := isValidCustomFieldCode(tc.code)
			if result != tc.expected {
				t.Errorf("isValidCustomFieldCode(%q) = %v, expected %v", tc.code, result, tc.expected)
			}
		})
	}
}

func TestIsValidCustomFieldType(t *testing.T) {
	testCases := []struct {
		fieldType string
		expected  bool
	}{
		{"string", true},
		{"boolean", true},
		{"array", true},
		{"invalid", false},
		{"int", false},
		{"", false},
	}

	for _, tc := range testCases {
		t.Run(tc.fieldType, func(t *testing.T) {
			result := isValidCustomFieldType(tc.fieldType)
			if result != tc.expected {
				t.Errorf("isValidCustomFieldType(%q) = %v, expected %v", tc.fieldType, result, tc.expected)
			}
		})
	}
}
