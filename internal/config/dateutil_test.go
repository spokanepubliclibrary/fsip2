package config

import (
	"testing"
	"time"
)

func TestSubtractDuration_Days(t *testing.T) {
	base := time.Date(2024, 3, 15, 12, 30, 0, 0, time.UTC)
	result, err := SubtractDuration(base, 10, "D")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := time.Date(2024, 3, 5, 0, 0, 0, 0, time.UTC)
	if !result.Equal(expected) {
		t.Errorf("expected %v, got %v", expected, result)
	}
}

func TestSubtractDuration_Months(t *testing.T) {
	base := time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC)
	result, err := SubtractDuration(base, 3, "M")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC)
	if !result.Equal(expected) {
		t.Errorf("expected %v, got %v", expected, result)
	}
}

func TestSubtractDuration_Years(t *testing.T) {
	base := time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC)
	result, err := SubtractDuration(base, 2, "Y")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := time.Date(2022, 6, 15, 0, 0, 0, 0, time.UTC)
	if !result.Equal(expected) {
		t.Errorf("expected %v, got %v", expected, result)
	}
}

func TestSubtractDuration_InvalidPeriod(t *testing.T) {
	base := time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC)
	_, err := SubtractDuration(base, 5, "W")
	if err == nil {
		t.Error("expected error for invalid period W")
	}
}

func TestSubtractDuration_NegativeValue(t *testing.T) {
	base := time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC)
	_, err := SubtractDuration(base, -5, "D")
	if err == nil {
		t.Error("expected error for negative value")
	}
}

func TestSubtractDuration_ZeroValue(t *testing.T) {
	base := time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC)
	result, err := SubtractDuration(base, 0, "D")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC)
	if !result.Equal(expected) {
		t.Errorf("expected %v, got %v", expected, result)
	}
}

func TestSubtractDuration_StripsTime(t *testing.T) {
	base := time.Date(2024, 6, 15, 23, 59, 59, 999, time.UTC)
	result, err := SubtractDuration(base, 1, "D")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Hour() != 0 || result.Minute() != 0 || result.Second() != 0 {
		t.Errorf("time not stripped: got %v", result)
	}
}

func TestAddDuration_Days(t *testing.T) {
	base := time.Date(2024, 3, 5, 0, 0, 0, 0, time.UTC)
	result, err := AddDuration(base, 10, "D")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC)
	if !result.Equal(expected) {
		t.Errorf("expected %v, got %v", expected, result)
	}
}

func TestAddDuration_Months(t *testing.T) {
	base := time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC)
	result, err := AddDuration(base, 3, "M")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC)
	if !result.Equal(expected) {
		t.Errorf("expected %v, got %v", expected, result)
	}
}

func TestAddDuration_Years(t *testing.T) {
	base := time.Date(2022, 6, 15, 0, 0, 0, 0, time.UTC)
	result, err := AddDuration(base, 2, "Y")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC)
	if !result.Equal(expected) {
		t.Errorf("expected %v, got %v", expected, result)
	}
}

func TestAddDuration_InvalidPeriod(t *testing.T) {
	base := time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC)
	_, err := AddDuration(base, 5, "H")
	if err == nil {
		t.Error("expected error for invalid period H")
	}
}

func TestAddDuration_NegativeValue(t *testing.T) {
	base := time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC)
	_, err := AddDuration(base, -1, "M")
	if err == nil {
		t.Error("expected error for negative value")
	}
}

func TestIsWithinPeriod_True(t *testing.T) {
	today := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
	// Expires in 30 days, threshold is 60 days => within period
	expiration := time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC)
	within, err := IsWithinPeriod(expiration, today, "60D")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !within {
		t.Error("expected within period to be true")
	}
}

func TestIsWithinPeriod_False(t *testing.T) {
	today := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
	// Expires in 90 days, threshold is 30 days => not within period
	expiration := time.Date(2024, 8, 30, 0, 0, 0, 0, time.UTC)
	within, err := IsWithinPeriod(expiration, today, "30D")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if within {
		t.Error("expected within period to be false")
	}
}

func TestIsWithinPeriod_InvalidDuration(t *testing.T) {
	today := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
	expiration := time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC)
	_, err := IsWithinPeriod(expiration, today, "invalid")
	if err == nil {
		t.Error("expected error for invalid duration")
	}
}

func TestIsExpired_Expired(t *testing.T) {
	today := time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC)
	expired := time.Date(2024, 6, 14, 0, 0, 0, 0, time.UTC)
	if !IsExpired(expired, today) {
		t.Error("expected expired=true")
	}
}

func TestIsExpired_NotExpired(t *testing.T) {
	today := time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC)
	notExpired := time.Date(2024, 6, 16, 0, 0, 0, 0, time.UTC)
	if IsExpired(notExpired, today) {
		t.Error("expected expired=false")
	}
}

func TestIsExpired_SameDay(t *testing.T) {
	today := time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC)
	sameDay := time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC)
	// Same day is NOT expired
	if IsExpired(sameDay, today) {
		t.Error("expected expired=false on same day")
	}
}

func TestStripTime(t *testing.T) {
	t1 := time.Date(2024, 6, 15, 14, 30, 59, 999, time.UTC)
	result := StripTime(t1)
	if result.Hour() != 0 || result.Minute() != 0 || result.Second() != 0 || result.Nanosecond() != 0 {
		t.Errorf("time not stripped: got %v", result)
	}
	if result.Year() != 2024 || result.Month() != 6 || result.Day() != 15 {
		t.Errorf("date changed: got %v", result)
	}
}

func TestFormatDate(t *testing.T) {
	t1 := time.Date(2024, 6, 15, 14, 30, 59, 0, time.UTC)
	result := FormatDate(t1)
	// Should produce a date-only format with zeroed time
	if len(result) == 0 {
		t.Error("expected non-empty formatted date")
	}
	// Verify the date part
	if result[:10] != "2024-06-15" {
		t.Errorf("expected date 2024-06-15, got %s", result[:10])
	}
	// Verify time is zeroed
	if result[11:19] != "00:00:00" {
		t.Errorf("expected zeroed time, got %s", result[11:19])
	}
}
