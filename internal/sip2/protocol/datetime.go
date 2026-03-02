package protocol

import (
	"time"
)

// SIP2 date/time format: YYYYMMDDZZZZHHMMSS
// - YYYY: 4-digit year
// - MM: 2-digit month (01-12)
// - DD: 2-digit day (01-31)
// - ZZZZ: 4 spaces (reserved)
// - HH: 2-digit hour (00-23)
// - MM: 2-digit minute (00-59)
// - SS: 2-digit second (00-59)

const (
	// SIP2DateTimeFormat is the standard SIP2 date/time format
	SIP2DateTimeFormat = "20060102    150405"

	// SIP2DateFormat is the SIP2 date-only format (used in some fields)
	SIP2DateFormat = "20060102"
)

// FormatSIP2DateTime formats a time.Time into SIP2 date/time format
func FormatSIP2DateTime(t time.Time, timezone string) string {
	if t.IsZero() {
		return ""
	}

	// Load timezone
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		// Fall back to UTC if timezone is invalid
		loc = time.UTC
	}

	// Convert to specified timezone
	t = t.In(loc)

	return t.Format(SIP2DateTimeFormat)
}

// FormatSIP2Date formats a time.Time into SIP2 date-only format
func FormatSIP2Date(t time.Time, timezone string) string {
	if t.IsZero() {
		return ""
	}

	// Load timezone
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		// Fall back to UTC if timezone is invalid
		loc = time.UTC
	}

	// Convert to specified timezone
	t = t.In(loc)

	return t.Format(SIP2DateFormat)
}

// ParseSIP2DateTime parses a SIP2 date/time string into time.Time
func ParseSIP2DateTime(s string, timezone string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}

	// Load timezone
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		// Fall back to UTC if timezone is invalid
		loc = time.UTC
	}

	// Parse the date/time in the specified timezone
	t, err := time.ParseInLocation(SIP2DateTimeFormat, s, loc)
	if err != nil {
		return time.Time{}, err
	}

	return t, nil
}

// ParseSIP2Date parses a SIP2 date string into time.Time
func ParseSIP2Date(s string, timezone string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}

	// Load timezone
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		// Fall back to UTC if timezone is invalid
		loc = time.UTC
	}

	// Parse the date in the specified timezone
	t, err := time.ParseInLocation(SIP2DateFormat, s, loc)
	if err != nil {
		return time.Time{}, err
	}

	return t, nil
}

// CurrentSIP2DateTime returns the current date/time in SIP2 format
func CurrentSIP2DateTime(timezone string) string {
	return FormatSIP2DateTime(time.Now(), timezone)
}

// CurrentSIP2Date returns the current date in SIP2 format
func CurrentSIP2Date(timezone string) string {
	return FormatSIP2Date(time.Now(), timezone)
}
