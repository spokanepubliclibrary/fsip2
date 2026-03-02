package parser

// FieldCode represents a SIP2 field identifier (2 characters)
type FieldCode string

// SIP2 Field Codes
const (
	// Patron and Item Identifiers
	PatronIdentifier FieldCode = "AA" // Patron identifier (barcode)
	ItemIdentifier   FieldCode = "AB" // Item identifier (barcode)
	TerminalPassword FieldCode = "AC" // Terminal password
	PatronPassword   FieldCode = "AD" // Patron password (PIN)
	PersonalName     FieldCode = "AE" // Personal name

	// Messages and Alerts
	ScreenMessage FieldCode = "AF" // Screen message
	PrintLine     FieldCode = "AG" // Print line

	// Dates and Times
	DueDate             FieldCode = "AH" // Due date
	ValidPatronPassword FieldCode = "CQ" // Valid patron password (Y/N)

	// Title and Location
	TitleIdentifier   FieldCode = "AJ" // Title identifier
	InstitutionID     FieldCode = "AO" // Institution ID
	CurrentLocation   FieldCode = "AP" // Current location (service point)
	PermanentLocation FieldCode = "AQ" // Permanent location

	// Library and Item Information
	LibraryName        FieldCode = "AM" // Library name
	TerminalLocation   FieldCode = "AN" // Terminal location
	HoldPickupLocation FieldCode = "BS" // Hold pickup location
	CallNumber         FieldCode = "CS" // Call number

	// Transaction Information
	SequenceNumber FieldCode = "AY" // Sequence number (for error detection)
	Checksum       FieldCode = "AZ" // Checksum (4 hex digits)

	// Patron Status Flags
	ChargePrivilegesDenied  FieldCode = "BH" // Charge privileges denied (Y/N)
	RenewalPrivilegesDenied FieldCode = "BI" // Renewal privileges denied (Y/N)
	RecallPrivilegesDenied  FieldCode = "BJ" // Recall privileges denied (Y/N)
	HoldPrivilegesDenied    FieldCode = "BK" // Hold privileges denied (Y/N)
	ValidPatron             FieldCode = "BL" // Valid patron (Y/N)
	CardReportedLost        FieldCode = "CD" // Card reported lost (Y/N)

	// Fee and Fine Information
	FeeAmount     FieldCode = "BV" // Fee amount
	FeeType       FieldCode = "BT" // Fee type
	FeeIdentifier FieldCode = "CG" // Fee identifier
	PaymentType   FieldCode = "BX" // Payment type (cash, credit, etc.)
	TransactionID FieldCode = "BW" // Transaction ID
	CurrencyType  FieldCode = "BY" // Currency type

	// Item Status Information
	PermanentItemType FieldCode = "CB" // Permanent item type
	CurrentItemType   FieldCode = "CH" // Current item type
	DesensitizeItem   FieldCode = "CI" // Desensitize item (Y/N)
	ResensitizeItem   FieldCode = "CL" // Resensitize item (Y/N)
	SecurityMarker    FieldCode = "CM" // Security marker
	MediaType         FieldCode = "CK" // Media type
	AlertType         FieldCode = "CV" // Alert type

	// Hold and Request Information
	HoldQueueLength         FieldCode = "CF" // Hold queue length
	PickupServicePoint      FieldCode = "CT" // Pickup service point
	RequestorBarcode        FieldCode = "CY" // Requestor barcode (patron who placed hold)
	HoldShelfExpirationDate FieldCode = "CM" // Hold shelf expiration date (overloaded with SecurityMarker)

	// Counts and Lists
	HoldItemsCount        FieldCode = "AS" // Hold items count
	OverdueItemsCount     FieldCode = "AT" // Overdue items count
	ChargedItemsCount     FieldCode = "AU" // Charged items count
	FineItemsCount        FieldCode = "AV" // Fine items count
	RecallItemsCount      FieldCode = "BU" // Recall items count
	UnavailableHoldsCount FieldCode = "CR" // Unavailable holds count

	// Item Lists (multiple instances allowed) - These use the same codes as counts in SIP2 spec
	HoldItems        FieldCode = "AS" // Hold items (list)
	OverdueItems     FieldCode = "AT" // Overdue items (list)
	ChargedItems     FieldCode = "AU" // Charged items (list)
	FineItems        FieldCode = "AV" // Fine items (list)
	RecallItems      FieldCode = "BU" // Recall items (list)
	UnavailableHolds FieldCode = "CR" // Unavailable holds (list)

	// Login and Session
	LoginUserID   FieldCode = "CN" // Login user ID
	LoginPassword FieldCode = "CO" // Login password
	LocationCode  FieldCode = "CP" // Location code

	// Renewal Information
	RenewalOK FieldCode = "OK" // Renewal OK (Y/N)

	// Checkout/Checkin Dates
	TransactionDate FieldCode = "DA" // Transaction date

	// Status Codes
	OK            FieldCode = "OK" // OK status (Y/N/U)
	RenewalStatus FieldCode = "OK" // Renewal status (for renewal response)

	// Summary Information (Patron Info)
	Summary FieldCode = "BP" // Summary (for patron information request)

	// Start/End Item
	StartItem FieldCode = "BP" // Start item (for pagination) - same as Summary in SIP2
	EndItem   FieldCode = "BQ" // End item (for pagination)

	// Language
	Language FieldCode = "LG" // Language (3-character code)

	// Home Address
	HomeAddress     FieldCode = "BD" // Home address
	EmailAddress    FieldCode = "BE" // Email address
	HomePhoneNumber FieldCode = "BF" // Home phone number

	// Owner
	Owner FieldCode = "BG" // Owner

	// ISBNs and Identifiers
	ISBN            FieldCode = "BN" // ISBN (legacy code)
	ISBNIdentifier  FieldCode = "IN" // ISBN identifier (preferred)
	OtherStandardID FieldCode = "NB" // Other standard identifier (UPC, etc.)

	// LCCN
	LCCN FieldCode = "BO" // LCCN (Library of Congress Control Number)

	// Instance/Bibliographic Information
	PrimaryContributor FieldCode = "EA" // Primary contributor (author)
	WorkDescription    FieldCode = "DE" // Work description (summary note)
)

