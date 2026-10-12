package protocol

import (
	"strings"

	"github.com/spokanepubliclibrary/fsip2/internal/helpers"
)

// ConvertDelimiter converts a delimiter string (with escape sequences) to actual bytes
func ConvertDelimiter(delimiter string) string {
	// Handle common escape sequences
	delimiter = strings.ReplaceAll(delimiter, "\\r", "\r")
	delimiter = strings.ReplaceAll(delimiter, "\\n", "\n")
	delimiter = strings.ReplaceAll(delimiter, "\\t", "\t")
	return delimiter
}

// GetMessageDelimiterBytes returns the message delimiter as bytes
func GetMessageDelimiterBytes(delimiter string) []byte {
	return []byte(ConvertDelimiter(delimiter))
}

// GetFieldDelimiterBytes returns the field delimiter as bytes
func GetFieldDelimiterBytes(delimiter string) []byte {
	return []byte(delimiter)
}

// SplitFields splits a message into fields using the specified delimiter
func SplitFields(message string, delimiter string) []string {
	return strings.Split(message, delimiter)
}

// ParseFields parses a message into a map of field code to value
// Fields are in the format: CODE<value>DELIMITER
// Example: AA1234567|AB9876543|
func ParseFields(message string, fieldDelimiter string) map[string]string {
	fields := make(map[string]string)

	parts := SplitFields(message, fieldDelimiter)

	for _, part := range parts {
		if len(part) < 2 {
			continue
		}

		// First 2 characters are the field code
		fieldCode := part[0:2]
		fieldValue := ""

		if len(part) > 2 {
			fieldValue = part[2:]
		}

		fields[fieldCode] = fieldValue
	}

	return fields
}

// ParseMultiValueFields parses fields that can have multiple values (like AS, AT, AU)
// Returns a map where each field code maps to a slice of values
func ParseMultiValueFields(message string, fieldDelimiter string) map[string][]string {
	fields := make(map[string][]string)

	parts := SplitFields(message, fieldDelimiter)

	for _, part := range parts {
		if len(part) < 2 {
			continue
		}

		// First 2 characters are the field code
		fieldCode := part[0:2]
		fieldValue := ""

		if len(part) > 2 {
			fieldValue = part[2:]
		}

		// Append to slice (allows multiple values for same field code)
		fields[fieldCode] = append(fields[fieldCode], fieldValue)
	}

	return fields
}

// GetField retrieves a field value from a parsed fields map
func GetField(fields map[string]string, fieldCode string) string {
	if value, ok := fields[fieldCode]; ok {
		return value
	}
	return ""
}

// GetMultiValueField retrieves multiple values for a field code
func GetMultiValueField(fields map[string][]string, fieldCode string) []string {
	if values, ok := fields[fieldCode]; ok {
		return values
	}
	return []string{}
}

// BuildField constructs a field string from code and value, omitting the field
// entirely when the value is empty OR sanitizes to empty.
//
// This covers the optional-field case on its own, so do not add a separate
// "optional field" wrapper around it. Such a wrapper could only early-return on
// value == "", which would make it byte-identical to this function for every
// input: the empty check below already yields "" for an empty value.
//
// The value is sanitized BEFORE the empty check: embedded control characters
// (notably newlines from FOLIO data) would break SIP2's line-oriented framing.
// Sanitizing first means a value consisting only of control characters
// collapses to "" and the field is omitted entirely, rather than emitting a
// field whose value is a raw control character. That ordering is load-bearing:
// checking emptiness first would let a control-character-only value reach the
// wire, so do not reorder it.
//
// Sanitization uses helpers.SanitizeFieldValue, so an occurrence of the field
// delimiter inside the value (which would otherwise inject a spurious field
// separator) is neutralized to a space as well.
func BuildField(fieldCode string, value string, delimiter string) string {
	value = helpers.SanitizeFieldValue(value, delimiter)
	if value == "" {
		return ""
	}
	return fieldCode + value + delimiter
}

// BuildRequiredField constructs a field string that is ALWAYS emitted, even
// when its value is empty.
//
// This is deliberately NOT the same as BuildField, and the two must not be
// merged:
//
//   - BuildField sanitizes and then OMITS the field when the sanitized value is
//     empty. That is correct for fields whose absence is meaningful (optional
//     fields, and fields where an empty value carries no information).
//   - BuildRequiredField sanitizes and then ALWAYS emits
//     fieldCode + sanitizedValue + delimiter, so an empty value produces a
//     present-but-blank field (e.g. "CS|").
//
// The difference exists because parts of the SIP2 spec require certain response
// fields to be included "even if blank" — notably the checkin response (message
// 10) fields AQ, AP, CK, CH, CS, CV and CT. Omitting those fields breaks clients
// that key off field position/presence, which is a worse protocol regression
// than an empty value. Those call sites previously concatenated raw strings to
// get this always-emit behavior, which let unsanitized FOLIO data (e.g. a
// callNumber of "741.5 KHAN\n741.5 KHAN") reach the wire and break SIP2's
// line-oriented framing. Use this builder instead of raw concatenation.
//
// Sanitization uses helpers.SanitizeFieldValue, so a value of only control
// characters collapses to "" and the field is still emitted, blank, and an
// occurrence of the field delimiter inside the value is neutralized to a space
// instead of injecting a spurious field separator.
func BuildRequiredField(fieldCode string, value string, delimiter string) string {
	return fieldCode + helpers.SanitizeFieldValue(value, delimiter) + delimiter
}

// BuildPrefixedField constructs a field in the DELIMITER-FIRST shape
// delimiter + fieldCode + sanitizedValue, e.g. "|AE" + value.
//
// It exists because the hand-rolled response builders in internal/handlers and
// internal/server emit the delimiter BEFORE the field code, as
// fmt.Sprintf("|XX%s", v) or content += "|XX" + v. BuildField produces the
// opposite shape (fieldCode + value + delimiter), so it cannot be used at those
// call sites: swapping one in would move the delimiter to the wrong side of the
// value and produce malformed messages.
//
// This builder ALWAYS emits, even when the value is empty or sanitizes to empty.
// That is deliberate: the raw concatenations it replaces always emitted too, and
// at those call sites field PRESENCE is governed by the surrounding if
// statements, not by the builder. Making it omit empty values would silently drop
// fields the callers intend to send.
//
// Do not merge this with BuildField or BuildRequiredField. All three differ in
// shape or in emit policy: BuildField is value-then-delimiter and omits empty,
// BuildRequiredField is value-then-delimiter and always emits, and this one is
// delimiter-then-code and always emits.
func BuildPrefixedField(fieldCode string, value string, delimiter string) string {
	return delimiter + fieldCode + helpers.SanitizeFieldValue(value, delimiter)
}

// BuildFixedField constructs a fixed-length field (no delimiter).
//
// Uses SanitizeFixedWidthString (strict 1:1 control-char-to-space replacement),
// never SanitizeString: fixed-width SIP2 fields carry semantically meaningful
// spaces (e.g. the 14-char patron status block), so trimming or collapsing
// would shift every subsequent character position. Length is preserved exactly,
// so the pad/truncate math below is unaffected.
func BuildFixedField(value string, length int) string {
	value = helpers.SanitizeFixedWidthString(value)
	if len(value) >= length {
		return value[:length]
	}
	// Pad with spaces if too short
	return value + strings.Repeat(" ", length-len(value))
}

// BuildYNField constructs a Y/N field
func BuildYNField(value bool) string {
	if value {
		return "Y"
	}
	return "N"
}

// ParseYNField parses a Y/N field to boolean
func ParseYNField(value string) bool {
	return value == "Y" || value == "y"
}
