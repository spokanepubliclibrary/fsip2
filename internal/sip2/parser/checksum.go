package parser

import (
	"fmt"
	"strconv"

	"golang.org/x/text/encoding"
)

// ChecksumResult contains the result of checksum validation
type ChecksumResult struct {
	Valid          bool
	SequenceNumber string
	ChecksumValue  string
	Message        string
}

// ValidateChecksum validates the SIP2 message checksum
// Algorithm (from Java implementation):
// 1. Extract message bytes (excluding last 4 chars which contain checksum)
// 2. Sum all byte values
// 3. Parse checksum from last 4 hex chars
// 4. Add checksum to sum and mask to 16-bit
// 5. Valid if result == 0
func ValidateChecksum(message string, encoder encoding.Encoding) (*ChecksumResult, error) {
	msgLen := len(message)

	if msgLen < 4 {
		return &ChecksumResult{
			Valid:   false,
			Message: "message too short for checksum",
		}, fmt.Errorf("message too short for checksum validation")
	}

	// Extract the last 4 characters as the checksum
	checksumStr := message[msgLen-4:]
	messageContent := message[:msgLen-4]

	// Look for AY (sequence number) and AZ (checksum) fields
	// Format: ...AY[seq]AZ[checksum]
	// We need to extract just the message without AZ field
	ayIndex := -1
	azIndex := -1

	for i := 0; i < len(messageContent)-1; i++ {
		if messageContent[i:i+2] == "AY" {
			ayIndex = i
		}
		if messageContent[i:i+2] == "AZ" {
			azIndex = i
			break
		}
	}

	sequenceNumber := ""
	actualMessageContent := messageContent

	if ayIndex != -1 && azIndex != -1 {
		// Extract sequence number (between AY and AZ)
		sequenceNumber = messageContent[ayIndex+2 : azIndex]
		// Message content is everything before AY
		actualMessageContent = messageContent[:ayIndex]
	} else if azIndex != -1 {
		// Only AZ found, no sequence number
		actualMessageContent = messageContent[:azIndex]
	}

	// Reconstruct the full string that was originally checksummed, in one piece.
	// CalculateChecksum always builds: message + fieldDelimiter + "AY" + sequenceNumber + "AZ"
	// actualMessageContent = messageContent[:ayIndex], so it already includes the trailing
	// field delimiter that CalculateChecksum prepended before "AY". No extra delimiter needed.
	var fullString string
	if ayIndex != -1 {
		fullString = actualMessageContent + "AY" + sequenceNumber + "AZ"
	} else {
		// No AY/AZ found — encode whatever content we have up to AZ.
		fullString = actualMessageContent + "AZ"
	}

	// Encode the entire reconstructed string in ONE call to avoid split-encoder bugs.
	messageBytes, err := encoder.NewEncoder().Bytes([]byte(fullString))
	if err != nil {
		return &ChecksumResult{
			Valid:   false,
			Message: fmt.Sprintf("encoding error: %v", err),
		}, fmt.Errorf("failed to encode message: %w", err)
	}

	// Sum all byte values
	sum := 0
	for _, b := range messageBytes {
		sum += int(b) & 0xff
	}

	// Parse the checksum as a hex value
	checksum, err := strconv.ParseInt(checksumStr, 16, 64)
	if err != nil {
		return &ChecksumResult{
			Valid:          false,
			SequenceNumber: sequenceNumber,
			ChecksumValue:  checksumStr,
			Message:        fmt.Sprintf("invalid checksum format: %s", checksumStr),
		}, fmt.Errorf("failed to parse checksum: %w", err)
	}

	// Add checksum to sum and mask to 16 bits
	total := (sum + int(checksum)) & 0xffff

	// Checksum is valid if the total equals 0
	valid := total == 0

	result := &ChecksumResult{
		Valid:          valid,
		SequenceNumber: sequenceNumber,
		ChecksumValue:  checksumStr,
	}

	if valid {
		result.Message = "checksum valid"
	} else {
		result.Message = fmt.Sprintf("checksum invalid: expected 0, got %d", total)
	}

	return result, nil
}

// CalculateChecksum calculates the checksum for a SIP2 message
// This is used when generating response messages
func CalculateChecksum(message string, sequenceNumber string, fieldDelimiter string, encoder encoding.Encoding) (string, error) {
	// Build the message with AY field - delimiter is added to ensure consistent format
	// even when message has no trailing delimiter (e.g., messages with no variable fields)
	messageWithSeq := message + fieldDelimiter + "AY" + sequenceNumber + "AZ"

	// Encode to bytes
	messageBytes, err := encoder.NewEncoder().Bytes([]byte(messageWithSeq))
	if err != nil {
		return "", fmt.Errorf("failed to encode message: %w", err)
	}

	// Sum all byte values
	sum := 0
	for _, b := range messageBytes {
		sum += int(b) & 0xff
	}

	// Calculate checksum: negate sum and mask to 16 bits
	checksum := (-sum) & 0xffff

	// Format as 4-digit hex
	return fmt.Sprintf("%04X", checksum), nil
}

// StripChecksum removes the checksum fields (AY and AZ) from a message
func StripChecksum(message string) string {
	// Find AY field
	for i := 0; i < len(message)-1; i++ {
		if message[i:i+2] == "AY" {
			// Return everything before AY
			return message[:i]
		}
	}
	return message
}

// ExtractSequenceNumber extracts the sequence number from a message
func ExtractSequenceNumber(message string) string {
	// Look for AY field
	for i := 0; i < len(message)-1; i++ {
		if message[i:i+2] == "AY" {
			// Find the next field code (2 uppercase letters) or end of message
			for j := i + 2; j < len(message); j++ {
				if j+1 < len(message) && message[j] >= 'A' && message[j] <= 'Z' && message[j+1] >= 'A' && message[j+1] <= 'Z' {
					return message[i+2 : j]
				}
			}
			// If no field code found, return the rest of the message
			return message[i+2:]
		}
	}
	return "0" // Default sequence number
}
