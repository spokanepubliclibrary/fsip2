// Package parser implements SIP2 protocol message parsing for fsip2.
//
// This package provides functionality to parse incoming SIP2 protocol messages from
// library self-service kiosks, extracting message types, fixed-length fields, and
// variable-length fields according to the SIP2 specification.
//
// # SIP2 Message Structure
//
// SIP2 messages consist of:
//  1. 2-character message type identifier (e.g., "23" for Patron Status Request)
//  2. Fixed-length fields (position-dependent, vary by message type)
//  3. Variable-length fields (identified by 2-character field codes like "AA", "AB")
//  4. Optional checksum (AY<sequence>AZ<checksum>)
//
// Example SIP2 message:
//
//	2300019700101    084625AO|AA12345678|AD1234|
//	\_/\____________/\__________________________/
//	 |       |                    |
//	 |       |                    Variable-length fields (pipe-delimited)
//	 |       Fixed-length fields (position-specific)
//	 Message type (23 = Patron Status Request)
//
// # Message Parsing Flow
//
// The Parser processes messages in several steps:
//  1. Strip and validate checksum (if enabled)
//  2. Extract 2-character message type
//  3. Extract fixed-length fields based on message type (position-dependent)
//  4. Parse variable-length fields using field delimiter
//  5. Handle multi-value fields (same field code appearing multiple times)
//  6. Return parsed Message struct with all extracted data
//
// # Field Delimiters
//
// The parser supports configurable field delimiters (default: pipe "|"):
//   - Single-byte delimiters: "|", "^", etc.
//   - Multi-byte delimiters: "||", "^|", etc.
//
// # Checksum Validation
//
// When error detection is enabled, the parser:
//   - Extracts the AY<sequence>AZ<checksum> suffix
//   - Validates the checksum against the message content
//   - Returns an error if checksum validation fails
//
// # Message Types and Fixed Fields
//
// Each SIP2 message type has a specific fixed-field format. For example:
//
//   - Message 23 (Patron Status): language (3 chars) + transaction date (18 chars)
//   - Message 11 (Checkout): SC renewal policy (1 char) + no block (1 char) + transaction date (18 chars)
//   - Message 63 (Patron Information): language (3 chars) + transaction date (18 chars) + summary (10 chars)
//
// See ExtractFixedFields() for the complete mapping of message types to fixed fields.
//
// # Usage Example
//
//	parser := NewParser("|", false) // pipe delimiter, checksum disabled
//	msg, err := parser.Parse([]byte("2300019700101    084625AO|AA12345678|"))
//	if err != nil {
//	    // Handle parse error
//	}
//	// Access parsed data
//	patronID := msg.GetField("AA")
//	institutionID := msg.GetField("AO")
//	language := msg.FixedFields["Language"]
package parser
