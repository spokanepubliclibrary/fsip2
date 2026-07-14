package config

import (
	"testing"
)

// TestTenantConfig_IsMessageSupported tests the IsMessageSupported method
func TestTenantConfig_IsMessageSupported(t *testing.T) {
	tc := &TenantConfig{
		SupportedMessages: []MessageSupport{
			{Code: "23", Enabled: true},
			{Code: "11", Enabled: true},
			{Code: "09", Enabled: false},
		},
	}

	tests := []struct {
		name     string
		code     string
		expected bool
	}{
		{"Supported message - patron status", "23", true},
		{"Supported message - checkout", "11", true},
		{"Disabled message - checkin", "09", false},
		{"Unsupported message", "99", false},
		{"Empty code", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tc.IsMessageSupported(tt.code)
			if result != tt.expected {
				t.Errorf("IsMessageSupported(%s) = %v, want %v", tt.code, result, tt.expected)
			}
		})
	}
}

// TestUnescapeDelimiter tests the UnescapeDelimiter helper function
func TestUnescapeDelimiter(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     `"\\r" converts to actual CR`,
			input:    "\\r",
			expected: "\r",
		},
		{
			name:     `"\\n" converts to actual LF`,
			input:    "\\n",
			expected: "\n",
		},
		{
			name:     `"\\r\\n" converts to actual CRLF`,
			input:    "\\r\\n",
			expected: "\r\n",
		},
		{
			name:     "actual CR passes through unchanged",
			input:    "\r",
			expected: "\r",
		},
		{
			name:     "actual LF passes through unchanged",
			input:    "\n",
			expected: "\n",
		},
		{
			name:     "pipe passes through unchanged",
			input:    "|",
			expected: "|",
		},
		{
			name:     "empty string passes through unchanged",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := UnescapeDelimiter(tt.input)
			if result != tt.expected {
				t.Errorf("UnescapeDelimiter(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestTenantConfig_GetMessageDelimiterBytes tests the GetMessageDelimiterBytes method.
// MessageDelimiter is normalized at load time, so the struct always holds actual bytes.
func TestTenantConfig_GetMessageDelimiterBytes(t *testing.T) {
	tests := []struct {
		name      string
		delimiter string
		expected  []byte
	}{
		{
			name:      "actual CR delimiter",
			delimiter: "\r",
			expected:  []byte{'\r'},
		},
		{
			name:      "actual CRLF delimiter",
			delimiter: "\r\n",
			expected:  []byte{'\r', '\n'},
		},
		{
			name:      "actual LF delimiter",
			delimiter: "\n",
			expected:  []byte{'\n'},
		},
		{
			name:      "empty delimiter returns empty",
			delimiter: "",
			expected:  []byte{},
		},
		{
			name:      "pipe delimiter",
			delimiter: "|",
			expected:  []byte{'|'},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc := &TenantConfig{
				MessageDelimiter: tt.delimiter,
			}

			result := tc.GetMessageDelimiterBytes()

			if len(result) != len(tt.expected) {
				t.Errorf("GetMessageDelimiterBytes() returned %d bytes, want %d", len(result), len(tt.expected))
				return
			}

			for i := range result {
				if result[i] != tt.expected[i] {
					t.Errorf("GetMessageDelimiterBytes() byte %d = %v, want %v", i, result[i], tt.expected[i])
				}
			}
		})
	}
}

// TestTenantConfig_GetFieldDelimiterBytes tests the GetFieldDelimiterBytes method
func TestTenantConfig_GetFieldDelimiterBytes(t *testing.T) {
	tests := []struct {
		name      string
		delimiter string
		expected  []byte
	}{
		{
			name:      "Pipe delimiter",
			delimiter: "|",
			expected:  []byte{'|'},
		},
		{
			name:      "Empty delimiter returns empty",
			delimiter: "",
			expected:  []byte{},
		},
		{
			name:      "Custom delimiter",
			delimiter: "~",
			expected:  []byte{'~'},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc := &TenantConfig{
				FieldDelimiter: tt.delimiter,
			}

			result := tc.GetFieldDelimiterBytes()

			if len(result) != len(tt.expected) {
				t.Errorf("GetFieldDelimiterBytes() returned %d bytes, want %d", len(result), len(tt.expected))
				return
			}

			for i := range result {
				if result[i] != tt.expected[i] {
					t.Errorf("GetFieldDelimiterBytes() byte %d = %v, want %v", i, result[i], tt.expected[i])
				}
			}
		})
	}
}

// TestTenantConfig_GetRenewAllMaxItems tests the GetRenewAllMaxItems method
func TestTenantConfig_GetRenewAllMaxItems(t *testing.T) {
	tests := []struct {
		name     string
		maxItems int
		expected int
	}{
		{
			name:     "Default value when not set",
			maxItems: 0,
			expected: 50,
		},
		{
			name:     "Custom value - 10 items",
			maxItems: 10,
			expected: 10,
		},
		{
			name:     "Custom value - 100 items",
			maxItems: 100,
			expected: 100,
		},
		{
			name:     "Negative value defaults to 50",
			maxItems: -5,
			expected: 50,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc := &TenantConfig{
				RenewAllMaxItems: tt.maxItems,
			}

			result := tc.GetRenewAllMaxItems()

			if result != tt.expected {
				t.Errorf("GetRenewAllMaxItems() = %d, want %d", result, tt.expected)
			}
		})
	}
}