// String returns the string representation of the field code
func (f FieldCode) String() string {
	return string(f)
}

// FieldName returns a human-readable name for the field code
func (f FieldCode) FieldName() string {
	names := map[FieldCode]string{
		PatronIdentifier:        "Patron Identifier",
		ItemIdentifier:          "Item Identifier",
		TerminalPassword:        "Terminal Password",
		PatronPassword:          "Patron Password",
		PersonalName:            "Personal Name",
		ScreenMessage:           "Screen Message",
		PrintLine:               "Print Line",
		DueDate:                 "Due Date",
		ValidPatronPassword:     "Valid Patron Password",
		TitleIdentifier:         "Title Identifier",
		InstitutionID:           "Institution ID",
		CurrentLocation:         "Current Location",
		PermanentLocation:       "Permanent Location",
		LibraryName:             "Library Name",
		TerminalLocation:        "Terminal Location",
		HoldPickupLocation:      "Hold Pickup Location",
		CallNumber:              "Call Number",
		SequenceNumber:          "Sequence Number",
		Checksum:                "Checksum",
		ChargePrivilegesDenied:  "Charge Privileges Denied",
		RenewalPrivilegesDenied: "Renewal Privileges Denied",
		RecallPrivilegesDenied:  "Recall Privileges Denied",
		HoldPrivilegesDenied:    "Hold Privileges Denied",
		ValidPatron:             "Valid Patron",
		CardReportedLost:        "Card Reported Lost",
		FeeAmount:               "Fee Amount",
		FeeType:                 "Fee Type",
		FeeIdentifier:           "Fee Identifier",
		PaymentType:             "Payment Type",
		TransactionID:           "Transaction ID",
		CurrencyType:            "Currency Type",
		PermanentItemType:       "Permanent Item Type",
		CurrentItemType:         "Current Item Type",
		DesensitizeItem:         "Desensitize Item",
		ResensitizeItem:         "Resensitize Item",
		MediaType:               "Media Type",
		AlertType:               "Alert Type",
		HoldQueueLength:         "Hold Queue Length",
		PickupServicePoint:      "Pickup Service Point",
		RequestorBarcode:        "Requestor Barcode",
		HoldShelfExpirationDate: "Hold Shelf Expiration Date / Security Marker",
		HoldItemsCount:          "Hold Items Count/List",
		OverdueItemsCount:       "Overdue Items Count/List",
		ChargedItemsCount:       "Charged Items Count/List",
		FineItemsCount:          "Fine Items Count/List",
		RecallItemsCount:        "Recall Items Count/List",
		UnavailableHoldsCount:   "Unavailable Holds Count/List",
		LoginUserID:             "Login User ID",
		LoginPassword:           "Login Password",
		LocationCode:            "Location Code",
		RenewalOK:               "Renewal OK/OK Status",
		TransactionDate:         "Transaction Date",
		Summary:                 "Summary/Start Item",
		EndItem:                 "End Item",
		Language:                "Language",
		HomeAddress:             "Home Address",
		EmailAddress:            "Email Address",
		HomePhoneNumber:         "Home Phone Number",
		Owner:                   "Owner",
		ISBN:                    "ISBN",
		ISBNIdentifier:          "ISBN Identifier",
		OtherStandardID:         "Other Standard Identifier",
		LCCN:                    "LCCN",
		PrimaryContributor:      "Primary Contributor",
		WorkDescription:         "Work Description",
	}

	if name, ok := names[f]; ok {
		return name
	}
	return "Unknown Field"
}

// IsSensitive returns true if the field contains sensitive data (passwords, PINs)
func (f FieldCode) IsSensitive() bool {
	sensitiveFields := []FieldCode{
		PatronPassword,
		TerminalPassword,
		LoginPassword,
	}

	for _, sensitive := range sensitiveFields {
		if f == sensitive {
			return true
		}
	}
	return false
}
