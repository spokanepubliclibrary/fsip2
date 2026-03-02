package customfields

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spokanepubliclibrary/fsip2/internal/config"
	"github.com/spokanepubliclibrary/fsip2/internal/folio/models"
	"go.uber.org/zap"
)

// ProcessCustomFields extracts and formats custom fields from user record
// Returns a slice of formatted SIP2 field strings (e.g., "|SAvalue")
func ProcessCustomFields(
	user *models.User,
	cfg *config.PatronCustomFieldsConfig,
	delimiter string,
	logger *zap.Logger,
) []string {
	// Check if custom fields are enabled
	if cfg == nil || !cfg.Enabled {
		return []string{}
	}

	// Check if user has custom fields
	if user == nil || user.CustomFields == nil {
		return []string{}
	}

	if len(cfg.Fields) == 0 {
		return []string{}
	}

	var result []string

	// Process each configured field in order
	for _, mapping := range cfg.Fields {
		// Get value from user custom fields
		value, exists := user.CustomFields[mapping.Source]
		if !exists {
			logger.Debug("Custom field not found in user record",
				zap.String("field_code", mapping.Code),
				zap.String("source", mapping.Source),
				zap.String("user_id", user.ID),
			)
			continue
		}

		// Skip nil values
		if value == nil {
			logger.Debug("Custom field is nil, skipping",
				zap.String("field_code", mapping.Code),
				zap.String("source", mapping.Source),
			)
			continue
		}

		// Format the value based on type
		formattedValue, err := formatCustomField(value, &mapping, logger)
		if err != nil {
			logger.Warn("Failed to format custom field, skipping",
				zap.String("field_code", mapping.Code),
				zap.String("source", mapping.Source),
				zap.Error(err),
			)
			continue
		}

		// Skip empty values
		if formattedValue == "" {
			logger.Debug("Custom field formatted to empty string, skipping",
				zap.String("field_code", mapping.Code),
				zap.String("source", mapping.Source),
			)
			continue
		}

		// Validate and sanitize the value
		sanitizedValue, valid := validateAndSanitize(formattedValue, delimiter, logger)
		if !valid {
			logger.Warn("Custom field contains invalid characters, skipping",
				zap.String("field_code", mapping.Code),
				zap.String("source", mapping.Source),
				zap.String("value", formattedValue),
			)
			continue
		}

		// Truncate if necessary
		if len(sanitizedValue) > mapping.MaxLength {
			logger.Warn("Custom field exceeds max length, truncating",
				zap.String("field_code", mapping.Code),
				zap.String("source", mapping.Source),
				zap.Int("original_length", len(sanitizedValue)),
				zap.Int("max_length", mapping.MaxLength),
			)
			sanitizedValue = sanitizedValue[:mapping.MaxLength]
		}

		// Build SIP2 field string
		fieldString := delimiter + strings.ToUpper(mapping.Code) + sanitizedValue

		result = append(result, fieldString)

		logger.Debug("Added custom field to response",
			zap.String("field_code", mapping.Code),
			zap.String("source", mapping.Source),
			zap.Int("value_length", len(sanitizedValue)),
		)
	}

	return result
}

// formatCustomField converts a field value to string based on configured type
func formatCustomField(
	value interface{},
	mapping *config.CustomFieldMapping,
	logger *zap.Logger,
) (string, error) {
	switch mapping.Type {
	case "string":
		// Try to get as string
		if str, ok := value.(string); ok {
			if str == "" {
				return "", fmt.Errorf("empty string")
			}
			return str, nil
		}
		// Type mismatch - try to convert
		logger.Warn("Type mismatch for string field, attempting conversion",
			zap.String("field_code", mapping.Code),
			zap.String("source", mapping.Source),
			zap.String("expected_type", "string"),
			zap.String("actual_type", fmt.Sprintf("%T", value)),
		)
		return fmt.Sprintf("%v", value), nil

	case "boolean":
		// Try to get as boolean
		if b, ok := value.(bool); ok {
			return strconv.FormatBool(b), nil
		}
		// Type mismatch
		logger.Warn("Type mismatch for boolean field",
			zap.String("field_code", mapping.Code),
			zap.String("source", mapping.Source),
			zap.String("expected_type", "boolean"),
			zap.String("actual_type", fmt.Sprintf("%T", value)),
		)
		return "", fmt.Errorf("not a boolean")

	case "array":
		// Try to get as array/slice
		if arr, ok := value.([]interface{}); ok {
			if len(arr) == 0 {
				return "", fmt.Errorf("empty array")
			}
			delimiter := mapping.ArrayDelimiter
			if delimiter == "" {
				delimiter = ","
			}
			parts := make([]string, len(arr))
			for i, v := range arr {
				parts[i] = fmt.Sprintf("%v", v)
			}
			return strings.Join(parts, delimiter), nil
		}
		// Type mismatch
		logger.Warn("Type mismatch for array field",
			zap.String("field_code", mapping.Code),
			zap.String("source", mapping.Source),
			zap.String("expected_type", "array"),
			zap.String("actual_type", fmt.Sprintf("%T", value)),
		)
		return "", fmt.Errorf("not an array")

	default:
		return "", fmt.Errorf("unknown type: %s", mapping.Type)
	}
}

// validateAndSanitize checks if value is safe for SIP2 output and sanitizes it
// Returns the sanitized value and a boolean indicating if the value is valid
func validateAndSanitize(value string, delimiter string, logger *zap.Logger) (string, bool) {
	// Check for pipe character (field delimiter) - this is a hard failure
	if strings.Contains(value, delimiter) {
		logger.Debug("Value contains field delimiter",
			zap.String("delimiter", delimiter),
		)
		return "", false
	}

	// Check for message delimiters (CR, LF) - this is a hard failure
	if strings.ContainsAny(value, "\r\n") {
		logger.Debug("Value contains message delimiter (CR or LF)")
		return "", false
	}

	// Sanitize control characters (replace with space, except tab)
	var sanitized strings.Builder
	hasControlChars := false
	for _, r := range value {
		if r < 32 && r != 9 { // Control character (not tab)
			sanitized.WriteRune(' ')
			hasControlChars = true
		} else {
			sanitized.WriteRune(r)
		}
	}

	if hasControlChars {
		logger.Debug("Replaced control characters with spaces")
	}

	return sanitized.String(), true
}
