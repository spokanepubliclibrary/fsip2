package customfields

import (
	"testing"

	"github.com/spokanepubliclibrary/fsip2/internal/config"
	"github.com/spokanepubliclibrary/fsip2/internal/folio/models"
	"go.uber.org/zap"
)

func TestProcessCustomFields_Disabled(t *testing.T) {
	user := &models.User{
		ID: "user-123",
		CustomFields: map[string]interface{}{
			"field1": "value1",
		},
	}

	cfg := &config.PatronCustomFieldsConfig{
		Enabled: false,
		Fields: []config.CustomFieldMapping{
			{Code: "SA", Source: "field1", Type: "string", MaxLength: 60},
		},
	}

	logger := zap.NewNop()
	result := ProcessCustomFields(user, cfg, "|", logger)

	if len(result) != 0 {
		t.Errorf("Expected empty result when disabled, got %d fields", len(result))
	}
}

func TestProcessCustomFields_NilConfig(t *testing.T) {
	user := &models.User{
		ID: "user-123",
		CustomFields: map[string]interface{}{
			"field1": "value1",
		},
	}

	logger := zap.NewNop()
	result := ProcessCustomFields(user, nil, "|", logger)

	if len(result) != 0 {
		t.Errorf("Expected empty result when config is nil, got %d fields", len(result))
	}
}

func TestProcessCustomFields_NoCustomFieldsInUser(t *testing.T) {
	user := &models.User{
		ID: "user-123",
	}

	cfg := &config.PatronCustomFieldsConfig{
		Enabled: true,
		Fields: []config.CustomFieldMapping{
			{Code: "SA", Source: "field1", Type: "string", MaxLength: 60},
		},
	}

	logger := zap.NewNop()
	result := ProcessCustomFields(user, cfg, "|", logger)

	if len(result) != 0 {
		t.Errorf("Expected empty result when user has no custom fields, got %d fields", len(result))
	}
}

func TestProcessCustomFields_FieldTypes(t *testing.T) {
	tests := []struct {
		name     string
		fieldKey string
		value    interface{}
		mapping  config.CustomFieldMapping
		expected string
	}{
		{
			name:     "string",
			fieldKey: "email",
			value:    "test@example.com",
			mapping:  config.CustomFieldMapping{Code: "SA", Source: "email", Type: "string", MaxLength: 60},
			expected: "|SAtest@example.com",
		},
		{
			name:     "boolean",
			fieldKey: "active",
			value:    true,
			mapping:  config.CustomFieldMapping{Code: "SB", Source: "active", Type: "boolean", MaxLength: 60},
			expected: "|SBtrue",
		},
		{
			name:     "array_with_delimiter",
			fieldKey: "permissions",
			value:    []interface{}{"opt_1", "opt_2", "opt_3"},
			mapping:  config.CustomFieldMapping{Code: "SC", Source: "permissions", Type: "array", ArrayDelimiter: ",", MaxLength: 60},
			expected: "|SCopt_1,opt_2,opt_3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user := &models.User{
				ID:           "user-123",
				CustomFields: map[string]interface{}{tt.fieldKey: tt.value},
			}
			cfg := &config.PatronCustomFieldsConfig{
				Enabled: true,
				Fields:  []config.CustomFieldMapping{tt.mapping},
			}
			logger := zap.NewNop()
			result := ProcessCustomFields(user, cfg, "|", logger)

			if len(result) != 1 {
				t.Fatalf("Expected 1 field, got %d", len(result))
			}
			if result[0] != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result[0])
			}
		})
	}
}

