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

// TruncateString truncates a string to a maximum length
func TruncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
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

// SanitizeString removes or replaces problematic characters
func SanitizeString(s string) string {
	// Remove null bytes
	s = strings.ReplaceAll(s, "\x00", "")
	// Remove other control characters except newlines and tabs
	var result strings.Builder
	for _, r := range s {
		if r >= 32 || r == '\n' || r == '\t' {
			result.WriteRune(r)
		}
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