// TestTenantConfig_GetInstitutionID tests the GetInstitutionID fallback chain
func TestTenantConfig_GetInstitutionID(t *testing.T) {
	tests := []struct {
		name          string
		tenant        string
		institutionID string
		expected      string
	}{
		{
			name:          "InstitutionID set — returned as-is",
			tenant:        "folio-tenant",
			institutionID: "Spokane Public Library",
			expected:      "Spokane Public Library",
		},
		{
			name:          "InstitutionID empty — falls back to Tenant",
			tenant:        "folio-tenant",
			institutionID: "",
			expected:      "folio-tenant",
		},
		{
			name:          "Both empty — returns empty string",
			tenant:        "",
			institutionID: "",
			expected:      "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc := &TenantConfig{
				Tenant:        tt.tenant,
				InstitutionID: tt.institutionID,
			}
			result := tc.GetInstitutionID()
			if result != tt.expected {
				t.Errorf("GetInstitutionID() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// TestTenantConfig_GetLibraryName tests the GetLibraryName fallback chain
func TestTenantConfig_GetLibraryName(t *testing.T) {
	tests := []struct {
		name          string
		tenant        string
		institutionID string
		libraryName   string
		expected      string
	}{
		{
			name:          "LibraryName set — returned as-is",
			tenant:        "folio-tenant",
			institutionID: "SPL",
			libraryName:   "Spokane Public Library",
			expected:      "Spokane Public Library",
		},
		{
			name:          "LibraryName empty — falls back to InstitutionID",
			tenant:        "folio-tenant",
			institutionID: "SPL",
			libraryName:   "",
			expected:      "SPL",
		},
		{
			name:          "LibraryName and InstitutionID empty — falls back to Tenant",
			tenant:        "folio-tenant",
			institutionID: "",
			libraryName:   "",
			expected:      "folio-tenant",
		},
		{
			name:          "All empty — returns empty string",
			tenant:        "",
			institutionID: "",
			libraryName:   "",
			expected:      "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc := &TenantConfig{
				Tenant:        tt.tenant,
				InstitutionID: tt.institutionID,
				LibraryName:   tt.libraryName,
			}
			result := tc.GetLibraryName()
			if result != tt.expected {
				t.Errorf("GetLibraryName() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// boolPtr returns a pointer to the given bool value, used in FieldConfiguration tests.
func boolPtr(b bool) *bool { return &b }

// TestTenantConfig_GetFieldConfig tests the GetFieldConfig method
func TestTenantConfig_GetFieldConfig(t *testing.T) {
	tc := &TenantConfig{
		SupportedMessages: []MessageSupport{
			{
				Code:    "63",
				Enabled: true,
				Fields: []FieldConfiguration{
					{Code: "AE", Enabled: true, PreferredFirstName: boolPtr(true)},
					{Code: "AA", Enabled: false},
				},
			},
			{
				Code:    "23",
				Enabled: true,
				Fields:  []FieldConfiguration{},
			},
		},
	}

	tests := []struct {
		name        string
		messageCode string
		fieldCode   string
		wantNil     bool
		wantCode    string
	}{
		{
			name:        "message code not in SupportedMessages returns nil",
			messageCode: "99",
			fieldCode:   "AE",
			wantNil:     true,
		},
		{
			name:        "message found but field code not in Fields returns nil",
			messageCode: "63",
			fieldCode:   "ZZ",
			wantNil:     true,
		},
		{
			name:        "both message and field found returns pointer",
			messageCode: "63",
			fieldCode:   "AE",
			wantNil:     false,
			wantCode:    "AE",
		},
		{
			name:        "message found with disabled field returns pointer",
			messageCode: "63",
			fieldCode:   "AA",
			wantNil:     false,
			wantCode:    "AA",
		},
		{
			name:        "message with empty Fields list returns nil for any field",
			messageCode: "23",
			fieldCode:   "AE",
			wantNil:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tc.GetFieldConfig(tt.messageCode, tt.fieldCode)
			if tt.wantNil {
				if result != nil {
					t.Errorf("GetFieldConfig(%q, %q) = %+v, want nil", tt.messageCode, tt.fieldCode, result)
				}
				return
			}
			if result == nil {
				t.Fatalf("GetFieldConfig(%q, %q) = nil, want non-nil", tt.messageCode, tt.fieldCode)
			}
			if result.Code != tt.wantCode {
				t.Errorf("GetFieldConfig(%q, %q).Code = %q, want %q", tt.messageCode, tt.fieldCode, result.Code, tt.wantCode)
			}
		})
	}
}

// TestTenantConfig_IsPreferredFirstNameEnabled tests the IsPreferredFirstNameEnabled method
func TestTenantConfig_IsPreferredFirstNameEnabled(t *testing.T) {
	tests := []struct {
		name        string
		tc          *TenantConfig
		messageCode string
		fieldCode   string
		want        bool
	}{
		{
			name: "message not configured (nil field config) returns false",
			tc: &TenantConfig{
				SupportedMessages: []MessageSupport{},
			},
			messageCode: "63",
			fieldCode:   "AE",
			want:        false,
		},
		{
			name: "field configured but PreferredFirstName nil returns false",
			tc: &TenantConfig{
				SupportedMessages: []MessageSupport{
					{Code: "63", Enabled: true, Fields: []FieldConfiguration{
						{Code: "AE", Enabled: true, PreferredFirstName: nil},
					}},
				},
			},
			messageCode: "63",
			fieldCode:   "AE",
			want:        false,
		},
		{
			name: "field configured with PreferredFirstName explicitly true returns true",
			tc: &TenantConfig{
				SupportedMessages: []MessageSupport{
					{Code: "63", Enabled: true, Fields: []FieldConfiguration{
						{Code: "AE", Enabled: true, PreferredFirstName: boolPtr(true)},
					}},
				},
			},
			messageCode: "63",
			fieldCode:   "AE",
			want:        true,
		},
		{
			name: "field configured with PreferredFirstName explicitly false returns false",
			tc: &TenantConfig{
				SupportedMessages: []MessageSupport{
					{Code: "63", Enabled: true, Fields: []FieldConfiguration{
						{Code: "AE", Enabled: true, PreferredFirstName: boolPtr(false)},
					}},
				},
			},
			messageCode: "63",
			fieldCode:   "AE",
			want:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.tc.IsPreferredFirstNameEnabled(tt.messageCode, tt.fieldCode)
			if result != tt.want {
				t.Errorf("IsPreferredFirstNameEnabled(%q, %q) = %v, want %v", tt.messageCode, tt.fieldCode, result, tt.want)
			}
		})
	}
}

// TestTenantConfig_GetPatronItemsLimit tests the GetPatronItemsLimit method
func TestTenantConfig_GetPatronItemsLimit(t *testing.T) {
	tests := []struct {
		name     string
		limit    int
		expected int
	}{
		{
			name:     "Zero value returns max int32 (no limit)",
			limit:    0,
			expected: 2147483647,
		},
		{
			name:     "Negative value returns max int32 (no limit)",
			limit:    -1,
			expected: 2147483647,
		},
		{
			name:     "Configured positive value is returned as-is",
			limit:    25,
			expected: 25,
		},
		{
			name:     "Minimum positive value is returned as-is",
			limit:    1,
			expected: 1,
		},
		{
			name:     "Max int32 value passes through unchanged",
			limit:    2147483647,
			expected: 2147483647,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc := &TenantConfig{
				PatronItemsLimit: tt.limit,
			}

			result := tc.GetPatronItemsLimit()

			if result != tt.expected {
				t.Errorf("GetPatronItemsLimit() = %d, want %d", result, tt.expected)
			}
		})
	}
}

// TestConfig_GetScanPeriod tests the GetScanPeriod method
func TestConfig_GetScanPeriod(t *testing.T) {
	tests := []struct {
		name         string
		scanPeriodMS int
		expectedSec  int
	}{
		{
			name:         "5000ms = 5 seconds",
			scanPeriodMS: 5000,
			expectedSec:  5,
		},
		{
			name:         "10000ms = 10 seconds",
			scanPeriodMS: 10000,
			expectedSec:  10,
		},
		{
			name:         "100ms = 0.1 seconds",
			scanPeriodMS: 100,
			expectedSec:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				ScanPeriod: tt.scanPeriodMS,
			}

			result := cfg.GetScanPeriod()

			if int(result.Seconds()) != tt.expectedSec {
				t.Errorf("GetScanPeriod() = %v seconds, want %d seconds", result.Seconds(), tt.expectedSec)
			}
		})
	}
}