func TestProcessCustomFields_MultipleFields(t *testing.T) {
	user := &models.User{
		ID: "user-123",
		CustomFields: map[string]interface{}{
			"email":       "test@example.com",
			"active":      true,
			"permissions": []interface{}{"opt_1", "opt_2"},
		},
	}

	cfg := &config.PatronCustomFieldsConfig{
		Enabled: true,
		Fields: []config.CustomFieldMapping{
			{Code: "SA", Source: "email", Type: "string", MaxLength: 60},
			{Code: "SB", Source: "active", Type: "boolean", MaxLength: 60},
			{Code: "SC", Source: "permissions", Type: "array", ArrayDelimiter: ",", MaxLength: 60},
		},
	}

	logger := zap.NewNop()
	result := ProcessCustomFields(user, cfg, "|", logger)

	if len(result) != 3 {
		t.Fatalf("Expected 3 fields, got %d", len(result))
	}

	// Check order is preserved
	expected := []string{
		"|SAtest@example.com",
		"|SBtrue",
		"|SCopt_1,opt_2",
	}

	for i, exp := range expected {
		if result[i] != exp {
			t.Errorf("Field %d: expected %q, got %q", i, exp, result[i])
		}
	}
}

func TestProcessCustomFields_MissingField(t *testing.T) {
	user := &models.User{
		ID: "user-123",
		CustomFields: map[string]interface{}{
			"email": "test@example.com",
		},
	}

	cfg := &config.PatronCustomFieldsConfig{
		Enabled: true,
		Fields: []config.CustomFieldMapping{
			{Code: "SA", Source: "email", Type: "string", MaxLength: 60},
			{Code: "SB", Source: "missing", Type: "string", MaxLength: 60}, // Missing field
			{Code: "SC", Source: "email", Type: "string", MaxLength: 60},
		},
	}

	logger := zap.NewNop()
	result := ProcessCustomFields(user, cfg, "|", logger)

	// Should skip the missing field
	if len(result) != 2 {
		t.Fatalf("Expected 2 fields (skipping missing), got %d", len(result))
	}
}

func TestProcessCustomFields_EmptyString(t *testing.T) {
	user := &models.User{
		ID: "user-123",
		CustomFields: map[string]interface{}{
			"email": "",
		},
	}

	cfg := &config.PatronCustomFieldsConfig{
		Enabled: true,
		Fields: []config.CustomFieldMapping{
			{Code: "SA", Source: "email", Type: "string", MaxLength: 60},
		},
	}

	logger := zap.NewNop()
	result := ProcessCustomFields(user, cfg, "|", logger)

	// Should skip empty strings
	if len(result) != 0 {
		t.Errorf("Expected 0 fields (empty string should be skipped), got %d", len(result))
	}
}

func TestProcessCustomFields_NilValue(t *testing.T) {
	user := &models.User{
		ID: "user-123",
		CustomFields: map[string]interface{}{
			"email": nil,
		},
	}

	cfg := &config.PatronCustomFieldsConfig{
		Enabled: true,
		Fields: []config.CustomFieldMapping{
			{Code: "SA", Source: "email", Type: "string", MaxLength: 60},
		},
	}

	logger := zap.NewNop()
	result := ProcessCustomFields(user, cfg, "|", logger)

	// Should skip nil values
	if len(result) != 0 {
		t.Errorf("Expected 0 fields (nil should be skipped), got %d", len(result))
	}
}

func TestProcessCustomFields_MaxLength(t *testing.T) {
	user := &models.User{
		ID: "user-123",
		CustomFields: map[string]interface{}{
			"longfield": "This is a very long string that exceeds the maximum length of 60 characters",
		},
	}

	cfg := &config.PatronCustomFieldsConfig{
		Enabled: true,
		Fields: []config.CustomFieldMapping{
			{Code: "SA", Source: "longfield", Type: "string", MaxLength: 60},
		},
	}

	logger := zap.NewNop()
	result := ProcessCustomFields(user, cfg, "|", logger)

	if len(result) != 1 {
		t.Fatalf("Expected 1 field, got %d", len(result))
	}

	// Should be truncated to 60 chars (plus |SA prefix)
	if len(result[0]) != 63 { // |SA + 60 chars
		t.Errorf("Expected length 63 (|SA + 60), got %d", len(result[0]))
	}

	expectedPrefix := "|SA"
	if !startsWith(result[0], expectedPrefix) {
		t.Errorf("Expected to start with %q, got %q", expectedPrefix, result[0])
	}
}

