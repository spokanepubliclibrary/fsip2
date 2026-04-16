package parser

import (
	"testing"

	"golang.org/x/text/encoding"
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

// TestValidateChecksum_MatchesCalculate_UTF8 verifies that a checksum produced by
// CalculateChecksum is accepted by ValidateChecksum when using UTF-8 (encoding.Nop).
func TestValidateChecksum_MatchesCalculate_UTF8(t *testing.T) {
	enc := encoding.Nop
	message := "9300|CNjdoe|COpassword"
	sequenceNumber := "1"
	fieldDelimiter := "|"

	checksum, err := CalculateChecksum(message, sequenceNumber, fieldDelimiter, enc)
	if err != nil {
		t.Fatalf("CalculateChecksum returned unexpected error: %v", err)
	}

	fullMessage := message + fieldDelimiter + "AY" + sequenceNumber + "AZ" + checksum

	result, err := ValidateChecksum(fullMessage, enc)
	if err != nil {
		t.Fatalf("ValidateChecksum returned unexpected error: %v", err)
	}
	if !result.Valid {
		t.Errorf("expected ValidateChecksum to accept checksum produced by CalculateChecksum (UTF-8), got Valid=false; message: %s", result.Message)
	}
}

// TestValidateChecksum_MatchesCalculate_Latin1 verifies that a checksum produced by
// CalculateChecksum is accepted by ValidateChecksum when using ISO-8859-1 and a
// message containing non-ASCII Latin characters.
func TestValidateChecksum_MatchesCalculate_Latin1(t *testing.T) {
	enc := charmap.ISO8859_1
	// "café" contains é (U+00E9), encoded as 0xE9 in ISO-8859-1.
	message := "9300|CNcafé|COpassword"
	sequenceNumber := "2"
	fieldDelimiter := "|"

	checksum, err := CalculateChecksum(message, sequenceNumber, fieldDelimiter, enc)
	if err != nil {
		t.Fatalf("CalculateChecksum returned unexpected error: %v", err)
	}

	fullMessage := message + fieldDelimiter + "AY" + sequenceNumber + "AZ" + checksum

	result, err := ValidateChecksum(fullMessage, enc)
	if err != nil {
		t.Fatalf("ValidateChecksum returned unexpected error: %v", err)
	}
	if !result.Valid {
		t.Errorf("expected ValidateChecksum to accept checksum produced by CalculateChecksum (ISO-8859-1/Latin-1), got Valid=false; message: %s", result.Message)
	}
}

// TestValidateChecksum_ConsistencyAcrossSegments verifies that for a multi-field
// message the checksum computed internally by ValidateChecksum matches what
// CalculateChecksum produces — i.e. the single-encode fix keeps both functions in sync.
func TestValidateChecksum_ConsistencyAcrossSegments(t *testing.T) {
	enc := encoding.Nop
	tests := []struct {
		name           string
		message        string
		sequenceNumber string
		fieldDelimiter string
	}{
		{
			name:           "multi-field patron status message",
			message:        "23000202501100815000|AAtestpatron|ADpassword|BLY",
			sequenceNumber: "3",
			fieldDelimiter: "|",
		},
		{
			name:           "checkout message",
			message:        "11YN20260101    120000          |AAjdoe|AB1234567890",
			sequenceNumber: "0",
			fieldDelimiter: "|",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checksum, err := CalculateChecksum(tt.message, tt.sequenceNumber, tt.fieldDelimiter, enc)
			if err != nil {
				t.Fatalf("CalculateChecksum error: %v", err)
			}

			fullMessage := tt.message + tt.fieldDelimiter + "AY" + tt.sequenceNumber + "AZ" + checksum

			result, err := ValidateChecksum(fullMessage, enc)
			if err != nil {
				t.Fatalf("ValidateChecksum error: %v", err)
			}
			if !result.Valid {
				t.Errorf("round-trip consistency failed: ValidateChecksum rejected checksum from CalculateChecksum; detail: %s", result.Message)
			}
		})
	}
}

// TestValidateChecksum_RejectsInvalidChecksum verifies that ValidateChecksum returns
// Valid=false when the trailing checksum bytes have been corrupted.
func TestValidateChecksum_RejectsInvalidChecksum(t *testing.T) {
	enc := encoding.Nop
	message := "9300|CNjdoe|COpassword"
	sequenceNumber := "1"
	fieldDelimiter := "|"

	checksum, err := CalculateChecksum(message, sequenceNumber, fieldDelimiter, enc)
	if err != nil {
		t.Fatalf("CalculateChecksum returned unexpected error: %v", err)
	}

	// Corrupt the checksum by flipping the last hex digit.
	corruptedChecksum := checksum[:3] + func() string {
		last := checksum[3]
		if last == '0' {
			return "1"
		}
		return "0"
	}()

	fullMessage := message + fieldDelimiter + "AY" + sequenceNumber + "AZ" + corruptedChecksum

	result, err := ValidateChecksum(fullMessage, enc)
	if err != nil {
		t.Fatalf("ValidateChecksum returned unexpected error on corrupted input: %v", err)
	}
	if result.Valid {
		t.Errorf("expected ValidateChecksum to reject corrupted checksum %q (original %q), but got Valid=true", corruptedChecksum, checksum)
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
