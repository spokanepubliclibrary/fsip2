package config

import (
	"testing"
)

func TestIsFieldEnabled(t *testing.T) {
	tests := []struct {
		name        string
		config      *TenantConfig
		messageCode string
		fieldCode   string
		want        bool
	}{
		{
			name: "Field explicitly enabled",
			config: &TenantConfig{
				SupportedMessages: []MessageSupport{
					{
						Code:    "17",
						Enabled: true,
						Fields: []FieldConfiguration{
							{Code: "AJ", Enabled: true},
							{Code: "EA", Enabled: false},
						},
					},
				},
			},
			messageCode: "17",
			fieldCode:   "AJ",
			want:        true,
		},
		{
			name: "Field explicitly disabled",
			config: &TenantConfig{
				SupportedMessages: []MessageSupport{
					{
						Code:    "17",
						Enabled: true,
						Fields: []FieldConfiguration{
							{Code: "AJ", Enabled: true},
							{Code: "EA", Enabled: false},
						},
					},
				},
			},
			messageCode: "17",
			fieldCode:   "EA",
			want:        false,
		},
		{
			name: "Field not configured (default enabled)",
			config: &TenantConfig{
				SupportedMessages: []MessageSupport{
					{
						Code:    "17",
						Enabled: true,
						Fields: []FieldConfiguration{
							{Code: "AJ", Enabled: true},
						},
					},
				},
			},
			messageCode: "17",
			fieldCode:   "AQ",
			want:        true,
		},
		{
			name: "No field configuration (default enabled)",
			config: &TenantConfig{
				SupportedMessages: []MessageSupport{
					{
						Code:    "17",
						Enabled: true,
					},
				},
			},
			messageCode: "17",
			fieldCode:   "AJ",
			want:        true,
		},
		{
			name: "Message not found (default enabled)",
			config: &TenantConfig{
				SupportedMessages: []MessageSupport{
					{
						Code:    "11",
						Enabled: true,
					},
				},
			},
			messageCode: "17",
			fieldCode:   "AJ",
			want:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.config.IsFieldEnabled(tt.messageCode, tt.fieldCode)
			if got != tt.want {
				t.Errorf("IsFieldEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMapCirculationStatus(t *testing.T) {
	tests := []struct {
		name        string
		config      *TenantConfig
		folioStatus string
		want        string
	}{
		{
			name: "Available status with custom mapping",
			config: &TenantConfig{
				CirculationStatusMapping: map[string]string{
					"Available": "03",
				},
			},
			folioStatus: "Available",
			want:        "03",
		},
		{
			name: "Checked out status with custom mapping",
			config: &TenantConfig{
				CirculationStatusMapping: map[string]string{
					"Checked out": "04",
				},
			},
			folioStatus: "Checked out",
			want:        "04",
		},
		{
			name: "Unknown status with custom default",
			config: &TenantConfig{
				CirculationStatusMapping: map[string]string{
					"Available": "03",
					"default":   "99",
				},
			},
			folioStatus: "Unknown Status",
			want:        "99",
		},
		{
			name: "No mapping configuration - use built-in defaults",
			config: &TenantConfig{
				CirculationStatusMapping: map[string]string{},
			},
			folioStatus: "Available",
			want:        "03",
		},
		{
			name:        "Empty config - use built-in defaults",
			config:      &TenantConfig{},
			folioStatus: "Checked out",
			want:        "04",
		},
		{
			name:        "Lost status - use built-in defaults",
			config:      &TenantConfig{},
			folioStatus: "Lost and paid",
			want:        "12",
		},
		{
			name:        "Missing status - use built-in defaults",
			config:      &TenantConfig{},
			folioStatus: "Missing",
			want:        "13",
		},
		{
			name:        "Unknown status - fallback to Other",
			config:      &TenantConfig{},
			folioStatus: "Some Unknown Status",
			want:        "01",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.config.MapCirculationStatus(tt.folioStatus)
			if got != tt.want {
				t.Errorf("MapCirculationStatus(%q) = %v, want %v", tt.folioStatus, got, tt.want)
			}
		})
	}
}

func TestGetDefaultCirculationStatusMapping(t *testing.T) {
	tests := []struct {
		folioStatus string
		want        string
	}{
		{"Available", "03"},
		{"Checked out", "04"},
		{"In process", "06"},
		{"Awaiting pickup", "08"},
		{"In transit", "10"},
		{"Claimed returned", "11"},
		{"Lost and paid", "12"},
		{"Aged to lost", "12"},
		{"Declared lost", "12"},
		{"Missing", "13"},
		{"Withdrawn", "01"},
		{"On order", "02"},
		{"Paged", "08"},
		{"Unknown Status", "01"}, // Fallback
	}

	for _, tt := range tests {
		t.Run(tt.folioStatus, func(t *testing.T) {
			got := getDefaultCirculationStatusMapping(tt.folioStatus)
			if got != tt.want {
				t.Errorf("getDefaultCirculationStatusMapping(%q) = %v, want %v", tt.folioStatus, got, tt.want)
			}
		})
	}
}

func TestGetPaymentMethod(t *testing.T) {
	tests := []struct {
		name   string
		config *TenantConfig
		want   string
	}{
		{
			name: "Custom payment method configured",
			config: &TenantConfig{
				PaymentMethod: "Cash",
			},
			want: "Cash",
		},
		{
			name: "Credit card configured",
			config: &TenantConfig{
				PaymentMethod: "Credit card",
			},
			want: "Credit card",
		},
		{
			name:   "Empty payment method - use default",
			config: &TenantConfig{},
			want:   "Credit card",
		},
		{
			name: "Empty string payment method - use default",
			config: &TenantConfig{
				PaymentMethod: "",
			},
			want: "Credit card",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.config.GetPaymentMethod()
			if got != tt.want {
				t.Errorf("GetPaymentMethod() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetAcceptBulkPayment(t *testing.T) {
	tests := []struct {
		name   string
		config *TenantConfig
		want   bool
	}{
		{
			name: "Bulk payment enabled",
			config: &TenantConfig{
				AcceptBulkPayment: true,
			},
			want: true,
		},
		{
			name: "Bulk payment disabled",
			config: &TenantConfig{
				AcceptBulkPayment: false,
			},
			want: false,
		},
		{
			name:   "Default - bulk payment disabled",
			config: &TenantConfig{},
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.config.GetAcceptBulkPayment()
			if got != tt.want {
				t.Errorf("GetAcceptBulkPayment() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetNotifyPatron(t *testing.T) {
	tests := []struct {
		name   string
		config *TenantConfig
		want   bool
	}{
		{
			name: "Notify patron enabled",
			config: &TenantConfig{
				NotifyPatron: true,
			},
			want: true,
		},
		{
			name: "Notify patron disabled",
			config: &TenantConfig{
				NotifyPatron: false,
			},
			want: false,
		},
		{
			name:   "Default - notify patron disabled",
			config: &TenantConfig{},
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.config.GetNotifyPatron()
			if got != tt.want {
				t.Errorf("GetNotifyPatron() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRollingRenewalConfig_Validate(t *testing.T) {
	tests := []struct {
		name        string
		config      *RollingRenewalConfig
		wantErr     bool
		errContains string
	}{
		{
			name: "valid configuration - all fields",
			config: &RollingRenewalConfig{
				Enabled:        true,
				RenewWithin:    "6M",
				ExtendFor:      "6Y",
				ExtendExpired:  true,
				DryRun:         false,
				SelectPatrons:  true,
				AllowedPatrons: []string{"uuid-1", "uuid-2"},
			},
			wantErr: false,
		},
		{
			name: "valid configuration - lowercase periods",
			config: &RollingRenewalConfig{
				Enabled:       true,
				RenewWithin:   "6m",
				ExtendFor:     "6y",
				ExtendExpired: false,
				DryRun:        true,
			},
			wantErr: false,
		},
		{
			name: "valid configuration - days period",
			config: &RollingRenewalConfig{
				Enabled:     true,
				RenewWithin: "30D",
				ExtendFor:   "365D",
			},
			wantErr: false,
		},
		{
			name: "valid configuration - disabled",
			config: &RollingRenewalConfig{
				Enabled: false,
				// Other fields can be invalid when disabled
			},
			wantErr: false,
		},
		{
			name: "valid configuration - selectPatrons false with empty allowedPatrons",
			config: &RollingRenewalConfig{
				Enabled:       true,
				RenewWithin:   "6M",
				ExtendFor:     "6Y",
				SelectPatrons: false,
				// AllowedPatrons can be empty when selectPatrons is false
			},
			wantErr: false,
		},
		{
			name: "invalid - missing renewWithin",
			config: &RollingRenewalConfig{
				Enabled:   true,
				ExtendFor: "6Y",
			},
			wantErr:     true,
			errContains: "renewWithin is required",
		},
		{
			name: "invalid - missing extendFor",
			config: &RollingRenewalConfig{
				Enabled:     true,
				RenewWithin: "6M",
			},
			wantErr:     true,
			errContains: "extendFor is required",
		},
		{
			name: "invalid - invalid renewWithin format",
			config: &RollingRenewalConfig{
				Enabled:     true,
				RenewWithin: "6X",
				ExtendFor:   "6Y",
			},
			wantErr:     true,
			errContains: "invalid renewWithin duration",
		},
		{
			name: "invalid - invalid extendFor format",
			config: &RollingRenewalConfig{
				Enabled:     true,
				RenewWithin: "6M",
				ExtendFor:   "invalid",
			},
			wantErr:     true,
			errContains: "invalid extendFor duration",
		},
		{
			name: "invalid - selectPatrons true with empty allowedPatrons",
			config: &RollingRenewalConfig{
				Enabled:        true,
				RenewWithin:    "6M",
				ExtendFor:      "6Y",
				SelectPatrons:  true,
				AllowedPatrons: []string{},
			},
			wantErr:     true,
			errContains: "allowedPatrons cannot be empty",
		},
		{
			name: "invalid - selectPatrons true with nil allowedPatrons",
			config: &RollingRenewalConfig{
				Enabled:        true,
				RenewWithin:    "6M",
				ExtendFor:      "6Y",
				SelectPatrons:  true,
				AllowedPatrons: nil,
			},
			wantErr:     true,
			errContains: "allowedPatrons cannot be empty",
		},
		{
			name: "invalid - zero value in renewWithin",
			config: &RollingRenewalConfig{
				Enabled:     true,
				RenewWithin: "0M",
				ExtendFor:   "6Y",
			},
			wantErr:     true,
			errContains: "invalid renewWithin duration",
		},
		{
			name: "invalid - negative value in extendFor",
			config: &RollingRenewalConfig{
				Enabled:     true,
				RenewWithin: "6M",
				ExtendFor:   "-6Y",
			},
			wantErr:     true,
			errContains: "invalid extendFor duration",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()

			if tt.wantErr {
				if err == nil {
					t.Errorf("Validate() error = nil, want error containing %q", tt.errContains)
					return
				}
				if tt.errContains != "" && !stringContains(err.Error(), tt.errContains) {
					t.Errorf("Validate() error = %q, want error containing %q", err.Error(), tt.errContains)
				}
				return
			}

			if err != nil {
				t.Errorf("Validate() unexpected error = %v", err)
			}
		})
	}
}

func TestTenantConfig_ValidateRollingRenewals(t *testing.T) {
	tests := []struct {
		name    string
		config  *TenantConfig
		wantErr bool
	}{
		{
			name: "valid configuration",
			config: &TenantConfig{
				RollingRenewals: &RollingRenewalConfig{
					Enabled:     true,
					RenewWithin: "6M",
					ExtendFor:   "6Y",
				},
			},
			wantErr: false,
		},
		{
			name: "nil rolling renewals config",
			config: &TenantConfig{
				RollingRenewals: nil,
			},
			wantErr: false,
		},
		{
			name: "invalid rolling renewals config",
			config: &TenantConfig{
				RollingRenewals: &RollingRenewalConfig{
					Enabled: true,
					// Missing required fields
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.ValidateRollingRenewals()

			if tt.wantErr && err == nil {
				t.Error("ValidateRollingRenewals() error = nil, want error")
			}

			if !tt.wantErr && err != nil {
				t.Errorf("ValidateRollingRenewals() unexpected error = %v", err)
			}
		})
	}
}

func TestTenantConfig_IsRollingRenewalEnabled(t *testing.T) {
	tests := []struct {
		name   string
		config *TenantConfig
		want   bool
	}{
		{
			name: "enabled",
			config: &TenantConfig{
				RollingRenewals: &RollingRenewalConfig{
					Enabled: true,
				},
			},
			want: true,
		},
		{
			name: "disabled",
			config: &TenantConfig{
				RollingRenewals: &RollingRenewalConfig{
					Enabled: false,
				},
			},
			want: false,
		},
		{
			name: "nil config",
			config: &TenantConfig{
				RollingRenewals: nil,
			},
			want: false,
		},
		{
			name:   "empty tenant config",
			config: &TenantConfig{},
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.config.IsRollingRenewalEnabled()
			if got != tt.want {
				t.Errorf("IsRollingRenewalEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTenantConfig_GetRollingRenewalConfig(t *testing.T) {
	tests := []struct {
		name   string
		config *TenantConfig
		want   *RollingRenewalConfig
	}{
		{
			name: "config present",
			config: &TenantConfig{
				RollingRenewals: &RollingRenewalConfig{
					Enabled:     true,
					RenewWithin: "6M",
					ExtendFor:   "6Y",
				},
			},
			want: &RollingRenewalConfig{
				Enabled:     true,
				RenewWithin: "6M",
				ExtendFor:   "6Y",
			},
		},
		{
			name: "config nil",
			config: &TenantConfig{
				RollingRenewals: nil,
			},
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.config.GetRollingRenewalConfig()

			if tt.want == nil && got != nil {
				t.Errorf("GetRollingRenewalConfig() = %v, want nil", got)
				return
			}

			if tt.want != nil && got == nil {
				t.Error("GetRollingRenewalConfig() = nil, want non-nil")
				return
			}

			if tt.want != nil && got != nil {
				if got.Enabled != tt.want.Enabled || got.RenewWithin != tt.want.RenewWithin || got.ExtendFor != tt.want.ExtendFor {
					t.Errorf("GetRollingRenewalConfig() = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

// Helper function for string contains check
func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestTenantConfig_GetTimeoutPeriod(t *testing.T) {
	tests := []struct {
		name     string
		timeout  int
		expected string
	}{
		{
			name:     "default value when not set",
			timeout:  0,
			expected: "030", // Default: 30 seconds
		},
		{
			name:     "negative value defaults to 30",
			timeout:  -1,
			expected: "030",
		},
		{
			name:     "custom value 5 seconds",
			timeout:  5,
			expected: "005",
		},
		{
			name:     "custom value 120 seconds",
			timeout:  120,
			expected: "120",
		},
		{
			name:     "maximum value 999",
			timeout:  999,
			expected: "999",
		},
		{
			name:     "value exceeding maximum capped at 999",
			timeout:  1500,
			expected: "999",
		},
		{
			name:     "single digit formatted correctly",
			timeout:  3,
			expected: "003",
		},
		{
			name:     "double digit formatted correctly",
			timeout:  45,
			expected: "045",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc := &TenantConfig{
				TimeoutPeriod: tt.timeout,
			}
			result := tc.GetTimeoutPeriod()
			if result != tt.expected {
				t.Errorf("GetTimeoutPeriod() = %s, expected %s", result, tt.expected)
			}
			// Verify length is always 3
			if len(result) != 3 {
				t.Errorf("GetTimeoutPeriod() length = %d, expected 3", len(result))
			}
		})
	}
}

func TestTenantConfig_GetRetriesAllowed(t *testing.T) {
	tests := []struct {
		name     string
		retries  int
		expected string
	}{
		{
			name:     "default value when not set",
			retries:  0,
			expected: "003", // Default: 3 retries
		},
		{
			name:     "negative value defaults to 3",
			retries:  -1,
			expected: "003",
		},
		{
			name:     "custom value 1 retry",
			retries:  1,
			expected: "001",
		},
		{
			name:     "custom value 10 retries",
			retries:  10,
			expected: "010",
		},
		{
			name:     "maximum value 999",
			retries:  999,
			expected: "999",
		},
		{
			name:     "value exceeding maximum capped at 999",
			retries:  2000,
			expected: "999",
		},
		{
			name:     "single digit formatted correctly",
			retries:  5,
			expected: "005",
		},
		{
			name:     "double digit formatted correctly",
			retries:  50,
			expected: "050",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc := &TenantConfig{
				RetriesAllowed: tt.retries,
			}
			result := tc.GetRetriesAllowed()
			if result != tt.expected {
				t.Errorf("GetRetriesAllowed() = %s, expected %s", result, tt.expected)
			}
			// Verify length is always 3
			if len(result) != 3 {
				t.Errorf("GetRetriesAllowed() length = %d, expected 3", len(result))
			}
		})
	}
}

func TestTenantConfig_BuildSupportedMessages(t *testing.T) {
	tests := []struct {
		name              string
		supportedMessages []MessageSupport
		expected          string
		description       string
	}{
		{
			name:              "no messages configured",
			supportedMessages: []MessageSupport{},
			expected:          "NNNNNNNNNNNNNNNN",
			description:       "All positions should be N when no messages configured",
		},
		{
			name: "all messages enabled",
			supportedMessages: []MessageSupport{
				{Code: "23", Enabled: true},  // Position 1
				{Code: "11", Enabled: true},  // Position 2
				{Code: "09", Enabled: true},  // Position 3
				{Code: "01", Enabled: true},  // Position 4 - Always N (not implemented)
				{Code: "99", Enabled: true},  // Position 5
				{Code: "97", Enabled: true},  // Position 6
				{Code: "93", Enabled: true},  // Position 7
				{Code: "63", Enabled: true},  // Position 8
				{Code: "35", Enabled: true},  // Position 9
				{Code: "37", Enabled: true},  // Position 10
				{Code: "17", Enabled: true},  // Position 11
				{Code: "19", Enabled: true},  // Position 12
				{Code: "25", Enabled: true},  // Position 13 - Always N (not implemented)
				{Code: "15", Enabled: true},  // Position 14
				{Code: "29", Enabled: true},  // Position 15
				{Code: "65", Enabled: true},  // Position 16
			},
			expected:    "YYYNYYYYYYYYNYYY",
			description: "Block Patron (pos 4) and Patron Enable (pos 13) always N, rest Y",
		},
		{
			name: "common configuration",
			supportedMessages: []MessageSupport{
				{Code: "23", Enabled: false}, // Patron Status Request - N
				{Code: "11", Enabled: true},  // Checkout - Y
				{Code: "09", Enabled: true},  // Checkin - Y
				{Code: "99", Enabled: true},  // SC/ACS Status - Y
				{Code: "97", Enabled: true},  // Request Resend - Y
				{Code: "93", Enabled: true},  // Login - Y
				{Code: "63", Enabled: true},  // Patron Information - Y
				{Code: "35", Enabled: true},  // End Patron Session - Y
				{Code: "37", Enabled: true},  // Fee Paid - Y
				{Code: "17", Enabled: false}, // Item Information - N
				{Code: "19", Enabled: false}, // Item Status Update - N
				{Code: "15", Enabled: false}, // Hold - N
				{Code: "29", Enabled: false}, // Renew - N
				{Code: "65", Enabled: false}, // Renew All - N
			},
			expected:    "NYYNYYYYYYNNNNNN",
			description: "Typical configuration with basic operations enabled",
		},
		{
			name: "block patron always returns N even when enabled",
			supportedMessages: []MessageSupport{
				{Code: "01", Enabled: true}, // Block Patron - Should still be N
			},
			expected:    "NNNNNNNNNNNNNNNN",
			description: "Position 4 (Block Patron) always N even when configured as enabled",
		},
		{
			name: "patron enable always returns N even when enabled",
			supportedMessages: []MessageSupport{
				{Code: "25", Enabled: true}, // Patron Enable - Should still be N
			},
			expected:    "NNNNNNNNNNNNNNNN",
			description: "Position 13 (Patron Enable) always N even when configured as enabled",
		},
		{
			name: "only checkout and checkin enabled",
			supportedMessages: []MessageSupport{
				{Code: "11", Enabled: true}, // Checkout
				{Code: "09", Enabled: true}, // Checkin
			},
			expected:    "NYYNNNNNNNNNNNNN",
			description: "Only positions 2 and 3 should be Y",
		},
		{
			name: "example from documentation",
			supportedMessages: []MessageSupport{
				{Code: "23", Enabled: false}, // Position 1 - N
				{Code: "11", Enabled: true},  // Position 2 - Y
				{Code: "09", Enabled: true},  // Position 3 - Y
				{Code: "99", Enabled: true},  // Position 5 - Y
				{Code: "97", Enabled: true},  // Position 6 - Y
				{Code: "93", Enabled: true},  // Position 7 - Y
				{Code: "63", Enabled: true},  // Position 8 - Y
				{Code: "35", Enabled: true},  // Position 9 - Y
				{Code: "37", Enabled: true},  // Position 10 - Y
			},
			expected:    "NYYNYYYYYYNNNNNN",
			description: "NYYNYYYYYYNNNNNN from documentation example",
		},
		{
			name: "mixed enabled and disabled",
			supportedMessages: []MessageSupport{
				{Code: "23", Enabled: true},  // Position 1 - Y
				{Code: "11", Enabled: false}, // Position 2 - N
				{Code: "09", Enabled: true},  // Position 3 - Y
				{Code: "99", Enabled: false}, // Position 5 - N
				{Code: "97", Enabled: true},  // Position 6 - Y
				{Code: "93", Enabled: false}, // Position 7 - N
				{Code: "63", Enabled: true},  // Position 8 - Y
				{Code: "29", Enabled: true},  // Position 15 - Y
				{Code: "65", Enabled: false}, // Position 16 - N
			},
			expected:    "YNYNNYNYNNNNNNYN",
			description: "Mixed configuration with various messages enabled/disabled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc := &TenantConfig{
				SupportedMessages: tt.supportedMessages,
			}
			result := tc.BuildSupportedMessages()
			if result != tt.expected {
				t.Errorf("BuildSupportedMessages() = %s, expected %s - %s", result, tt.expected, tt.description)
			}
			// Verify length is always 16
			if len(result) != 16 {
				t.Errorf("BuildSupportedMessages() length = %d, expected 16", len(result))
			}
			// Verify only Y or N characters
			for i, ch := range result {
				if ch != 'Y' && ch != 'N' {
					t.Errorf("BuildSupportedMessages() position %d contains invalid character '%c', expected Y or N", i+1, ch)
				}
			}
		})
	}
}

func TestTenantConfig_BuildSupportedMessages_PositionMapping(t *testing.T) {
	// Test each position individually to verify correct mapping
	positionTests := []struct {
		code        string
		position    int
		messageName string
	}{
		{"23", 1, "Patron Status Request"},
		{"11", 2, "Checkout"},
		{"09", 3, "Checkin"},
		{"01", 4, "Block Patron"},
		{"99", 5, "SC/ACS Status"},
		{"97", 6, "Request SC/ACS Resend"},
		{"93", 7, "Login"},
		{"63", 8, "Patron Information"},
		{"35", 9, "End Patron Session"},
		{"37", 10, "Fee Paid"},
		{"17", 11, "Item Information"},
		{"19", 12, "Item Status Update"},
		{"25", 13, "Patron Enable"},
		{"15", 14, "Hold"},
		{"29", 15, "Renew"},
		{"65", 16, "Renew All"},
	}

	for _, tt := range positionTests {
		t.Run(tt.messageName, func(t *testing.T) {
			tc := &TenantConfig{
				SupportedMessages: []MessageSupport{
					{Code: tt.code, Enabled: true},
				},
			}
			result := tc.BuildSupportedMessages()

			// Special case for Block Patron (01) and Patron Enable (25) - always N
			expectedChar := 'Y'
			if tt.code == "01" || tt.code == "25" {
				expectedChar = 'N'
			}

			if result[tt.position-1] != byte(expectedChar) {
				t.Errorf("%s (code %s) at position %d: got '%c', expected '%c'",
					tt.messageName, tt.code, tt.position, result[tt.position-1], expectedChar)
			}
		})
	}
}

func TestTenantConfig_GetClaimedReturnedResolution(t *testing.T) {
	tests := []struct {
		name     string
		config   *TenantConfig
		expected string
	}{
		{
			name: "patron resolution - lowercase",
			config: &TenantConfig{
				ClaimedReturnedResolution: "patron",
			},
			expected: "patron",
		},
		{
			name: "patron resolution - uppercase",
			config: &TenantConfig{
				ClaimedReturnedResolution: "PATRON",
			},
			expected: "patron",
		},
		{
			name: "patron resolution - mixed case",
			config: &TenantConfig{
				ClaimedReturnedResolution: "Patron",
			},
			expected: "patron",
		},
		{
			name: "library resolution - lowercase",
			config: &TenantConfig{
				ClaimedReturnedResolution: "library",
			},
			expected: "library",
		},
		{
			name: "library resolution - uppercase",
			config: &TenantConfig{
				ClaimedReturnedResolution: "LIBRARY",
			},
			expected: "library",
		},
		{
			name: "library resolution - mixed case",
			config: &TenantConfig{
				ClaimedReturnedResolution: "Library",
			},
			expected: "library",
		},
		{
			name: "none resolution - lowercase",
			config: &TenantConfig{
				ClaimedReturnedResolution: "none",
			},
			expected: "none",
		},
		{
			name: "none resolution - uppercase",
			config: &TenantConfig{
				ClaimedReturnedResolution: "NONE",
			},
			expected: "none",
		},
		{
			name: "empty string - defaults to none",
			config: &TenantConfig{
				ClaimedReturnedResolution: "",
			},
			expected: "none",
		},
		{
			name: "invalid value - defaults to none",
			config: &TenantConfig{
				ClaimedReturnedResolution: "invalid",
			},
			expected: "none",
		},
		{
			name: "random invalid value - defaults to none",
			config: &TenantConfig{
				ClaimedReturnedResolution: "xyz123",
			},
			expected: "none",
		},
		{
			name:     "nil config - defaults to none",
			config:   &TenantConfig{},
			expected: "none",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.config.GetClaimedReturnedResolution()
			if result != tt.expected {
				t.Errorf("GetClaimedReturnedResolution() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestTenantConfig_MapClaimedReturnedResolutionToFOLIO(t *testing.T) {
	tests := []struct {
		name     string
		config   *TenantConfig
		expected string
	}{
		{
			name: "patron resolution maps to 'Returned by patron'",
			config: &TenantConfig{
				ClaimedReturnedResolution: "patron",
			},
			expected: "Returned by patron",
		},
		{
			name: "patron resolution - case insensitive",
			config: &TenantConfig{
				ClaimedReturnedResolution: "PATRON",
			},
			expected: "Returned by patron",
		},
		{
			name: "library resolution maps to 'Found by library'",
			config: &TenantConfig{
				ClaimedReturnedResolution: "library",
			},
			expected: "Found by library",
		},
		{
			name: "library resolution - case insensitive",
			config: &TenantConfig{
				ClaimedReturnedResolution: "LIBRARY",
			},
			expected: "Found by library",
		},
		{
			name: "none resolution maps to empty string",
			config: &TenantConfig{
				ClaimedReturnedResolution: "none",
			},
			expected: "",
		},
		{
			name: "empty string maps to empty string",
			config: &TenantConfig{
				ClaimedReturnedResolution: "",
			},
			expected: "",
		},
		{
			name: "invalid value maps to empty string",
			config: &TenantConfig{
				ClaimedReturnedResolution: "invalid",
			},
			expected: "",
		},
		{
			name:     "unset config maps to empty string",
			config:   &TenantConfig{},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.config.MapClaimedReturnedResolutionToFOLIO()
			if result != tt.expected {
				t.Errorf("MapClaimedReturnedResolutionToFOLIO() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		wantErr     bool
		errContains string
	}{
		{
			name: "valid configuration - all fields",
			config: &Config{
				Port:               6443,
				HealthCheckPort:    8081,
				OkapiURL:           "https://okapi.example.com",
				TokenCacheCapacity: 100,
				ScanPeriod:         300000,
				LogLevel:           "info",
			},
			wantErr: false,
		},
		{
			name: "valid configuration - debug log level",
			config: &Config{
				Port:               6443,
				HealthCheckPort:    8081,
				OkapiURL:           "https://okapi.example.com",
				TokenCacheCapacity: 100,
				ScanPeriod:         300000,
				LogLevel:           "debug",
			},
			wantErr: false,
		},
		{
			name: "valid configuration - warn log level",
			config: &Config{
				Port:               6443,
				HealthCheckPort:    8081,
				OkapiURL:           "https://okapi.example.com",
				TokenCacheCapacity: 100,
				ScanPeriod:         300000,
				LogLevel:           "warn",
			},
			wantErr: false,
		},
		{
			name: "valid configuration - error log level",
			config: &Config{
				Port:               6443,
				HealthCheckPort:    8081,
				OkapiURL:           "https://okapi.example.com",
				TokenCacheCapacity: 100,
				ScanPeriod:         300000,
				LogLevel:           "error",
			},
			wantErr: false,
		},
		{
			name: "valid configuration - case insensitive log level",
			config: &Config{
				Port:               6443,
				HealthCheckPort:    8081,
				OkapiURL:           "https://okapi.example.com",
				TokenCacheCapacity: 100,
				ScanPeriod:         300000,
				LogLevel:           "INFO",
			},
			wantErr: false,
		},
		{
			name: "valid configuration - with TLS disabled",
			config: &Config{
				Port:               6443,
				HealthCheckPort:    8081,
				OkapiURL:           "https://okapi.example.com",
				TokenCacheCapacity: 100,
				ScanPeriod:         300000,
				LogLevel:           "info",
				TLS: &TLSConfig{
					Enabled: false,
				},
			},
			wantErr: false,
		},
		{
			name: "valid configuration - with TLS enabled",
			config: &Config{
				Port:               6443,
				HealthCheckPort:    8081,
				OkapiURL:           "https://okapi.example.com",
				TokenCacheCapacity: 100,
				ScanPeriod:         300000,
				LogLevel:           "info",
				TLS: &TLSConfig{
					Enabled:  true,
					CertFile: "/path/to/cert.pem",
					KeyFile:  "/path/to/key.pem",
				},
			},
			wantErr: false,
		},
		{
			name: "valid configuration - zero scan period",
			config: &Config{
				Port:               6443,
				HealthCheckPort:    8081,
				OkapiURL:           "https://okapi.example.com",
				TokenCacheCapacity: 100,
				ScanPeriod:         0,
				LogLevel:           "info",
			},
			wantErr: false,
		},
		{
			name: "invalid - port too low",
			config: &Config{
				Port:               0,
				HealthCheckPort:    8081,
				OkapiURL:           "https://okapi.example.com",
				TokenCacheCapacity: 100,
				ScanPeriod:         300000,
				LogLevel:           "info",
			},
			wantErr:     true,
			errContains: "invalid port",
		},
		{
			name: "invalid - port too high",
			config: &Config{
				Port:               65536,
				HealthCheckPort:    8081,
				OkapiURL:           "https://okapi.example.com",
				TokenCacheCapacity: 100,
				ScanPeriod:         300000,
				LogLevel:           "info",
			},
			wantErr:     true,
			errContains: "invalid port",
		},
		{
			name: "invalid - negative port",
			config: &Config{
				Port:               -1,
				HealthCheckPort:    8081,
				OkapiURL:           "https://okapi.example.com",
				TokenCacheCapacity: 100,
				ScanPeriod:         300000,
				LogLevel:           "info",
			},
			wantErr:     true,
			errContains: "invalid port",
		},
		{
			name: "invalid - health check port too low",
			config: &Config{
				Port:               6443,
				HealthCheckPort:    0,
				OkapiURL:           "https://okapi.example.com",
				TokenCacheCapacity: 100,
				ScanPeriod:         300000,
				LogLevel:           "info",
			},
			wantErr:     true,
			errContains: "invalid health check port",
		},
		{
			name: "invalid - health check port too high",
			config: &Config{
				Port:               6443,
				HealthCheckPort:    70000,
				OkapiURL:           "https://okapi.example.com",
				TokenCacheCapacity: 100,
				ScanPeriod:         300000,
				LogLevel:           "info",
			},
			wantErr:     true,
			errContains: "invalid health check port",
		},
		{
			name: "invalid - empty OkapiURL",
			config: &Config{
				Port:               6443,
				HealthCheckPort:    8081,
				OkapiURL:           "",
				TokenCacheCapacity: 100,
				ScanPeriod:         300000,
				LogLevel:           "info",
			},
			wantErr:     true,
			errContains: "OkapiURL is required",
		},
		{
			name: "invalid - malformed OkapiURL",
			config: &Config{
				Port:               6443,
				HealthCheckPort:    8081,
				OkapiURL:           "://invalid-url",
				TokenCacheCapacity: 100,
				ScanPeriod:         300000,
				LogLevel:           "info",
			},
			wantErr:     true,
			errContains: "invalid OkapiURL",
		},
		{
			name: "invalid - token cache capacity zero",
			config: &Config{
				Port:               6443,
				HealthCheckPort:    8081,
				OkapiURL:           "https://okapi.example.com",
				TokenCacheCapacity: 0,
				ScanPeriod:         300000,
				LogLevel:           "info",
			},
			wantErr:     true,
			errContains: "token cache capacity must be positive",
		},
		{
			name: "invalid - token cache capacity negative",
			config: &Config{
				Port:               6443,
				HealthCheckPort:    8081,
				OkapiURL:           "https://okapi.example.com",
				TokenCacheCapacity: -1,
				ScanPeriod:         300000,
				LogLevel:           "info",
			},
			wantErr:     true,
			errContains: "token cache capacity must be positive",
		},
		{
			name: "invalid - scan period negative",
			config: &Config{
				Port:               6443,
				HealthCheckPort:    8081,
				OkapiURL:           "https://okapi.example.com",
				TokenCacheCapacity: 100,
				ScanPeriod:         -1,
				LogLevel:           "info",
			},
			wantErr:     true,
			errContains: "scan period must be non-negative",
		},
		{
			name: "invalid - log level",
			config: &Config{
				Port:               6443,
				HealthCheckPort:    8081,
				OkapiURL:           "https://okapi.example.com",
				TokenCacheCapacity: 100,
				ScanPeriod:         300000,
				LogLevel:           "invalid",
			},
			wantErr:     true,
			errContains: "invalid log level",
		},
		{
			name: "invalid - TLS enabled but missing cert file",
			config: &Config{
				Port:               6443,
				HealthCheckPort:    8081,
				OkapiURL:           "https://okapi.example.com",
				TokenCacheCapacity: 100,
				ScanPeriod:         300000,
				LogLevel:           "info",
				TLS: &TLSConfig{
					Enabled: true,
					KeyFile: "/path/to/key.pem",
				},
			},
			wantErr:     true,
			errContains: "TLS cert file is required",
		},
		{
			name: "invalid - TLS enabled but missing key file",
			config: &Config{
				Port:               6443,
				HealthCheckPort:    8081,
				OkapiURL:           "https://okapi.example.com",
				TokenCacheCapacity: 100,
				ScanPeriod:         300000,
				LogLevel:           "info",
				TLS: &TLSConfig{
					Enabled:  true,
					CertFile: "/path/to/cert.pem",
				},
			},
			wantErr:     true,
			errContains: "TLS key file is required",
		},
		{
			name: "invalid - TLS enabled but missing both files",
			config: &Config{
				Port:               6443,
				HealthCheckPort:    8081,
				OkapiURL:           "https://okapi.example.com",
				TokenCacheCapacity: 100,
				ScanPeriod:         300000,
				LogLevel:           "info",
				TLS: &TLSConfig{
					Enabled: true,
				},
			},
			wantErr:     true,
			errContains: "TLS cert file is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()

			if tt.wantErr {
				if err == nil {
					t.Errorf("Validate() error = nil, want error containing %q", tt.errContains)
					return
				}
				if tt.errContains != "" && !stringContains(err.Error(), tt.errContains) {
					t.Errorf("Validate() error = %q, want error containing %q", err.Error(), tt.errContains)
				}
				return
			}

			if err != nil {
				t.Errorf("Validate() unexpected error = %v", err)
			}
		})
	}
}