func TestProcessCustomFields_PipeInValue(t *testing.T) {
	user := &models.User{
		ID: "user-123",
		CustomFields: map[string]interface{}{
			"badfield": "value|with|pipes",
		},
	}

	cfg := &config.PatronCustomFieldsConfig{
		Enabled: true,
		Fields: []config.CustomFieldMapping{
			{Code: "SA", Source: "badfield", Type: "string", MaxLength: 60},
		},
	}

	logger := zap.NewNop()
	result := ProcessCustomFields(user, cfg, "|", logger)

	// Should skip fields with pipe characters
	if len(result) != 0 {
		t.Errorf("Expected 0 fields (pipe should cause skip), got %d", len(result))
	}
}

func TestProcessCustomFields_QuotesInValue(t *testing.T) {
	user := &models.User{
		ID: "user-123",
		CustomFields: map[string]interface{}{
			"name": `John "dawg" Johnson`,
		},
	}

	cfg := &config.PatronCustomFieldsConfig{
		Enabled: true,
		Fields: []config.CustomFieldMapping{
			{Code: "SA", Source: "name", Type: "string", MaxLength: 60},
		},
	}

	logger := zap.NewNop()
	result := ProcessCustomFields(user, cfg, "|", logger)

	if len(result) != 1 {
		t.Fatalf("Expected 1 field (quotes are allowed), got %d", len(result))
	}

	expected := `|SAJohn "dawg" Johnson`
	if result[0] != expected {
		t.Errorf("Expected %q, got %q", expected, result[0])
	}
}

func TestProcessCustomFields_NewlineInValue(t *testing.T) {
	user := &models.User{
		ID: "user-123",
		CustomFields: map[string]interface{}{
			"badfield": "line1\nline2",
		},
	}

	cfg := &config.PatronCustomFieldsConfig{
		Enabled: true,
		Fields: []config.CustomFieldMapping{
			{Code: "SA", Source: "badfield", Type: "string", MaxLength: 60},
		},
	}

	logger := zap.NewNop()
	result := ProcessCustomFields(user, cfg, "|", logger)

	// Should skip fields with newlines
	if len(result) != 0 {
		t.Errorf("Expected 0 fields (newline should cause skip), got %d", len(result))
	}
}

func TestProcessCustomFields_TypeMismatch_BooleanToString(t *testing.T) {
	user := &models.User{
		ID: "user-123",
		CustomFields: map[string]interface{}{
			"flag": true, // Boolean value
		},
	}

	cfg := &config.PatronCustomFieldsConfig{
		Enabled: true,
		Fields: []config.CustomFieldMapping{
			{Code: "SA", Source: "flag", Type: "string", MaxLength: 60}, // Expecting string
		},
	}

	logger := zap.NewNop()
	result := ProcessCustomFields(user, cfg, "|", logger)

	// Should convert boolean to string
	if len(result) != 1 {
		t.Fatalf("Expected 1 field (should convert), got %d", len(result))
	}

	expected := "|SAtrue"
	if result[0] != expected {
		t.Errorf("Expected %q, got %q", expected, result[0])
	}
}

func TestProcessCustomFields_CaseInsensitiveFieldCode(t *testing.T) {
	user := &models.User{
		ID: "user-123",
		CustomFields: map[string]interface{}{
			"field1": "value1",
		},
	}

	cfg := &config.PatronCustomFieldsConfig{
		Enabled: true,
		Fields: []config.CustomFieldMapping{
			{Code: "sa", Source: "field1", Type: "string", MaxLength: 60}, // lowercase
		},
	}

	logger := zap.NewNop()
	result := ProcessCustomFields(user, cfg, "|", logger)

	if len(result) != 1 {
		t.Fatalf("Expected 1 field, got %d", len(result))
	}

	// Should be uppercased
	expected := "|SAvalue1"
	if result[0] != expected {
		t.Errorf("Expected %q, got %q", expected, result[0])
	}
}

func TestProcessCustomFields_EmptyFields(t *testing.T) {
	user := &models.User{
		ID: "user-123",
		CustomFields: map[string]interface{}{
			"field1": "value1",
		},
	}

	cfg := &config.PatronCustomFieldsConfig{
		Enabled: true,
		Fields:  []config.CustomFieldMapping{},
	}

	logger := zap.NewNop()
	result := ProcessCustomFields(user, cfg, "|", logger)

	if len(result) != 0 {
		t.Errorf("Expected empty result when no field mappings configured, got %d fields", len(result))
	}
}

