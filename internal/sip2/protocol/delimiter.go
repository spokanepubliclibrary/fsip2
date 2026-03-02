package protocol

import (
	"strings"
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

// BuildField constructs a field string from code and value
func BuildField(fieldCode string, value string, delimiter string) string {
	if value == "" {
		return ""
	}
	return fieldCode + value + delimiter
}

// BuildOptionalField constructs a field string only if value is not empty
func BuildOptionalField(fieldCode string, value string, delimiter string) string {
	if value == "" {
		return ""
	}
	return BuildField(fieldCode, value, delimiter)
}

// BuildFixedField constructs a fixed-length field (no delimiter)
func BuildFixedField(value string, length int) string {
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
