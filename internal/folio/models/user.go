package models

import "time"

// User represents a FOLIO user/patron
type User struct {
	ID               string                 `json:"id"`
	Username         string                 `json:"username"`
	ExternalSystemID string                 `json:"externalSystemId"`
	Barcode          string                 `json:"barcode"`
	Active           bool                   `json:"active"`
	Type             string                 `json:"type"`
	PatronGroup      string                 `json:"patronGroup"`
	Personal         PersonalInfo           `json:"personal"`
	EnrollmentDate   *time.Time             `json:"enrollmentDate,omitempty"`
	ExpirationDate   *time.Time             `json:"expirationDate,omitempty"`
	CreatedDate      *time.Time             `json:"createdDate,omitempty"`
	UpdatedDate      *time.Time             `json:"updatedDate,omitempty"`
	Metadata         Metadata               `json:"metadata,omitempty"`
	CustomFields     map[string]interface{} `json:"customFields,omitempty"`
}

// PersonalInfo represents personal information for a user
type PersonalInfo struct {
	LastName               string     `json:"lastName"`
	FirstName              string     `json:"firstName"`
	PreferredFirstName     string     `json:"preferredFirstName,omitempty"`
	MiddleName             string     `json:"middleName,omitempty"`
	Email                  string     `json:"email,omitempty"`
	Phone                  string     `json:"phone,omitempty"`
	MobilePhone            string     `json:"mobilePhone,omitempty"`
	DateOfBirth            *time.Time `json:"dateOfBirth,omitempty"`
	Addresses              []Address  `json:"addresses,omitempty"`
	PreferredContactTypeID string     `json:"preferredContactTypeId,omitempty"`
}

// Address represents an address
type Address struct {
	ID             string `json:"id,omitempty"`
	CountryID      string `json:"countryId,omitempty"`
	AddressLine1   string `json:"addressLine1,omitempty"`
	AddressLine2   string `json:"addressLine2,omitempty"`
	City           string `json:"city,omitempty"`
	Region         string `json:"region,omitempty"`
	PostalCode     string `json:"postalCode,omitempty"`
	AddressTypeID  string `json:"addressTypeId,omitempty"`
	PrimaryAddress bool   `json:"primaryAddress,omitempty"`
}

// UserCollection represents a collection of users
type UserCollection struct {
	Users        []User `json:"users"`
	TotalRecords int    `json:"totalRecords"`
}

// ManualBlock represents a manual block on a patron
type ManualBlock struct {
	ID               string     `json:"id"`
	Type             string     `json:"type"` // Manual, Automated
	Desc             string     `json:"desc"`
	Code             string     `json:"code,omitempty"`
	StaffInformation string     `json:"staffInformation,omitempty"`
	PatronMessage    string     `json:"patronMessage,omitempty"`
	ExpirationDate   *time.Time `json:"expirationDate,omitempty"`
	Borrowing        bool       `json:"borrowing"`
	Renewals         bool       `json:"renewals"`
	Requests         bool       `json:"requests"`
	UserID           string     `json:"userId"`
	Metadata         Metadata   `json:"metadata,omitempty"`
}

// ManualBlockCollection represents a collection of manual blocks
type ManualBlockCollection struct {
	ManualBlocks []ManualBlock `json:"manualblocks"`
	TotalRecords int           `json:"totalRecords"`
}

// AutomatedPatronBlock represents automated patron blocks
type AutomatedPatronBlock struct {
	AutomatedPatronBlocks []AutomatedBlock `json:"automatedPatronBlocks"`
}

// AutomatedBlock represents a single automated block
type AutomatedBlock struct {
	PatronBlockConditionID string `json:"patronBlockConditionId"`
	BlockBorrowing         bool   `json:"blockBorrowing"`
	BlockRenewals          bool   `json:"blockRenewals"`
	BlockRequests          bool   `json:"blockRequests"`
	Message                string `json:"message"`
}

// Metadata represents common metadata fields
type Metadata struct {
	CreatedDate     *time.Time `json:"createdDate,omitempty"`
	CreatedByUserID string     `json:"createdByUserId,omitempty"`
	UpdatedDate     *time.Time `json:"updatedDate,omitempty"`
	UpdatedByUserID string     `json:"updatedByUserId,omitempty"`
}

// GetFullName returns the user's full name
func (u *User) GetFullName() string {
	fullName := u.Personal.FirstName
	if u.Personal.MiddleName != "" {
		fullName += " " + u.Personal.MiddleName
	}
	fullName += " " + u.Personal.LastName
	return fullName
}

// GetPrimaryAddress returns the user's primary address or first address
func (u *User) GetPrimaryAddress() *Address {
	for _, addr := range u.Personal.Addresses {
		if addr.PrimaryAddress {
			return &addr
		}
	}
	if len(u.Personal.Addresses) > 0 {
		return &u.Personal.Addresses[0]
	}
	return nil
}

// IsExpired checks if the user account is expired
func (u *User) IsExpired() bool {
	if u.ExpirationDate == nil {
		return false
	}
	return u.ExpirationDate.Before(time.Now())
}

// HasBlocks checks if the user has any blocks (manual or automated)
func (mb *ManualBlockCollection) HasBorrowingBlock() bool {
	for _, block := range mb.ManualBlocks {
		if block.Borrowing {
			return true
		}
	}
	return false
}

// HasRenewalsBlock checks if blocks prevent renewals
func (mb *ManualBlockCollection) HasRenewalsBlock() bool {
	for _, block := range mb.ManualBlocks {
		if block.Renewals {
			return true
		}
	}
	return false
}

// HasRequestsBlock checks if blocks prevent requests
func (mb *ManualBlockCollection) HasRequestsBlock() bool {
	for _, block := range mb.ManualBlocks {
		if block.Requests {
			return true
		}
	}
	return false
}

// GetCustomField retrieves a custom field value by key
// Returns the value and true if found, nil and false if not found
func (u *User) GetCustomField(key string) (interface{}, bool) {
	if u.CustomFields == nil {
		return nil, false
	}
	value, exists := u.CustomFields[key]
	return value, exists
}