// TestProcessCustomFields_ArrayAllEmptyElements tests the formattedValue == ""
// guard on processor.go lines 71–76. This path is only reachable when
// formatCustomField returns ("", nil), which happens when an array-type field
// exists in the user record but contains only empty-string elements:
//
//	[]interface{}{""} → strings.Join([""], ",") → ""
//
// Without this test the guard appears dead; it is not — it defends against
// patron custom-field arrays that are present but blank.
func TestProcessCustomFields_ArrayAllEmptyElements(t *testing.T) {
	user := &models.User{
		ID: "user-123",
		CustomFields: map[string]interface{}{
			"permissions": []interface{}{""},
		},
	}

	cfg := &config.PatronCustomFieldsConfig{
		Enabled: true,
		Fields: []config.CustomFieldMapping{
			{Code: "SC", Source: "permissions", Type: "array", ArrayDelimiter: ",", MaxLength: 60},
		},
	}

	logger := zap.NewNop()
	result := ProcessCustomFields(user, cfg, "|", logger)

	if len(result) != 0 {
		t.Errorf("Expected empty result when array contains only empty elements, got %d fields", len(result))
	}
}

func TestFormatCustomField(t *testing.T) {
	tests := []struct {
		name      string
		value     interface{}
		mapping   config.CustomFieldMapping
		wantValue string
		wantErr   bool
	}{
		{
			name:    "boolean_type_mismatch",
			value:   "not-a-bool",
			mapping: config.CustomFieldMapping{Code: "SB", Source: "active", Type: "boolean"},
			wantErr: true,
		},
		{
			name:    "array_empty_slice",
			value:   []interface{}{},
			mapping: config.CustomFieldMapping{Code: "SC", Source: "perms", Type: "array", ArrayDelimiter: ","},
			wantErr: true,
		},
		{
			// array_default_delimiter: when ArrayDelimiter is unset, formatCustomField
			// defaults to "," (processor.go:161–163).
			name:      "array_default_delimiter",
			value:     []interface{}{"a", "b"},
			mapping:   config.CustomFieldMapping{Code: "SC", Source: "perms", Type: "array", ArrayDelimiter: ""},
			wantValue: "a,b",
			wantErr:   false,
		},
		{
			name:    "array_type_mismatch",
			value:   "not-an-array",
			mapping: config.CustomFieldMapping{Code: "SC", Source: "perms", Type: "array", ArrayDelimiter: ","},
			wantErr: true,
		},
		{
			// unknown_type: defensive test for the default branch (processor.go:179–180).
			// In production, config.Validate() rejects unknown types before
			// ProcessCustomFields is ever called, so this branch is unreachable through
			// normal usage. The test documents the fallback behavior.
			name:    "unknown_type",
			value:   "anything",
			mapping: config.CustomFieldMapping{Code: "SX", Source: "field", Type: "unknown"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := zap.NewNop()
			got, err := formatCustomField(tt.value, &tt.mapping, logger)
			if (err != nil) != tt.wantErr {
				t.Errorf("wantErr=%v, got err=%v", tt.wantErr, err)
			}
			if !tt.wantErr && got != tt.wantValue {
				t.Errorf("expected %q, got %q", tt.wantValue, got)
			}
		})
	}
}

func TestValidateAndSanitize_ControlCharacters(t *testing.T) {
	logger := zap.NewNop()
	// \x01 is a non-tab control char (r < 32 && r != 9) → replaced with space.
	// \t (\x09) satisfies r != 9 false so it is preserved as-is.
	input := "hello\x01world\ttab"
	sanitized, valid := validateAndSanitize(input, "|", logger)
	if !valid {
		t.Fatalf("expected valid=true, got false")
	}
	expected := "hello world\ttab"
	if sanitized != expected {
		t.Errorf("expected %q, got %q", expected, sanitized)
	}
}

// Helper function
func startsWith(s, prefix string) bool {
	return len(s) >= len(prefix) && s[0:len(prefix)] == prefix
}
