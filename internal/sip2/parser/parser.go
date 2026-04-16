package parser

import (
	"fmt"
	"strings"

	"github.com/spokanepubliclibrary/fsip2/internal/config"
	"github.com/spokanepubliclibrary/fsip2/internal/sip2/protocol"
)

// Message represents a parsed SIP2 message
type Message struct {
	Code             MessageCode
	RawMessage       string
	Fields           map[string]string
	MultiValueFields map[string][]string
	SequenceNumber   string
	ChecksumValid    bool
}

// Parser handles parsing of SIP2 messages
type Parser struct {
	config *config.TenantConfig
}

// NewParser creates a new SIP2 parser
func NewParser(cfg *config.TenantConfig) *Parser {
	return &Parser{
		config: cfg,
	}
}

// Parse parses a raw SIP2 message into a structured Message object.
//
// This is the core parsing function that converts a raw SIP2 protocol message string
// into a structured Message with typed fields. It handles all 13 SIP2 message types.
//
// SIP2 Message Structure:
//
//	<2-char code><fixed fields><variable fields>[<checksum>]
//
// Example message breakdown:
//
//	Raw: "2300019700101    084625AO|AA12345678|AD1234|AY1AZFB6B"
//	Code: "23" (Patron Status Request)
//	Fixed: "00019700101    084625" (language + transaction date)
//	Variable: "AO|AA12345678|AD1234|" (institution ID, patron ID, password)
//	Checksum: "AY1AZFB6B" (sequence number 1, checksum FB6B)
//
// Parsing Algorithm (5 steps):
//
//  1. CHECKSUM VALIDATION (if error detection enabled):
//     - Extract AY<sequence>AZ<checksum> suffix
//     - Calculate expected checksum from message content
//     - Compare calculated vs. received checksum
//     - Return error if checksum invalid
//     - Strip checksum fields from message for further parsing
//
//  2. EXTRACT MESSAGE CODE:
//     - Read first 2 characters as message type identifier
//     - Message codes: "09"=Checkin, "11"=Checkout, "23"=PatronStatus, etc.
//
//  3. EXTRACT FIXED-LENGTH FIELDS:
//     - Each message type has a different fixed-field format
//     - Fixed fields are position-dependent (no field codes)
//     - Length and positions are defined by SIP2 specification
//     - Examples:
//     * Message 23: 3-char language + 18-char transaction date
//     * Message 11: 1-char renewal policy + 1-char no block + 18-char transaction date
//     * Message 63: 3-char language + 18-char date + 10-char summary
//     - Extracted into FixedFields map by field name
//
//  4. PARSE VARIABLE-LENGTH FIELDS:
//     - Remaining content consists of field code + data + delimiter
//     - Format: <2-char code><data><delimiter>
//     - Example: "AO|AA12345|AB123456|" with delimiter "|"
//     * Field AO (institution ID): "" (empty)
//     * Field AA (patron ID): "12345"
//     * Field AB (item ID): "123456"
//     - Split by field delimiter (configurable, default "|")
//     - Extract field code (first 2 chars of each segment)
//     - Extract field value (remaining chars after code)
//
//  5. HANDLE MULTI-VALUE FIELDS:
//     - Some fields can appear multiple times (e.g., BD for patron addresses)
//     - First occurrence stored in Fields map
//     - Subsequent occurrences appended to MultiValueFields map
//     - Example: "BDAddress1|BDAddress2|" creates MultiValueFields["BD"] = ["Address1", "Address2"]
//
// Returns:
//   - *Message: Parsed message with Code, Fields, MultiValueFields, ChecksumValid
//   - error: Parse errors (invalid format, checksum failure, etc.)
//
// Example usage:
//
//	parser := NewParser(tenantConfig)
//	msg, err := parser.Parse("2300019700101    084625AO|AA12345678|")
//	if err != nil { return err }
//	patronID := msg.GetField("AA")  // Returns "12345678"
//	institutionID := msg.GetField("AO")  // Returns ""
func (p *Parser) Parse(rawMessage string) (*Message, error) {
	if len(rawMessage) < 2 {
		return nil, fmt.Errorf("message too short: must be at least 2 characters")
	}

	msg := &Message{
		RawMessage: rawMessage,
	}

	// STEP 1: CHECKSUM VALIDATION
	// If error detection is enabled, validate the AY/AZ checksum fields
	// and strip them from the message before further parsing
	if p.config.ErrorDetectionEnabled {
		encoder, err := protocol.GetEncoder(p.config.Charset)
		if err != nil {
			return nil, fmt.Errorf("failed to get encoder: %w", err)
		}

		checksumResult, err := ValidateChecksum(rawMessage, encoder)
		if err != nil {
			return nil, fmt.Errorf("checksum validation error: %w", err)
		}

		msg.ChecksumValid = checksumResult.Valid
		msg.SequenceNumber = checksumResult.SequenceNumber

		if !checksumResult.Valid {
			return nil, fmt.Errorf("checksum validation failed: %s", checksumResult.Message)
		}

		// Strip checksum fields (AY<seq>AZ<checksum>) for further parsing
		rawMessage = StripChecksum(rawMessage)
	}

	// STEP 2: EXTRACT MESSAGE CODE
	// The first 2 characters identify the message type (e.g., "23", "11", "63")
	msg.Code = MessageCode(rawMessage[0:2])

	// STEP 3: EXTRACT MESSAGE CONTENT
	// Everything after the 2-character code is the message content
	// This contains both fixed-length and variable-length fields
	messageContent := rawMessage[2:]

	// STEP 4: PARSE FIXED-LENGTH FIELDS
	// Each message type has specific fixed fields at specific positions
	// The number and position of fixed fields varies by message type
	var fieldsStart int
	switch msg.Code {
	case LoginRequest:
		// 93<UID_ALGO><PWD_ALGO>
		if len(messageContent) >= 2 {
			fieldsStart = 2
		}
	case PatronStatusRequest:
		// 23<language><transaction_date>
		if len(messageContent) >= 21 { // 3 + 18
			fieldsStart = 21
		}
	case CheckoutRequest:
		// 11<SC_renewal_policy><no_block><transaction_date><nb_due_date>
		// nb_due_date is optional, but if message is long enough, assume it's present
		if len(messageContent) >= 38 { // 1 + 1 + 18 + 18 (with optional nb_due_date)
			fieldsStart = 38
		} else if len(messageContent) >= 20 { // 1 + 1 + 18 (without nb_due_date)
			fieldsStart = 20
		}
	case CheckinRequest:
		// 09<no_block><transaction_date><return_date>
		if len(messageContent) >= 37 { // 1 + 18 + 18
			fieldsStart = 37
		}
	case PatronInformationRequest:
		// 63<language><transaction_date><summary>
		if len(messageContent) >= 31 { // 3 + 18 + 10
			fieldsStart = 31
		}
	case ItemInformationRequest:
		// 17<transaction_date>
		if len(messageContent) >= 18 {
			fieldsStart = 18
		}
	case RenewRequest:
		// 29<third_party_allowed><no_block><transaction_date><nb_due_date>
		// nb_due_date is optional, but if message is long enough, assume it's present
		if len(messageContent) >= 38 { // 1 + 1 + 18 + 18 (with optional nb_due_date)
			fieldsStart = 38
		} else if len(messageContent) >= 20 { // 1 + 1 + 18 (without nb_due_date)
			fieldsStart = 20
		}
	case RenewAllRequest:
		// 65<transaction_date>
		if len(messageContent) >= 18 {
			fieldsStart = 18
		}
	case EndPatronSessionRequest:
		// 35<transaction_date>
		if len(messageContent) >= 18 {
			fieldsStart = 18
		}
	case FeePaidRequest:
		// 37<transaction_date><fee_type><payment_type><currency_type>
		if len(messageContent) >= 25 { // 18 + 2 + 2 + 3
			fieldsStart = 25
		}
	case ItemStatusUpdateRequest:
		// 19<transaction_date>
		if len(messageContent) >= 18 {
			fieldsStart = 18
		}
	case SCStatus:
		// 99<status_code><max_print_width><protocol_version>
		if len(messageContent) >= 8 { // 1 + 3 + 4
			fieldsStart = 8
		}
	default:
		// For other messages, variable-length fields start immediately
		fieldsStart = 0
	}

	// Step 5: Parse variable-length fields
	if fieldsStart < len(messageContent) {
		variableFields := messageContent[fieldsStart:]

		// Parse both single and multi-value fields
		msg.Fields = protocol.ParseFields(variableFields, p.config.FieldDelimiter)
		msg.MultiValueFields = protocol.ParseMultiValueFields(variableFields, p.config.FieldDelimiter)
	} else {
		msg.Fields = make(map[string]string)
		msg.MultiValueFields = make(map[string][]string)
	}

	// Step 6: Extract and merge fixed-length fields into Fields map
	fixedFields := p.ExtractFixedFields(msg)
	for key, value := range fixedFields {
		msg.Fields[key] = value
	}

	return msg, nil
}

