package protocol

import (
	"fmt"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/unicode"
)

// GetEncoder returns the encoding.Encoding for the specified charset name
func GetEncoder(charset string) (encoding.Encoding, error) {
	switch charset {
	case "IBM850":
		return charmap.CodePage850, nil
	case "ISO-8859-1":
		return charmap.ISO8859_1, nil
	case "UTF-8":
		return unicode.UTF8, nil
	case "IBM437":
		return charmap.CodePage437, nil
	case "Windows-1252":
		return charmap.Windows1252, nil
	default:
		return nil, fmt.Errorf("unsupported charset: %s", charset)
	}
}

// EncodeString encodes a string using the specified charset
func EncodeString(s string, charset string) ([]byte, error) {
	encoder, err := GetEncoder(charset)
	if err != nil {
		return nil, err
	}

	encoded, err := encoder.NewEncoder().Bytes([]byte(s))
	if err != nil {
		return nil, fmt.Errorf("failed to encode string: %w", err)
	}

	return encoded, nil
}

// DecodeBytes decodes bytes using the specified charset
func DecodeBytes(b []byte, charset string) (string, error) {
	encoder, err := GetEncoder(charset)
	if err != nil {
		return "", err
	}

	decoded, err := encoder.NewDecoder().Bytes(b)
	if err != nil {
		return "", fmt.Errorf("failed to decode bytes: %w", err)
	}

	return string(decoded), nil
}

// SupportedCharsets returns a list of supported character sets
func SupportedCharsets() []string {
	return []string{
		"IBM850",
		"ISO-8859-1",
		"UTF-8",
		"IBM437",
		"Windows-1252",
	}
}

// IsCharsetSupported checks if a charset is supported
func IsCharsetSupported(charset string) bool {
	for _, supported := range SupportedCharsets() {
		if charset == supported {
			return true
		}
	}
	return false
}
