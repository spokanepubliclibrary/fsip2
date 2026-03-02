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

// TestTenantConfig_GetMessageDelimiterBytes tests the GetMessageDelimiterBytes method
func TestTenantConfig_GetMessageDelimiterBytes(t *testing.T) {
	tests := []struct {
		name      string
		delimiter string
		expected  []byte
	}{
		{
			name:      "CR delimiter escaped",
			delimiter: "\\r",
			expected:  []byte{'\r'},
		},
		{
			name:      "CRLF delimiter escaped",
			delimiter: "\\r\\n",
			expected:  []byte{'\r', '\n'},
		},
		{
			name:      "LF delimiter escaped",
			delimiter: "\\n",
			expected:  []byte{'\n'},
		},
		{
			name:      "Empty delimiter returns empty",
			delimiter: "",
			expected:  []byte{},
		},
		{
			name:      "Custom delimiter",
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