// GetField retrieves a field value from the message
func (m *Message) GetField(fieldCode FieldCode) string {
	return protocol.GetField(m.Fields, string(fieldCode))
}

// GetMultiValueField retrieves multiple values for a field code
func (m *Message) GetMultiValueField(fieldCode FieldCode) []string {
	return protocol.GetMultiValueField(m.MultiValueFields, string(fieldCode))
}

// HasField checks if a field exists in the message
func (m *Message) HasField(fieldCode FieldCode) bool {
	_, ok := m.Fields[string(fieldCode)]
	return ok
}

// ExtractFixedFields extracts fixed-length fields based on message type
// Returns a map of field names to values
func (p *Parser) ExtractFixedFields(msg *Message) map[string]string {
	fixed := make(map[string]string)

	if len(msg.RawMessage) < 2 {
		return fixed
	}

	messageContent := msg.RawMessage[2:] // Skip message code

	switch msg.Code {
	case LoginRequest:
		// 93<UID_ALGO><PWD_ALGO>
		if len(messageContent) >= 2 {
			fixed["uid_algorithm"] = messageContent[0:1]
			fixed["pwd_algorithm"] = messageContent[1:2]
		}

	case PatronStatusRequest:
		// 23<language><transaction_date>
		if len(messageContent) >= 21 {
			fixed["language"] = messageContent[0:3]
			fixed["transaction_date"] = messageContent[3:21]
		}

	case CheckoutRequest:
		// 11<SC_renewal_policy><no_block><transaction_date><nb_due_date>
		if len(messageContent) >= 20 {
			fixed["sc_renewal_policy"] = messageContent[0:1]
			fixed["no_block"] = messageContent[1:2]
			fixed["transaction_date"] = messageContent[2:20]
			if len(messageContent) >= 38 {
				fixed["nb_due_date"] = messageContent[20:38]
			}
		}

	case CheckinRequest:
		// 09<no_block><transaction_date><return_date>
		if len(messageContent) >= 37 {
			fixed["no_block"] = messageContent[0:1]
			fixed["transaction_date"] = messageContent[1:19]
			fixed["return_date"] = messageContent[19:37]
		}

	case PatronInformationRequest:
		// 63<language><transaction_date><summary>
		if len(messageContent) >= 31 {
			fixed["language"] = messageContent[0:3]
			fixed["transaction_date"] = messageContent[3:21]
			fixed["summary"] = messageContent[21:31]
		}

	case ItemInformationRequest:
		// 17<transaction_date>
		if len(messageContent) >= 18 {
			fixed["transaction_date"] = messageContent[0:18]
		}

	case RenewRequest:
		// 29<third_party_allowed><no_block><transaction_date><nb_due_date>
		if len(messageContent) >= 20 {
			fixed["third_party_allowed"] = messageContent[0:1]
			fixed["no_block"] = messageContent[1:2]
			fixed["transaction_date"] = messageContent[2:20]
			if len(messageContent) >= 38 {
				fixed["nb_due_date"] = messageContent[20:38]
			}
		}

	case RenewAllRequest:
		// 65<transaction_date>
		if len(messageContent) >= 18 {
			fixed["transaction_date"] = messageContent[0:18]
		}

	case EndPatronSessionRequest:
		// 35<transaction_date>
		if len(messageContent) >= 18 {
			fixed["transaction_date"] = messageContent[0:18]
		}

	case FeePaidRequest:
		// 37<transaction_date><fee_type><payment_type><currency_type>
		if len(messageContent) >= 20 {
			fixed["transaction_date"] = messageContent[0:18]
			fixed["fee_type"] = messageContent[18:20]
			if len(messageContent) >= 22 {
				fixed["payment_type"] = messageContent[20:22]
			}
			if len(messageContent) >= 25 {
				fixed["currency_type"] = messageContent[22:25]
			}
		}

	case ItemStatusUpdateRequest:
		// 19<transaction_date>
		if len(messageContent) >= 18 {
			fixed["transaction_date"] = messageContent[0:18]
		}

	case SCStatus:
		// 99<status_code><max_print_width><protocol_version>
		if len(messageContent) >= 4 {
			fixed["status_code"] = messageContent[0:1]
			fixed["max_print_width"] = messageContent[1:4]
			if len(messageContent) >= 8 {
				fixed["protocol_version"] = messageContent[4:8]
			}
		}
	}

	return fixed
}

// StripMessageDelimiter removes the message delimiter from the end of a message
func (p *Parser) StripMessageDelimiter(message string) string {
	delimiter := p.config.GetMessageDelimiterBytes()
	return strings.TrimSuffix(message, string(delimiter))
}

// ValidateMessage performs basic validation on a parsed message
func (p *Parser) ValidateMessage(msg *Message) error {
	// Check if message code is valid
	if !msg.Code.IsRequestMessage() && !msg.Code.IsResponseMessage() {
		return fmt.Errorf("invalid message code: %s", msg.Code)
	}

	// Check if message is supported
	if !p.config.IsMessageSupported(string(msg.Code)) {
		return fmt.Errorf("message type %s (%s) is not enabled in configuration", msg.Code, msg.Code.MessageName())
	}

	return nil
}
