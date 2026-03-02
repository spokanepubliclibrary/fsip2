package models

import "time"

// Loan represents a FOLIO loan
type Loan struct {
	ID                                string     `json:"id"`
	UserID                            string     `json:"userId"`
	ProxyUserID                       string     `json:"proxyUserId,omitempty"`
	ItemID                            string     `json:"itemId"`
	ItemEffectiveLocationIdAtCheckOut string     `json:"itemEffectiveLocationIdAtCheckOut,omitempty"`
	Status                            LoanStatus `json:"status"`
	LoanDate                          *time.Time `json:"loanDate"`
	DueDate                           *time.Time `json:"dueDate"`
	ReturnDate                        *time.Time `json:"returnDate,omitempty"`
	SystemReturnDate                  *time.Time `json:"systemReturnDate,omitempty"`
	Action                            string     `json:"action"`
	ActionComment                     string     `json:"actionComment,omitempty"`
	ItemStatus                        string     `json:"itemStatus,omitempty"`
	RenewalCount                      int        `json:"renewalCount"`
	LoanPolicyID                      string     `json:"loanPolicyId,omitempty"`
	CheckoutServicePointID            string     `json:"checkoutServicePointId,omitempty"`
	CheckinServicePointID             string     `json:"checkinServicePointId,omitempty"`
	Metadata                          Metadata   `json:"metadata,omitempty"`
	// Populated fields (not from API directly)
	Item *Item `json:"item,omitempty"`
}

// LoanStatus represents the status of a loan
type LoanStatus struct {
	Name string `json:"name"` // Open, Closed
}

// LoanCollection represents a collection of loans
type LoanCollection struct {
	Loans        []Loan     `json:"loans"`
	TotalRecords int        `json:"totalRecords"`
	ResultInfo   ResultInfo `json:"resultInfo,omitempty"`
}

// ResultInfo provides pagination information
type ResultInfo struct {
	TotalRecords int                      `json:"totalRecords"`
	Facets       []map[string]interface{} `json:"facets,omitempty"`
	Diagnostics  []map[string]interface{} `json:"diagnostics,omitempty"`
}

// IsOpen checks if the loan is currently open
func (l *Loan) IsOpen() bool {
	return l.Status.Name == "Open"
}

// IsOverdue checks if the loan is overdue
func (l *Loan) IsOverdue() bool {
	if !l.IsOpen() || l.DueDate == nil {
		return false
	}
	return l.DueDate.Before(time.Now())
}

// CanRenew checks if the loan can be renewed (basic check)
func (l *Loan) CanRenew() bool {
	return l.IsOpen() && l.ReturnDate == nil
}
