package models

import (
	"testing"
	"time"
)

// --- User.GetFullName ---

func TestUser_GetFullName_FirstLast(t *testing.T) {
	u := &User{
		Personal: PersonalInfo{FirstName: "Jane", LastName: "Doe"},
	}
	got := u.GetFullName()
	want := "Jane Doe"
	if got != want {
		t.Errorf("GetFullName() = %q, want %q", got, want)
	}
}

func TestUser_GetFullName_WithMiddle(t *testing.T) {
	u := &User{
		Personal: PersonalInfo{FirstName: "Jane", MiddleName: "Marie", LastName: "Doe"},
	}
	got := u.GetFullName()
	want := "Jane Marie Doe"
	if got != want {
		t.Errorf("GetFullName() = %q, want %q", got, want)
	}
}

func TestUser_GetFullName_NoMiddle(t *testing.T) {
	u := &User{
		Personal: PersonalInfo{FirstName: "John", LastName: "Smith"},
	}
	got := u.GetFullName()
	want := "John Smith"
	if got != want {
		t.Errorf("GetFullName() = %q, want %q", got, want)
	}
}

// --- User.GetPrimaryAddress ---

func TestUser_GetPrimaryAddress_ReturnsPrimary(t *testing.T) {
	u := &User{
		Personal: PersonalInfo{
			Addresses: []Address{
				{AddressLine1: "123 Main St", PrimaryAddress: false},
				{AddressLine1: "456 Oak Ave", PrimaryAddress: true},
			},
		},
	}
	addr := u.GetPrimaryAddress()
	if addr == nil {
		t.Fatal("GetPrimaryAddress() = nil, want non-nil")
	}
	if addr.AddressLine1 != "456 Oak Ave" {
		t.Errorf("GetPrimaryAddress().AddressLine1 = %q, want %q", addr.AddressLine1, "456 Oak Ave")
	}
}

func TestUser_GetPrimaryAddress_FallsBackToFirst(t *testing.T) {
	u := &User{
		Personal: PersonalInfo{
			Addresses: []Address{
				{AddressLine1: "First Street"},
				{AddressLine1: "Second Street"},
			},
		},
	}
	addr := u.GetPrimaryAddress()
	if addr == nil {
		t.Fatal("GetPrimaryAddress() = nil, want non-nil")
	}
	if addr.AddressLine1 != "First Street" {
		t.Errorf("GetPrimaryAddress().AddressLine1 = %q, want %q", addr.AddressLine1, "First Street")
	}
}

func TestUser_GetPrimaryAddress_NoAddresses(t *testing.T) {
	u := &User{}
	addr := u.GetPrimaryAddress()
	if addr != nil {
		t.Errorf("GetPrimaryAddress() = %v, want nil", addr)
	}
}

// --- User.IsExpired ---

func TestUser_IsExpired_Expired(t *testing.T) {
	past := time.Now().Add(-24 * time.Hour)
	u := &User{ExpirationDate: &past}
	if !u.IsExpired() {
		t.Error("IsExpired() = false, want true for past expiration date")
	}
}

func TestUser_IsExpired_NotExpired(t *testing.T) {
	future := time.Now().Add(24 * time.Hour)
	u := &User{ExpirationDate: &future}
	if u.IsExpired() {
		t.Error("IsExpired() = true, want false for future expiration date")
	}
}

func TestUser_IsExpired_NilExpiration(t *testing.T) {
	u := &User{ExpirationDate: nil}
	if u.IsExpired() {
		t.Error("IsExpired() = true, want false for nil expiration date")
	}
}

// --- User.GetCustomField ---

func TestUser_GetCustomField_Found(t *testing.T) {
	u := &User{
		CustomFields: map[string]interface{}{
			"studentID": "S12345",
		},
	}
	val, ok := u.GetCustomField("studentID")
	if !ok {
		t.Fatal("GetCustomField() returned ok=false, want true")
	}
	if val != "S12345" {
		t.Errorf("GetCustomField() = %v, want %q", val, "S12345")
	}
}

func TestUser_GetCustomField_NotFound(t *testing.T) {
	u := &User{
		CustomFields: map[string]interface{}{},
	}
	val, ok := u.GetCustomField("missing")
	if ok {
		t.Error("GetCustomField() returned ok=true, want false")
	}
	if val != nil {
		t.Errorf("GetCustomField() = %v, want nil", val)
	}
}

func TestUser_GetCustomField_NilMap(t *testing.T) {
	u := &User{}
	val, ok := u.GetCustomField("anything")
	if ok {
		t.Error("GetCustomField() returned ok=true, want false for nil CustomFields")
	}
	if val != nil {
		t.Errorf("GetCustomField() = %v, want nil", val)
	}
}
