package parser

import (
	"testing"

	"golang.org/x/text/encoding/charmap"
)

func TestCalculateChecksum(t *testing.T) {
	tests := []struct {
		name           string
		message        string
		sequenceNumber string
		fieldDelimiter string
		expected       string
	}{
		{
			name:           "Simple message",
			message:        "9300",
			sequenceNumber: "0",
			fieldDelimiter: "|",
			expected:       "FD53",
		},
		{
			name:           "Login message",
			message:        "9300|CNjdoe|COpassword",
			sequenceNumber: "1",
			fieldDelimiter: "|",
			expected:       "F622",
		},
		{
			name:           "Patron status",
			message:        "23000202501100815000",
			sequenceNumber: "0",
			fieldDelimiter: "|",
			expected:       "FA41",
		},
	}

	encoding := charmap.ISO8859_1

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checksum, err := CalculateChecksum(tt.message, tt.sequenceNumber, tt.fieldDelimiter, encoding)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if checksum != tt.expected {
				t.Errorf("Expected checksum %s, got %s for message '%s'",
					tt.expected, checksum, tt.message)
			}
		})
	}
}

func TestValidateChecksum(t *testing.T) {
	tests := []struct {
		name    string
		message string
		valid   bool
	}{
		{
			name:    "Valid checksum",
			message: "9300AZFE99",
			valid:   true,
		},
		{
			name:    "Invalid checksum",
			message: "9300AZFFFF",
			valid:   false,
		},
		{
			name:    "Message without checksum",
			message: "9300",
			valid:   false,
		},
	}

	encoding := charmap.ISO8859_1

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ValidateChecksum(tt.message, encoding)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if result.Valid != tt.valid {
				t.Errorf("Expected valid=%v, got valid=%v for message '%s'",
					tt.valid, result.Valid, tt.message)
			}
		})
	}
}

func TestCalculateAndValidateChecksum(t *testing.T) {
	encoding := charmap.ISO8859_1

	message := "9300"
	sequenceNumber := "0"
	fieldDelimiter := "|"

	// Calculate checksum
	checksum, err := CalculateChecksum(message, sequenceNumber, fieldDelimiter, encoding)
	if err != nil {
		t.Fatalf("Unexpected error calculating checksum: %v", err)
	}

	// Build message with checksum
	withChecksum := message + fieldDelimiter + "AY" + sequenceNumber + "AZ" + checksum

	// Validate that the checksum is valid
	result, err := ValidateChecksum(withChecksum, encoding)
	if err != nil {
		t.Fatalf("Unexpected error validating checksum: %v", err)
	}

	if !result.Valid {
		t.Errorf("Expected valid checksum after adding, but validation failed: %s", result.Message)
	}

	// Check that message has correct structure
	if len(checksum) != 4 {
		t.Errorf("Expected checksum length 4, got %d", len(checksum))
	}
}

func TestChecksumWithDifferentEncodings(t *testing.T) {
	message := "9300|CNtest"
	sequenceNumber := "0"
	fieldDelimiter := "|"

	encodings := []struct {
		name     string
		encoding *charmap.Charmap
	}{
		{"ISO8859-1", charmap.ISO8859_1},
		{"IBM850", charmap.CodePage850},
		{"Windows1252", charmap.Windows1252},
	}

	for _, enc := range encodings {
		t.Run(enc.name, func(t *testing.T) {
			checksum, err := CalculateChecksum(message, sequenceNumber, fieldDelimiter, enc.encoding)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if checksum == "" {
				t.Errorf("Expected non-empty checksum for encoding %s", enc.name)
			}

			if len(checksum) != 4 {
				t.Errorf("Expected checksum length 4, got %d", len(checksum))
			}

			// Verify it's valid hex
			for _, c := range checksum {
				if !((c >= '0' && c <= '9') || (c >= 'A' && c <= 'F')) {
					t.Errorf("Invalid hex character '%c' in checksum %s", c, checksum)
				}
			}
		})
	}
}
