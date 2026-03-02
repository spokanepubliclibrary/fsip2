package config

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var durationRegex = regexp.MustCompile(`^(\d+)([MDYmdy])$`)

// ParsedDuration represents a parsed duration with value and period
type ParsedDuration struct {
	Value  int
	Period string // M (months), D (days), or Y (years) - normalized to uppercase
}

// ParseDuration parses a duration string (e.g., "6M", "30D", "1Y") into value and period
// The period is case-insensitive and will be normalized to uppercase
// Returns error if the format is invalid or period is not M, D, or Y
func ParseDuration(s string) (*ParsedDuration, error) {
	if s == "" {
		return nil, fmt.Errorf("duration string cannot be empty")
	}

	// Trim whitespace
	s = strings.TrimSpace(s)

	// Match against regex pattern
	matches := durationRegex.FindStringSubmatch(s)
	if matches == nil {
		return nil, fmt.Errorf("invalid duration format: %s (expected format: <number><period>, e.g., 6M, 30D, 1Y)", s)
	}

	// Parse the numeric value
	value, err := strconv.Atoi(matches[1])
	if err != nil {
		return nil, fmt.Errorf("invalid numeric value in duration: %s", matches[1])
	}

	if value <= 0 {
		return nil, fmt.Errorf("duration value must be greater than 0, got: %d", value)
	}

	// Normalize period to uppercase
	period := strings.ToUpper(matches[2])

	return &ParsedDuration{
		Value:  value,
		Period: period,
	}, nil
}

// String returns the string representation of the duration (e.g., "6M")
func (d *ParsedDuration) String() string {
	return fmt.Sprintf("%d%s", d.Value, d.Period)
}
