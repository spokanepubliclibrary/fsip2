package testutil

import "fmt"

// testSIP2DateTime is a fixed timestamp used in all test SIP2 messages.
const testSIP2DateTime = "20250110    081500"

// NewLoginMessage builds a SIP2 Login (93) message.
func NewLoginMessage(username, password string) string {
	return fmt.Sprintf("9300|CN%s|CO%s\r", username, password)
}

// NewPatronStatusMessage builds a SIP2 Patron Status (23) message.
func NewPatronStatusMessage(institutionID, patronBarcode string) string {
	return fmt.Sprintf("23000%s|AO%s|AA%s\r", testSIP2DateTime, institutionID, patronBarcode)
}

// NewCheckoutMessage builds a SIP2 Checkout (11) message.
func NewCheckoutMessage(institutionID, patronBarcode, itemBarcode string) string {
	return fmt.Sprintf("11YN%s%s|AO%s|AA%s|AB%s\r",
		testSIP2DateTime, testSIP2DateTime, institutionID, patronBarcode, itemBarcode)
}

// NewCheckinMessage builds a SIP2 Checkin (09) message.
func NewCheckinMessage(institutionID, itemBarcode string) string {
	return fmt.Sprintf("09N%s%s|AO%s|AB%s\r",
		testSIP2DateTime, testSIP2DateTime, institutionID, itemBarcode)
}

// NewPatronInformationMessage builds a SIP2 Patron Information (63) message.
func NewPatronInformationMessage(institutionID, patronBarcode string) string {
	return fmt.Sprintf("63001%s          |AO%s|AA%s\r",
		testSIP2DateTime, institutionID, patronBarcode)
}

// NewRenewalMessage builds a SIP2 Renew (29) message.
func NewRenewalMessage(institutionID, patronBarcode, itemBarcode string) string {
	return fmt.Sprintf("29YN%s%s|AO%s|AA%s|AB%s\r",
		testSIP2DateTime, testSIP2DateTime, institutionID, patronBarcode, itemBarcode)
}

// NewRenewAllMessage builds a SIP2 Renew All (65) message.
func NewRenewAllMessage(institutionID, patronBarcode string) string {
	return fmt.Sprintf("6520250110    081500|AO%s|AA%s\r", institutionID, patronBarcode)
}

// NewItemInformationMessage builds a SIP2 Item Information (17) message.
func NewItemInformationMessage(institutionID, itemBarcode string) string {
	return fmt.Sprintf("17%s|AO%s|AB%s\r", testSIP2DateTime, institutionID, itemBarcode)
}

// NewFeePaidMessage builds a SIP2 Fee Paid (37) message.
func NewFeePaidMessage(institutionID, patronBarcode string, amount float64) string {
	return fmt.Sprintf("3701USD%s|AO%s|AA%s|BV%.2f\r",
		testSIP2DateTime, institutionID, patronBarcode, amount)
}
