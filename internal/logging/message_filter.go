package logging

import (
	"regexp"
	"strings"
)

// MessageLogLevel represents the SIP2 message logging level from tenant config
type MessageLogLevel string

const (
	// LogLevelDebugging logs all messages with no obfuscation
	LogLevelDebugging MessageLogLevel = "Debugging"
	// LogLevelFull logs all messages except 93/94 (login), PINs obfuscated
	LogLevelFull MessageLogLevel = "Full"
	// LogLevelPatron logs only 63/64 (patron info) messages, PINs obfuscated
	LogLevelPatron MessageLogLevel = "Patron"
	// LogLevelNone disables message logging
	LogLevelNone MessageLogLevel = "None"
)

// ShouldLogMessage determines if a SIP2 message should be logged based on the tenant's log level
func ShouldLogMessage(messageCode string, logLevel string) bool {
	level := MessageLogLevel(logLevel)

	switch level {
	case LogLevelNone:
		return false
	case LogLevelDebugging:
		return true
	case LogLevelFull:
		// Log all messages except 93/94 (login)
		return messageCode != "93" && messageCode != "94"
	case LogLevelPatron:
		// Only log 63/64 (patron information)
		return messageCode == "63" || messageCode == "64"
	default:
		// Default to None if invalid/unrecognized
		return false
	}
}

// ObfuscateMessage obfuscates sensitive fields in a SIP2 message based on log level
func ObfuscateMessage(message string, messageCode string, logLevel string) string {
	level := MessageLogLevel(logLevel)

	// No obfuscation for Debugging level
	if level == LogLevelDebugging {
		return message
	}

	// For Full and Patron levels, obfuscate PINs and passwords
	if level == LogLevelFull || level == LogLevelPatron {
		return obfuscatePINsAndPasswords(message)
	}

	return message
}

// obfuscatePINsAndPasswords replaces PIN and password fields with asterisks
func obfuscatePINsAndPasswords(message string) string {
	// SIP2 uses field delimiters (typically |) to separate fields
	// PIN fields are typically:
	// - AD (patron password/PIN in message 93, 23, 63, 11, 09, 29, 65, 35, 37)
	// - CO (login password in message 93)

	// Regex pattern to match field code followed by content until next field or end
	// Pattern: (AD|CO)([^|]*) - captures field code and content
	pinPattern := regexp.MustCompile(`(AD|CO)([^|]*)`)

	result := pinPattern.ReplaceAllStringFunc(message, func(match string) string {
		// Extract field code (2 chars) and value
		if len(match) < 2 {
			return match
		}

		fieldCode := match[:2]
		value := match[2:]

		// If the value is empty, return as-is
		if len(value) == 0 {
			return match
		}

		// Replace with asterisks (same length as original for format preservation)
		obfuscated := strings.Repeat("*", len(value))
		return fieldCode + obfuscated
	})

	return result
}

// ExtractMessageCode extracts the message code from a SIP2 message
// SIP2 messages start with a 2-character code
func ExtractMessageCode(message string) string {
	if len(message) < 2 {
		return ""
	}
	return message[:2]
}
