package models

import (
	"testing"
	"time"
)

func TestLoan_IsOpen(t *testing.T) {
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
			l := &Loan{Status: LoanStatus{Name: tt.status}}
			if got := l.IsOpen(); got != tt.want {
				t.Errorf("IsOpen() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLoan_IsOverdue_OpenAndOverdue(t *testing.T) {
	past := time.Now().Add(-24 * time.Hour)
	l := &Loan{
		Status:  LoanStatus{Name: "Open"},
		DueDate: &past,
	}
	if !l.IsOverdue() {
		t.Error("IsOverdue() = false, want true for open loan past due date")
	}
}

func TestLoan_IsOverdue_OpenNotOverdue(t *testing.T) {
	future := time.Now().Add(24 * time.Hour)
	l := &Loan{
		Status:  LoanStatus{Name: "Open"},
		DueDate: &future,
	}
	if l.IsOverdue() {
		t.Error("IsOverdue() = true, want false for open loan with future due date")
	}
}

func TestLoan_IsOverdue_Closed(t *testing.T) {
	past := time.Now().Add(-24 * time.Hour)
	l := &Loan{
		Status:  LoanStatus{Name: "Closed"},
		DueDate: &past,
	}
	if l.IsOverdue() {
		t.Error("IsOverdue() = true, want false for closed loan")
	}
}

func TestLoan_IsOverdue_NilDueDate(t *testing.T) {
	l := &Loan{
		Status:  LoanStatus{Name: "Open"},
		DueDate: nil,
	}
	if l.IsOverdue() {
		t.Error("IsOverdue() = true, want false for loan with nil due date")
	}
}

func TestLoan_CanRenew_OpenNoReturnDate(t *testing.T) {
	l := &Loan{
		Status:     LoanStatus{Name: "Open"},
		ReturnDate: nil,
	}
	if !l.CanRenew() {
		t.Error("CanRenew() = false, want true for open loan without return date")
	}
}

func TestLoan_CanRenew_Closed(t *testing.T) {
	l := &Loan{
		Status:     LoanStatus{Name: "Closed"},
		ReturnDate: nil,
	}
	if l.CanRenew() {
		t.Error("CanRenew() = true, want false for closed loan")
	}
}

func TestLoan_CanRenew_OpenWithReturnDate(t *testing.T) {
	returnDate := time.Now().Add(-1 * time.Hour)
	l := &Loan{
		Status:     LoanStatus{Name: "Open"},
		ReturnDate: &returnDate,
	}
	if l.CanRenew() {
		t.Error("CanRenew() = true, want false for open loan with return date set")
	}
}
