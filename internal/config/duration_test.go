package config

import (
	"testing"
)

func TestParseDuration(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantValue   int
		wantPeriod  string
		wantErr     bool
		errContains string
	}{
		// Valid formats - uppercase
		{
			name:       "valid 6 months uppercase",
			input:      "6M",
			wantValue:  6,
			wantPeriod: "M",
			wantErr:    false,
		},
		{
			name:       "valid 30 days uppercase",
			input:      "30D",
			wantValue:  30,
			wantPeriod: "D",
			wantErr:    false,
		},
		{
			name:       "valid 1 year uppercase",
			input:      "1Y",
			wantValue:  1,
			wantPeriod: "Y",
			wantErr:    false,
		},

		// Valid formats - lowercase (should be normalized to uppercase)
		{
			name:       "valid 6 months lowercase",
			input:      "6m",
			wantValue:  6,
			wantPeriod: "M",
			wantErr:    false,
		},
		{
			name:       "valid 30 days lowercase",
			input:      "30d",
			wantValue:  30,
			wantPeriod: "D",
			wantErr:    false,
		},
		{
			name:       "valid 1 year lowercase",
			input:      "1y",
			wantValue:  1,
			wantPeriod: "Y",
			wantErr:    false,
		},

		// Valid formats - with whitespace (should be trimmed)
		{
			name:       "valid with leading whitespace",
			input:      " 6M",
			wantValue:  6,
			wantPeriod: "M",
			wantErr:    false,
		},
		{
			name:       "valid with trailing whitespace",
			input:      "6M ",
			wantValue:  6,
			wantPeriod: "M",
			wantErr:    false,
		},
		{
			name:       "valid with both whitespace",
			input:      " 6M ",
			wantValue:  6,
			wantPeriod: "M",
			wantErr:    false,
		},

		// Valid formats - large values
		{
			name:       "valid large value",
			input:      "999Y",
			wantValue:  999,
			wantPeriod: "Y",
			wantErr:    false,
		},
		{
			name:       "valid 12 months",
			input:      "12M",
			wantValue:  12,
			wantPeriod: "M",
			wantErr:    false,
		},
		{
			name:       "valid 365 days",
			input:      "365D",
			wantValue:  365,
			wantPeriod: "D",
			wantErr:    false,
		},

		// Invalid formats - empty
		{
			name:        "empty string",
			input:       "",
			wantErr:     true,
			errContains: "cannot be empty",
		},

		// Invalid formats - wrong period
		{
			name:        "invalid period X",
			input:       "6X",
			wantErr:     true,
			errContains: "invalid duration format",
		},
		{
			name:        "invalid period W",
			input:       "6W",
			wantErr:     true,
			errContains: "invalid duration format",
		},

		// Invalid formats - reversed
		{
			name:        "reversed format",
			input:       "M6",
			wantErr:     true,
			errContains: "invalid duration format",
		},

		// Invalid formats - no number
		{
			name:        "no number",
			input:       "M",
			wantErr:     true,
			errContains: "invalid duration format",
		},

		// Invalid formats - no period
		{
			name:        "no period",
			input:       "6",
			wantErr:     true,
			errContains: "invalid duration format",
		},

		// Invalid formats - text
		{
			name:        "text format",
			input:       "six months",
			wantErr:     true,
			errContains: "invalid duration format",
		},

		// Invalid formats - zero value
		{
			name:        "zero value",
			input:       "0M",
			wantErr:     true,
			errContains: "must be greater than 0",
		},

		// Invalid formats - negative value
		{
			name:        "negative value",
			input:       "-6M",
			wantErr:     true,
			errContains: "invalid duration format",
		},

		// Invalid formats - multiple periods
		{
			name:        "multiple periods",
			input:       "6M12D",
			wantErr:     true,
			errContains: "invalid duration format",
		},

		// Invalid formats - space in middle
		{
			name:        "space in middle",
			input:       "6 M",
			wantErr:     true,
			errContains: "invalid duration format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseDuration(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ParseDuration() error = nil, want error containing %q", tt.errContains)
					return
				}
				if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("ParseDuration() error = %q, want error containing %q", err.Error(), tt.errContains)
				}
				return
			}

			if err != nil {
				t.Errorf("ParseDuration() unexpected error = %v", err)
				return
			}

			if got.Value != tt.wantValue {
				t.Errorf("ParseDuration() Value = %d, want %d", got.Value, tt.wantValue)
			}

			if got.Period != tt.wantPeriod {
				t.Errorf("ParseDuration() Period = %q, want %q", got.Period, tt.wantPeriod)
			}
		})
	}
}

func TestParsedDuration_String(t *testing.T) {
	tests := []struct {
		name     string
		duration *ParsedDuration
		want     string
	}{
		{
			name:     "6 months",
			duration: &ParsedDuration{Value: 6, Period: "M"},
			want:     "6M",
		},
		{
			name:     "30 days",
			duration: &ParsedDuration{Value: 30, Period: "D"},
			want:     "30D",
		},
		{
			name:     "1 year",
			duration: &ParsedDuration{Value: 1, Period: "Y"},
			want:     "1Y",
		},
		{
			name:     "large value",
			duration: &ParsedDuration{Value: 999, Period: "Y"},
			want:     "999Y",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.duration.String()
			if got != tt.want {
				t.Errorf("ParsedDuration.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && stringContains(s, substr)))
}
