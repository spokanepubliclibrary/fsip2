package parser

// MessageCode represents a SIP2 message type code
type MessageCode string

// SIP2 Request Message Codes
const (
	// Block Patron (01) - Not implemented
	BlockPatron MessageCode = "01"

	// Checkin (09/10)
	CheckinRequest  MessageCode = "09"
	CheckinResponse MessageCode = "10"

	// Checkout (11/12)
	CheckoutRequest  MessageCode = "11"
	CheckoutResponse MessageCode = "12"

	// Hold (15/16) - Not implemented
	HoldRequest  MessageCode = "15"
	HoldResponse MessageCode = "16"

	// Item Information (17/18)
	ItemInformationRequest  MessageCode = "17"
	ItemInformationResponse MessageCode = "18"

	// Item Status Update (19/20)
	ItemStatusUpdateRequest  MessageCode = "19"
	ItemStatusUpdateResponse MessageCode = "20"

	// Patron Status (23/24)
	PatronStatusRequest  MessageCode = "23"
	PatronStatusResponse MessageCode = "24"

	// Patron Enable (25/26) - Not implemented
	PatronEnableRequest  MessageCode = "25"
	PatronEnableResponse MessageCode = "26"

	// Renew (29/30)
	RenewRequest  MessageCode = "29"
	RenewResponse MessageCode = "30"

	// End Patron Session (35/36)
	EndPatronSessionRequest  MessageCode = "35"
	EndPatronSessionResponse MessageCode = "36"

	// Fee Paid (37/38)
	FeePaidRequest  MessageCode = "37"
	FeePaidResponse MessageCode = "38"

	// Patron Information (63/64)
	PatronInformationRequest  MessageCode = "63"
	PatronInformationResponse MessageCode = "64"

	// Renew All (65/66)
	RenewAllRequest  MessageCode = "65"
	RenewAllResponse MessageCode = "66"

	// Login (93/94)
	LoginRequest  MessageCode = "93"
	LoginResponse MessageCode = "94"

	// Request SC Resend (96)
	RequestSCResend MessageCode = "96"

	// Request ACS Resend (97)
	RequestACSResend MessageCode = "97"

	// ACS Status (98)
	ACSStatus MessageCode = "98"

	// SC Status (99)
	SCStatus MessageCode = "99"
)

// IsRequestMessage returns true if the message code is a request message
func (m MessageCode) IsRequestMessage() bool {
	requests := []MessageCode{
		BlockPatron,
		CheckinRequest,
		CheckoutRequest,
		HoldRequest,
		ItemInformationRequest,
		ItemStatusUpdateRequest,
		PatronStatusRequest,
		PatronEnableRequest,
		RenewRequest,
		EndPatronSessionRequest,
		FeePaidRequest,
		PatronInformationRequest,
		RenewAllRequest,
		LoginRequest,
		RequestACSResend,
		SCStatus,
	}

	for _, req := range requests {
		if m == req {
			return true
		}
	}
	return false
}

// IsResponseMessage returns true if the message code is a response message
func (m MessageCode) IsResponseMessage() bool {
	responses := []MessageCode{
		CheckinResponse,
		CheckoutResponse,
		HoldResponse,
		ItemInformationResponse,
		ItemStatusUpdateResponse,
		PatronStatusResponse,
		PatronEnableResponse,
		RenewResponse,
		EndPatronSessionResponse,
		FeePaidResponse,
		PatronInformationResponse,
		RenewAllResponse,
		LoginResponse,
		RequestSCResend,
		ACSStatus,
	}

	for _, resp := range responses {
		if m == resp {
			return true
		}
	}
	return false
}

// String returns the string representation of the message code
func (m MessageCode) String() string {
	return string(m)
}

// GetResponseCode returns the corresponding response code for a request code
func (m MessageCode) GetResponseCode() MessageCode {
	responseMap := map[MessageCode]MessageCode{
		BlockPatron:              "",
		CheckinRequest:           CheckinResponse,
		CheckoutRequest:          CheckoutResponse,
		HoldRequest:              HoldResponse,
		ItemInformationRequest:   ItemInformationResponse,
		ItemStatusUpdateRequest:  ItemStatusUpdateResponse,
		PatronStatusRequest:      PatronStatusResponse,
		PatronEnableRequest:      PatronEnableResponse,
		RenewRequest:             RenewResponse,
		EndPatronSessionRequest:  EndPatronSessionResponse,
		FeePaidRequest:           FeePaidResponse,
		PatronInformationRequest: PatronInformationResponse,
		RenewAllRequest:          RenewAllResponse,
		LoginRequest:             LoginResponse,
		RequestACSResend:         RequestSCResend,
		SCStatus:                 ACSStatus,
	}

	return responseMap[m]
}

// MessageName returns a human-readable name for the message code
func (m MessageCode) MessageName() string {
	names := map[MessageCode]string{
		BlockPatron:               "Block Patron",
		CheckinRequest:            "Checkin Request",
		CheckinResponse:           "Checkin Response",
		CheckoutRequest:           "Checkout Request",
		CheckoutResponse:          "Checkout Response",
		HoldRequest:               "Hold Request",
		HoldResponse:              "Hold Response",
		ItemInformationRequest:    "Item Information Request",
		ItemInformationResponse:   "Item Information Response",
		ItemStatusUpdateRequest:   "Item Status Update Request",
		ItemStatusUpdateResponse:  "Item Status Update Response",
		PatronStatusRequest:       "Patron Status Request",
		PatronStatusResponse:      "Patron Status Response",
		PatronEnableRequest:       "Patron Enable Request",
		PatronEnableResponse:      "Patron Enable Response",
		RenewRequest:              "Renew Request",
		RenewResponse:             "Renew Response",
		EndPatronSessionRequest:   "End Patron Session Request",
		EndPatronSessionResponse:  "End Patron Session Response",
		FeePaidRequest:            "Fee Paid Request",
		FeePaidResponse:           "Fee Paid Response",
		PatronInformationRequest:  "Patron Information Request",
		PatronInformationResponse: "Patron Information Response",
		RenewAllRequest:           "Renew All Request",
		RenewAllResponse:          "Renew All Response",
		LoginRequest:              "Login Request",
		LoginResponse:             "Login Response",
		RequestSCResend:           "Request SC Resend",
		RequestACSResend:          "Request ACS Resend",
		ACSStatus:                 "ACS Status",
		SCStatus:                  "SC Status",
	}

	if name, ok := names[m]; ok {
		return name
	}
	return "Unknown Message"
}
