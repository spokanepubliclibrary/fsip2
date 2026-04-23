package models

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

// FlexibleFloat handles JSON fields that can be either string or number
type FlexibleFloat float64

// UnmarshalJSON implements custom unmarshaling for FlexibleFloat
// Handles both string ("10.00") and number (10.00) formats
func (f *FlexibleFloat) UnmarshalJSON(data []byte) error {
	// Try to unmarshal as float64 first
	var numValue float64
	if err := json.Unmarshal(data, &numValue); err == nil {
		*f = FlexibleFloat(numValue)
		return nil
	}

	// Try to unmarshal as string
	var strValue string
	if err := json.Unmarshal(data, &strValue); err == nil {
		numValue, err := strconv.ParseFloat(strValue, 64)
		if err != nil {
			return fmt.Errorf("failed to parse string as float: %w", err)
		}
		*f = FlexibleFloat(numValue)
		return nil
	}

	return fmt.Errorf("value is neither a number nor a string")
}

// MarshalJSON implements custom marshaling for FlexibleFloat
func (f FlexibleFloat) MarshalJSON() ([]byte, error) {
	return json.Marshal(float64(f))
}

// Float64 returns the float64 value
func (f FlexibleFloat) Float64() float64 {
	return float64(f)
}

// Account represents a fee/fine account
type Account struct {
	ID             string        `json:"id"`
	UserID         string        `json:"userId"`
	ItemID         string        `json:"itemId,omitempty"`
	MaterialTypeID string        `json:"materialTypeId,omitempty"`
	FeeFineID      string        `json:"feeFineId"`
	FeeFineType    string        `json:"feeFineType"`
	FeeFineOwner   string        `json:"feeFineOwner"`
	Amount         FlexibleFloat `json:"amount"`
	Remaining      FlexibleFloat `json:"remaining"`
	DateCreated    *time.Time    `json:"dateCreated,omitempty"`
	DateUpdated    *time.Time    `json:"dateUpdated,omitempty"`
	Status         AccountStatus `json:"status"`
	PaymentStatus  PaymentStatus `json:"paymentStatus"`
	LoanID         string        `json:"loanId,omitempty"`
	CallNumber     string        `json:"callNumber,omitempty"`
	Barcode        string        `json:"barcode,omitempty"`
	MaterialType   string        `json:"materialType,omitempty"`
	Title          string        `json:"title,omitempty"`
	DueDate        *time.Time    `json:"dueDate,omitempty"`
	ReturnedDate   *time.Time    `json:"returnedDate,omitempty"`
	Contributors   []Contributor `json:"contributors,omitempty"`
	Metadata       Metadata      `json:"metadata,omitempty"`
}

// AccountStatus represents the status of an account
type AccountStatus struct {
	Name string `json:"name"` // Open, Closed
}

// PaymentStatus represents the payment status of an account
type PaymentStatus struct {
	Name string `json:"name"` // Outstanding, Paid partially, Paid fully, etc.
}

// AccountCollection represents a collection of accounts
type AccountCollection struct {
	Accounts     []Account  `json:"accounts"`
	TotalRecords int        `json:"totalRecords"`
	ResultInfo   ResultInfo `json:"resultInfo,omitempty"`
}

// FeeFineAction represents an action on a fee/fine
type FeeFineAction struct {
	ID                     string     `json:"id"`
	DateAction             *time.Time `json:"dateAction"`
	TypeAction             string     `json:"typeAction"` // Payment, Waive, Refund, etc.
	Comments               string     `json:"comments,omitempty"`
	Notify                 bool       `json:"notify"`
	AmountAction           float64    `json:"amountAction"`
	Balance                float64    `json:"balance"`
	TransactionInformation string     `json:"transactionInformation,omitempty"`
	CreatedAt              string     `json:"createdAt"`
	Source                 string     `json:"source"`
	PaymentMethod          string     `json:"paymentMethod,omitempty"`
	AccountID              string     `json:"accountId"`
	UserID                 string     `json:"userId"`
	Metadata               Metadata   `json:"metadata,omitempty"`
}

// FeeFineActionCollection represents a collection of fee/fine actions
type FeeFineActionCollection struct {
	FeeFineActions []FeeFineAction `json:"feefineactions"`
	TotalRecords   int             `json:"totalRecords"`
}

// Payment represents a payment request for bulk payment operations
type Payment struct {
	Amount                 string   `json:"amount"`
	TransactionInformation string   `json:"transactionInfo,omitempty"`
	ServicePointID         string   `json:"servicePointId"`
	UserName               string   `json:"userName"`
	PaymentMethod          string   `json:"paymentMethod"`
	NotifyPatron           bool     `json:"notifyPatron"`
	Comments               string   `json:"comments,omitempty"`
	AccountIds             []string `json:"accountIds"`
}

// PaymentRequest represents a payment request for a single account
// Used with POST /accounts/{accountId}/pay endpoint
type PaymentRequest struct {
	Amount         string `json:"amount"`
	NotifyPatron   bool    `json:"notifyPatron"`
	ServicePointID string  `json:"servicePointId"`
	UserName       string  `json:"userName"`
	PaymentMethod  string  `json:"paymentMethod"`
	Comments       string  `json:"comments,omitempty"`
}

// IsOpen checks if the account is open
func (a *Account) IsOpen() bool {
	return a.Status.Name == "Open"
}

// IsOutstanding checks if the account has outstanding balance
func (a *Account) IsOutstanding() bool {
	return a.Remaining.Float64() > 0
}

// IsPaid checks if the account is paid
func (a *Account) IsPaid() bool {
	return a.PaymentStatus.Name == "Paid fully"
}

// GetTotalOutstanding calculates total outstanding balance from accounts
func (ac *AccountCollection) GetTotalOutstanding() float64 {
	total := 0.0
	for _, account := range ac.Accounts {
		if account.IsOutstanding() {
			total += account.Remaining.Float64()
		}
	}
	return total
}

// GetOpenAccounts returns all open accounts
func (ac *AccountCollection) GetOpenAccounts() []Account {
	var openAccounts []Account
	for _, account := range ac.Accounts {
		if account.IsOpen() {
			openAccounts = append(openAccounts, account)
		}
	}
	return openAccounts
}

// PaymentResponse represents the response from POST /accounts/{accountId}/pay
type PaymentResponse struct {
	AccountID       string          `json:"accountId"`
	Amount          string          `json:"amount"`
	RemainingAmount string          `json:"remainingAmount"`
	FeeFineActions  []FeeFineAction `json:"feefineactions"`
}
