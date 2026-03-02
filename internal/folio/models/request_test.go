package models

import "testing"

func TestRequest_IsOpen(t *testing.T) {
	tests := []struct {
		name   string
		status string
		want   bool
	}{
		{"not yet filled", "Open - Not yet filled", true},
		{"awaiting pickup", "Open - Awaiting pickup", true},
		{"in transit", "Open - In transit", true},
		{"closed filled", "Closed - Filled", false},
		{"closed unfilled", "Closed - Unfilled", false},
		{"empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Request{Status: tt.status}
			if got := r.IsOpen(); got != tt.want {
				t.Errorf("IsOpen() = %v, want %v (status: %q)", got, tt.want, tt.status)
			}
		})
	}
}

func TestRequest_IsAwaitingPickup(t *testing.T) {
	tests := []struct {
		name   string
		status string
		want   bool
	}{
		{"awaiting pickup", "Open - Awaiting pickup", true},
		{"not yet filled", "Open - Not yet filled", false},
		{"in transit", "Open - In transit", false},
		{"closed", "Closed - Filled", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Request{Status: tt.status}
			if got := r.IsAwaitingPickup(); got != tt.want {
				t.Errorf("IsAwaitingPickup() = %v, want %v (status: %q)", got, tt.want, tt.status)
			}
		})
	}
}

func TestRequest_IsHold(t *testing.T) {
	tests := []struct {
		name        string
		requestType string
		want        bool
	}{
		{"hold", "Hold", true},
		{"recall", "Recall", true},
		{"page", "Page", false},
		{"empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Request{RequestType: tt.requestType}
			if got := r.IsHold(); got != tt.want {
				t.Errorf("IsHold() = %v, want %v (type: %q)", got, tt.want, tt.requestType)
			}
		})
	}
}
