package helpers

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
)

// GenerateID generates a random ID string
func GenerateID() string {
	bytes := make([]byte, 16)
	rand.Read(bytes) // crypto/rand.Read never fails on Go 1.20+ (guaranteed by spec)
	return hex.EncodeToString(bytes)
}

// TruncateString truncates a string to at most maxLen characters (runes).
//
// Truncation happens on character boundaries, never in the middle of a
// multi-byte UTF-8 sequence. FOLIO bibliographic data routinely contains
// diacritics and CJK text, so slicing by byte offset would emit a corrupt
// partial character into the SIP2 response. No ellipsis or other truncation
// marker is appended, because SIP2 field widths are fixed budgets and a marker
// would consume characters of real data.
func TruncateString(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	// Fast path: a string of maxLen bytes or fewer cannot hold more than maxLen
	// runes, so no truncation is possible and no conversion is needed.
	if len(s) <= maxLen {
		return s
	}
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen])
}

// PadRight pads a string to the right with spaces
func PadRight(s string, length int) string {
	if len(s) >= length {
		return s[:length]
	}
	return s + strings.Repeat(" ", length-len(s))
}

// PadLeft pads a string to the left with spaces
func PadLeft(s string, length int) string {
	if len(s) >= length {
		return s[:length]
	}
	return strings.Repeat(" ", length-len(s)) + s
}

// isControlRune reports whether r is a C0 control character (0x00-0x1F) or DEL
// (0x7F). The plain space (0x20) is not a control character.
func isControlRune(r rune) bool {
	return r <= 0x1F || r == 0x7F
}

// SanitizeString makes a value safe for use in a variable-length SIP2 field.
//
// SIP2 is a line-oriented protocol, so any embedded control character (notably
// CR or LF arriving from FOLIO data such as a call number) breaks message
// framing. Every maximal run of control characters - along with any spaces
// immediately adjacent to that run - collapses to a single space when the run
// sits in the interior of the value; a run at the very start or very end
// contributes no space at all. Pre-existing runs of spaces that contain no
// control characters are preserved, since they may be meaningful data, and that
// includes leading and trailing spaces that are not adjacent to a control run.
// Non-ASCII text passes through unchanged.
//
// Invariant: if the input contains no control characters, the output is
// byte-identical to the input.
func SanitizeString(s string) string {
	result := make([]rune, 0, len(s))
	inRun := false      // currently inside a run of control characters
	pendingSep := false // a run was seen; emit its space only if real text follows

	for _, r := range s {
		if isControlRune(r) {
			if !inRun {
				// Absorb spaces that immediately precede the run.
				for len(result) > 0 && result[len(result)-1] == ' ' {
					result = result[:len(result)-1]
				}
				inRun = true
				// The separator is emitted lazily: a run is only "interior"
				// once a later non-control character actually arrives, and a
				// trailing run can only be recognised at end of string.
				pendingSep = true
			}
			continue
		}
		if inRun {
			// Absorb spaces that immediately follow the run.
			if r == ' ' {
				continue
			}
			inRun = false
		}
		if pendingSep {
			pendingSep = false
			// Nothing emitted yet means the run was leading, so it
			// contributes no space.
			if len(result) > 0 {
				result = append(result, ' ')
			}
		}
		result = append(result, r)
	}

	return string(result)
}

// SanitizeFieldValue makes a value safe for use in a variable-length SIP2 field
// whose fields are separated by delim.
//
// It handles the two distinct ways FOLIO data corrupts a SIP2 message:
//
//   - The field delimiter itself (default "|", a printable character, so outside
//     SanitizeString's control-character scope) appearing inside a value injects
//     a spurious field separator and shifts every following field.
//   - Control characters break SIP2's line-oriented framing; SanitizeString
//     handles those.
//
// Each occurrence of delim is replaced by a space rather than stripped, because
// stripping merges tokens that were visually distinct ("741.5|KHAN" would become
// "741.5KHAN"); replacing with a space preserves them and matches the
// control-character policy already used by SanitizeString. Escaping is not an
// option: SIP2 defines no escape mechanism for a delimiter inside a field value.
//
// The delim != "" guard is load-bearing, not defensive boilerplate.
// strings.ReplaceAll(s, "", " ") inserts a space between every single character,
// which would corrupt every field of every message, and an empty configured
// delimiter is reachable in practice.
//
// Delimiter replacement runs BEFORE SanitizeString, and that ordering matters.
// For "A|\nB" the delimiter pass yields "A \nB", and SanitizeString's run
// collapsing then absorbs the space adjacent to the control run, giving "A B".
// The reverse order would give "A  B" with a stray double space.
//
// Invariant: a value containing no control characters and no occurrence of the
// configured delimiter is returned byte-identical.
func SanitizeFieldValue(value, delim string) string {
	if delim != "" {
		value = strings.ReplaceAll(value, delim, " ")
	}
	return SanitizeString(value)
}

// SanitizeFixedWidthString makes a value safe for use in a fixed-length SIP2
// field without changing its width.
//
// Each control character (0x00-0x1F and 0x7F) is replaced by exactly one space;
// nothing is collapsed, trimmed or removed. Fixed-length SIP2 fields - such as
// the 14-character patron status block and the fixed date and language fields -
// carry semantically meaningful spaces, so for any ASCII input the output length
// always equals the input length and field positions are preserved.
func SanitizeFixedWidthString(s string) string {
	var result strings.Builder
	result.Grow(len(s))

	for _, r := range s {
		if isControlRune(r) {
			result.WriteRune(' ')
			continue
		}
		result.WriteRune(r)
	}

	return result.String()
}

// CoalesceString returns the first non-empty string
func CoalesceString(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// DefaultString returns the value or default if empty
func DefaultString(value, defaultValue string) string {
	if value == "" {
		return defaultValue
	}
	return value
}

// BoolToYN converts a boolean to Y/N string
func BoolToYN(b bool) string {
	if b {
		return "Y"
	}
	return "N"
}

// YNToBool converts a Y/N string to boolean
func YNToBool(s string) bool {
	return strings.ToUpper(s) == "Y"
}

// Contains checks if a slice contains a string
func Contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// FormatCurrency formats a float as currency string
func FormatCurrency(amount float64) string {
	return fmt.Sprintf("%.2f", amount)
}
