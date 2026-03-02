package config

import (
	"fmt"
	"time"
)

// AddDuration adds a duration to a base date and returns the result with time stripped (set to 00:00:00.000+00:00)
// Handles M (months), D (days), and Y (years)
// For end-of-month scenarios, implements overflow logic (e.g., Jan 31 + 1M = Mar 3 if Feb has 28 days)
func AddDuration(baseDate time.Time, value int, period string) (time.Time, error) {
	if value < 0 {
		return time.Time{}, fmt.Errorf("value must be non-negative, got: %d", value)
	}

	// Strip time component from base date (set to midnight UTC)
	baseDate = time.Date(baseDate.Year(), baseDate.Month(), baseDate.Day(), 0, 0, 0, 0, time.UTC)

	var result time.Time

	switch period {
	case "D":
		// Add days
		result = baseDate.AddDate(0, 0, value)

	case "M":
		// Add months
		// Go's AddDate handles month overflow automatically
		// If result day doesn't exist in target month, it overflows to next month
		result = baseDate.AddDate(0, value, 0)

	case "Y":
		// Add years
		// Go's AddDate handles leap year transitions automatically
		result = baseDate.AddDate(value, 0, 0)

	default:
		return time.Time{}, fmt.Errorf("invalid period: %s (must be D, M, or Y)", period)
	}

	// Ensure time is stripped (set to midnight UTC)
	result = time.Date(result.Year(), result.Month(), result.Day(), 0, 0, 0, 0, time.UTC)

	return result, nil
}

// SubtractDuration subtracts a duration from a base date and returns the result with time stripped (set to 00:00:00.000+00:00)
// Handles M (months), D (days), and Y (years)
// For end-of-month scenarios, implements overflow logic (e.g., Mar 31 - 1M = Mar 3 if Feb has 28 days)
func SubtractDuration(baseDate time.Time, value int, period string) (time.Time, error) {
	if value < 0 {
		return time.Time{}, fmt.Errorf("value must be non-negative, got: %d", value)
	}

	// Strip time component from base date (set to midnight UTC)
	baseDate = time.Date(baseDate.Year(), baseDate.Month(), baseDate.Day(), 0, 0, 0, 0, time.UTC)

	var result time.Time

	switch period {
	case "D":
		// Subtract days
		result = baseDate.AddDate(0, 0, -value)

	case "M":
		// Subtract months
		// Go's AddDate handles month overflow automatically
		result = baseDate.AddDate(0, -value, 0)

	case "Y":
		// Subtract years
		result = baseDate.AddDate(-value, 0, 0)

	default:
		return time.Time{}, fmt.Errorf("invalid period: %s (must be D, M, or Y)", period)
	}

	// Ensure time is stripped (set to midnight UTC)
	result = time.Date(result.Year(), result.Month(), result.Day(), 0, 0, 0, 0, time.UTC)

	return result, nil
}

// IsWithinPeriod checks if the expiration date is within the specified period from today
// Returns true if expirationDate <= (today + renewWithin period)
func IsWithinPeriod(expirationDate, today time.Time, renewWithin string) (bool, error) {
	// Strip time components
	expirationDate = time.Date(expirationDate.Year(), expirationDate.Month(), expirationDate.Day(), 0, 0, 0, 0, time.UTC)
	today = time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)

	// Parse the renewWithin duration
	parsed, err := ParseDuration(renewWithin)
	if err != nil {
		return false, fmt.Errorf("failed to parse renewWithin: %w", err)
	}

	// Calculate the threshold date (today + renewWithin period)
	thresholdDate, err := AddDuration(today, parsed.Value, parsed.Period)
	if err != nil {
		return false, fmt.Errorf("failed to calculate threshold date: %w", err)
	}

	// Check if expiration date is on or before the threshold
	return !expirationDate.After(thresholdDate), nil
}

// IsExpired checks if the expiration date is in the past (before today)
// Returns true if expirationDate < today (using date-only comparison)
func IsExpired(expirationDate, today time.Time) bool {
	// Strip time components for date-only comparison
	expirationDate = time.Date(expirationDate.Year(), expirationDate.Month(), expirationDate.Day(), 0, 0, 0, 0, time.UTC)
	today = time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)

	// Check if expiration is before today
	return expirationDate.Before(today)
}

// StripTime returns a copy of the time with the time component set to 00:00:00.000 UTC
// This ensures date-only comparisons
func StripTime(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// FormatDate formats a time as YYYY-MM-DDT00:00:00.000+00:00 (FOLIO format)
func FormatDate(t time.Time) string {
	// Strip time and format
	t = StripTime(t)
	return t.Format("2006-01-02T15:04:05.000-07:00")
}
