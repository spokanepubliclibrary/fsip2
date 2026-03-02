package models

import (
	"encoding/json"
	"testing"
)

// --- FlexibleFloat tests ---

func TestFlexibleFloat_UnmarshalJSON_Number(t *testing.T) {
	var f FlexibleFloat
	if err := json.Unmarshal([]byte(`10.5`), &f); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f.Float64() != 10.5 {
		t.Errorf("got %v, want 10.5", f.Float64())
	}
}

func TestFlexibleFloat_UnmarshalJSON_StringNumber(t *testing.T) {
	var f FlexibleFloat
	if err := json.Unmarshal([]byte(`"10.50"`), &f); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f.Float64() != 10.5 {
		t.Errorf("got %v, want 10.5", f.Float64())
	}
}

func TestFlexibleFloat_UnmarshalJSON_Zero(t *testing.T) {
	var f FlexibleFloat
	if err := json.Unmarshal([]byte(`0`), &f); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f.Float64() != 0 {
		t.Errorf("got %v, want 0", f.Float64())
	}
}

func TestFlexibleFloat_UnmarshalJSON_StringZero(t *testing.T) {
	var f FlexibleFloat
	if err := json.Unmarshal([]byte(`"0.00"`), &f); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f.Float64() != 0.0 {
		t.Errorf("got %v, want 0.0", f.Float64())
	}
}

func TestFlexibleFloat_UnmarshalJSON_InvalidString(t *testing.T) {
	var f FlexibleFloat
	err := json.Unmarshal([]byte(`"not-a-number"`), &f)
	if err == nil {
		t.Error("expected error for non-numeric string, got nil")
	}
}

func TestFlexibleFloat_UnmarshalJSON_Null(t *testing.T) {
	// Go's JSON decoder treats null as zero for numeric types
	var f FlexibleFloat
	if err := json.Unmarshal([]byte(`null`), &f); err != nil {
		t.Fatalf("unexpected error for null: %v", err)
	}
	if f.Float64() != 0.0 {
		t.Errorf("got %v, want 0.0 for null input", f.Float64())
	}
}

func TestFlexibleFloat_MarshalJSON(t *testing.T) {
	f := FlexibleFloat(3.14)
	data, err := f.MarshalJSON()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var result float64
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("failed to unmarshal marshaled value: %v", err)
	}
	if result != 3.14 {
		t.Errorf("got %v, want 3.14", result)
	}
}

func TestFlexibleFloat_Float64(t *testing.T) {
	f := FlexibleFloat(99.99)
	if f.Float64() != 99.99 {
		t.Errorf("got %v, want 99.99", f.Float64())
	}
}

// --- Account method tests ---

func TestAccount_IsOpen(t *testing.T) {
	tests := []struct {
		name   string
		status string
		want   bool
	}{
		{"open", "Open", true},
		{"closed", "Closed", false},
		{"empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &Account{Status: AccountStatus{Name: tt.status}}
			if got := a.IsOpen(); got != tt.want {
				t.Errorf("IsOpen() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAccount_IsOutstanding(t *testing.T) {
	tests := []struct {
		name      string
		remaining float64
		want      bool
	}{
		{"has balance", 5.00, true},
		{"zero balance", 0.00, false},
		{"small balance", 0.01, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &Account{Remaining: FlexibleFloat(tt.remaining)}
			if got := a.IsOutstanding(); got != tt.want {
				t.Errorf("IsOutstanding() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAccount_IsPaid(t *testing.T) {
	tests := []struct {
		name   string
		status string
		want   bool
	}{
		{"paid fully", "Paid fully", true},
		{"paid partially", "Paid partially", false},
		{"outstanding", "Outstanding", false},
		{"empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &Account{PaymentStatus: PaymentStatus{Name: tt.status}}
			if got := a.IsPaid(); got != tt.want {
				t.Errorf("IsPaid() = %v, want %v", got, tt.want)
			}
		})
	}
}

// --- AccountCollection tests ---

func TestAccountCollection_GetTotalOutstanding(t *testing.T) {
	ac := &AccountCollection{
		Accounts: []Account{
			{Remaining: FlexibleFloat(10.00)},
			{Remaining: FlexibleFloat(0.00)},
			{Remaining: FlexibleFloat(5.50)},
		},
	}
	got := ac.GetTotalOutstanding()
	want := 15.50
	if got != want {
		t.Errorf("GetTotalOutstanding() = %v, want %v", got, want)
	}
}

func TestAccountCollection_GetTotalOutstanding_Empty(t *testing.T) {
	ac := &AccountCollection{}
	if got := ac.GetTotalOutstanding(); got != 0.0 {
		t.Errorf("GetTotalOutstanding() = %v, want 0.0", got)
	}
}

func TestAccountCollection_GetTotalOutstanding_AllZero(t *testing.T) {
	ac := &AccountCollection{
		Accounts: []Account{
			{Remaining: FlexibleFloat(0)},
			{Remaining: FlexibleFloat(0)},
		},
	}
	if got := ac.GetTotalOutstanding(); got != 0.0 {
		t.Errorf("GetTotalOutstanding() = %v, want 0.0", got)
	}
}

func TestAccountCollection_GetOpenAccounts(t *testing.T) {
	ac := &AccountCollection{
		Accounts: []Account{
			{Status: AccountStatus{Name: "Open"}},
			{Status: AccountStatus{Name: "Closed"}},
			{Status: AccountStatus{Name: "Open"}},
		},
	}
	open := ac.GetOpenAccounts()
	if len(open) != 2 {
		t.Errorf("GetOpenAccounts() returned %d accounts, want 2", len(open))
	}
	for _, a := range open {
		if a.Status.Name != "Open" {
			t.Errorf("GetOpenAccounts() returned non-open account: %v", a.Status.Name)
		}
	}
}

func TestAccountCollection_GetOpenAccounts_None(t *testing.T) {
	ac := &AccountCollection{
		Accounts: []Account{
			{Status: AccountStatus{Name: "Closed"}},
		},
	}
	open := ac.GetOpenAccounts()
	if len(open) != 0 {
		t.Errorf("GetOpenAccounts() returned %d accounts, want 0", len(open))
	}
}

func TestAccountCollection_GetOpenAccounts_Empty(t *testing.T) {
	ac := &AccountCollection{}
	open := ac.GetOpenAccounts()
	if open != nil {
		t.Errorf("GetOpenAccounts() on empty collection should return nil, got %v", open)
	}
}
