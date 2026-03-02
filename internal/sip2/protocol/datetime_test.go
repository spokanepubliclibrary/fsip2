package protocol

import (
	"testing"
	"time"
)

func TestFormatSIP2DateTime(t *testing.T) {
	tests := []struct {
		name     string
		t        time.Time
		timezone string
		want     string
	}{
		{
			name:     "zero time returns empty string",
			t:        time.Time{},
			timezone: "UTC",
			want:     "",
		},
		{
			name:     "known time UTC",
			t:        time.Date(2024, 3, 15, 10, 30, 45, 0, time.UTC),
			timezone: "UTC",
			want:     "20240315    103045",
		},
		{
			name:     "invalid timezone falls back to UTC",
			t:        time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			timezone: "Invalid/Zone",
			want:     "20240101    000000",
		},
		{
			name:     "America/New_York timezone",
			t:        time.Date(2024, 6, 15, 15, 0, 0, 0, time.UTC),
			timezone: "America/New_York",
			want:     "20240615    110000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatSIP2DateTime(tt.t, tt.timezone)
			if got != tt.want {
				t.Errorf("FormatSIP2DateTime() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFormatSIP2Date(t *testing.T) {
	tests := []struct {
		name     string
		t        time.Time
		timezone string
		want     string
	}{
		{
			name:     "zero time returns empty string",
			t:        time.Time{},
			timezone: "UTC",
			want:     "",
		},
		{
			name:     "known date UTC",
			t:        time.Date(2024, 3, 15, 10, 30, 45, 0, time.UTC),
			timezone: "UTC",
			want:     "20240315",
		},
		{
			name:     "invalid timezone falls back to UTC",
			t:        time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC),
			timezone: "Bad/Zone",
			want:     "20241231",
		},
		{
			name:     "timezone shifts date",
			t:        time.Date(2024, 6, 16, 2, 0, 0, 0, time.UTC),
			timezone: "America/New_York",
			want:     "20240615",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatSIP2Date(tt.t, tt.timezone)
			if got != tt.want {
				t.Errorf("FormatSIP2Date() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseSIP2DateTime(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		timezone string
		want     time.Time
		wantErr  bool
	}{
		{
			name:     "empty string returns zero time",
			s:        "",
			timezone: "UTC",
			want:     time.Time{},
			wantErr:  false,
		},
		{
			name:     "valid datetime UTC",
			s:        "20240315    103045",
			timezone: "UTC",
			want:     time.Date(2024, 3, 15, 10, 30, 45, 0, time.UTC),
			wantErr:  false,
		},
		{
			name:     "invalid format returns error",
			s:        "not-a-date",
			timezone: "UTC",
			wantErr:  true,
		},
		{
			name:     "invalid timezone falls back to UTC",
			s:        "20240101    000000",
			timezone: "Bad/Zone",
			want:     time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseSIP2DateTime(tt.s, tt.timezone)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ParseSIP2DateTime(%q) expected error, got nil", tt.s)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseSIP2DateTime(%q) unexpected error: %v", tt.s, err)
			}
			if !got.Equal(tt.want) {
				t.Errorf("ParseSIP2DateTime(%q) = %v, want %v", tt.s, got, tt.want)
			}
		})
	}
}

func TestParseSIP2Date(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		timezone string
		want     time.Time
		wantErr  bool
	}{
		{
			name:     "empty string returns zero time",
			s:        "",
			timezone: "UTC",
			want:     time.Time{},
			wantErr:  false,
		},
		{
			name:     "valid date UTC",
			s:        "20240315",
			timezone: "UTC",
			want:     time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC),
			wantErr:  false,
		},
		{
			name:     "invalid format returns error",
			s:        "not-a-date",
			timezone: "UTC",
			wantErr:  true,
		},
		{
			name:     "invalid timezone falls back to UTC",
			s:        "20241231",
			timezone: "Bad/Zone",
			want:     time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC),
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseSIP2Date(tt.s, tt.timezone)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ParseSIP2Date(%q) expected error, got nil", tt.s)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseSIP2Date(%q) unexpected error: %v", tt.s, err)
			}
			if !got.Equal(tt.want) {
				t.Errorf("ParseSIP2Date(%q) = %v, want %v", tt.s, got, tt.want)
			}
		})
	}
}

func TestFormatParseRoundTrip(t *testing.T) {
	original := time.Date(2024, 7, 4, 14, 30, 0, 0, time.UTC)
	timezone := "UTC"

	formatted := FormatSIP2DateTime(original, timezone)
	parsed, err := ParseSIP2DateTime(formatted, timezone)
	if err != nil {
		t.Fatalf("ParseSIP2DateTime: %v", err)
	}
	if !parsed.Equal(original) {
		t.Errorf("round-trip: got %v, want %v", parsed, original)
	}
}

func TestCurrentSIP2DateTime(t *testing.T) {
	before := time.Now()
	result := CurrentSIP2DateTime("UTC")
	after := time.Now()

	if len(result) != len(SIP2DateTimeFormat) {
		t.Errorf("CurrentSIP2DateTime length = %d, want %d", len(result), len(SIP2DateTimeFormat))
	}

	parsed, err := ParseSIP2DateTime(result, "UTC")
	if err != nil {
		t.Fatalf("CurrentSIP2DateTime returned unparseable string %q: %v", result, err)
	}

	if parsed.Before(before.Truncate(time.Second)) || parsed.After(after.Add(time.Second)) {
		t.Errorf("CurrentSIP2DateTime returned time %v outside range [%v, %v]", parsed, before, after)
	}
}

func TestCurrentSIP2Date(t *testing.T) {
	result := CurrentSIP2Date("UTC")
	if len(result) != len(SIP2DateFormat) {
		t.Errorf("CurrentSIP2Date length = %d, want %d", len(result), len(SIP2DateFormat))
	}

	_, err := ParseSIP2Date(result, "UTC")
	if err != nil {
		t.Fatalf("CurrentSIP2Date returned unparseable string %q: %v", result, err)
	}
}
