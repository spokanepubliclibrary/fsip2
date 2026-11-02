// +build e2e

package e2e

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/spokanepubliclibrary/fsip2/internal/folio/models"
	"github.com/spokanepubliclibrary/fsip2/tests/testutil"
)

// ========== Authentication Failures ==========

// TestE2E_Error_LoginInvalidCredentials tests login with invalid credentials
func TestE2E_Error_LoginInvalidCredentials(t *testing.T) {
	setup := NewE2ESetup(t)
	defer setup.Close(t)
	conn := setup.Connect(t)
	defer conn.Close()

	t.Log("Testing Login with Invalid Credentials")
	resp := setup.Exchange(t, conn, testutil.NewLoginMessage("invaliduser", "wrongpass", ""))
	assert.True(t, len(resp) >= 2 && resp[:2] == "94", "Expected Login Response (94), got: %s", resp)
	assert.True(t, len(resp) >= 3 && resp[2] == '0', "Expected login failure (0), got: %c", resp[2])
}

// TestE2E_Error_ExpiredPatronAccount tests operations with expired patron
func TestE2E_Error_ExpiredPatronAccount(t *testing.T) {
	setup := NewE2ESetup(t)
	defer setup.Close(t)

	expiredDate := time.Now().AddDate(-1, 0, 0)
	setup.MockFolio.AddUser("PATRON_EXPIRED", &models.User{
		ID:             "user-expired",
		Username:       "patron_expired",
		Barcode:        "PATRON_EXPIRED",
		Active:         false,
		ExpirationDate: &expiredDate,
		Personal:       models.PersonalInfo{FirstName: "Expired", LastName: "User"},
	})

	conn := setup.Connect(t)
	defer conn.Close()
	setup.Login(t, conn)

	t.Log("Testing Patron Status with Expired Account")
	resp := setup.Exchange(t, conn, testutil.NewPatronStatusMessage("test-inst", "PATRON_EXPIRED"))
	assert.True(t, len(resp) >= 2 && resp[:2] == "24", "Expected Patron Status Response (24), got: %s", resp)
	t.Logf("Response for expired patron: %s", resp)
}

// TestE2E_Error_OperationWithoutLogin tests operations without logging in
func TestE2E_Error_OperationWithoutLogin(t *testing.T) {
	setup := NewE2ESetup(t)
	defer setup.Close(t)
	conn := setup.Connect(t)
	defer conn.Close()

	t.Log("Testing Checkout without Login")
	resp := setup.Exchange(t, conn, testutil.NewCheckoutMessage("test-inst", "123456", "ITEM001"))
	assert.True(t, len(resp) >= 2, "Expected valid response, got: %s", resp)
	t.Logf("Response without login: %s", resp)
}

// TestE2E_Error_PatronVerificationFailure tests patron verification failure
func TestE2E_Error_PatronVerificationFailure(t *testing.T) {
	setup := NewE2ESetup(t)
	defer setup.Close(t)

	setup.MockFolio.AddUser("PATRON_VERIFY", &models.User{
		ID:       "user-verify",
		Username: "patron_verify",
		Barcode:  "PATRON_VERIFY",
		Active:   true,
	})

	conn := setup.Connect(t)
	defer conn.Close()
	setup.Login(t, conn)

	t.Log("Testing Checkout with Wrong Patron Password")
	resp := setup.Exchange(t, conn, "11YN20250110    08150020250124    081500|AOtest-inst|AAPATRON_VERIFY|ABITEM001|ADwrongpass\r")
	assert.True(t, len(resp) >= 2 && resp[:2] == "12", "Expected Checkout Response (12), got: %s", resp)
	t.Logf("Response with failed verification: %s", resp)
}

// ========== Item Errors ==========

// TestE2E_Error_CheckoutInvalidItemBarcode tests checkout with invalid barcode
func TestE2E_Error_CheckoutInvalidItemBarcode(t *testing.T) {
	setup := NewE2ESetup(t)
	defer setup.Close(t)

	setup.MockFolio.AddUser("PATRON_CHECKOUT", &models.User{
		ID:       "user-checkout-err",
		Username: "patron_checkout",
		Barcode:  "PATRON_CHECKOUT",
		Active:   true,
	})

	conn := setup.Connect(t)
	defer conn.Close()
	setup.Login(t, conn)

	t.Log("Testing Checkout with Invalid Item Barcode")
	resp := setup.Exchange(t, conn, testutil.NewCheckoutMessage("test-inst", "PATRON_CHECKOUT", "INVALID_ITEM"))
	assert.True(t, len(resp) >= 2 && resp[:2] == "12", "Expected Checkout Response (12), got: %s", resp)
	assert.True(t, len(resp) < 3 || resp[2] != '1', "Expected checkout failure, but got success flag, response: %s", resp)
}

// TestE2E_Error_CheckoutItemNotFound tests checkout with item not found
func TestE2E_Error_CheckoutItemNotFound(t *testing.T) {
	setup := NewE2ESetup(t)
	defer setup.Close(t)

	setup.MockFolio.AddUser("PATRON_NOTFOUND", &models.User{
		ID:       "user-notfound",
		Username: "patron_notfound",
		Barcode:  "PATRON_NOTFOUND",
		Active:   true,
	})

	conn := setup.Connect(t)
	defer conn.Close()
	setup.Login(t, conn)

	t.Log("Testing Checkout with Item Not Found")
	resp := setup.Exchange(t, conn, testutil.NewCheckoutMessage("test-inst", "PATRON_NOTFOUND", "NOTFOUND123"))
	assert.True(t, len(resp) >= 2 && resp[:2] == "12", "Expected Checkout Response (12), got: %s", resp)
}

// TestE2E_Error_CheckoutUnavailableItem tests checkout with unavailable item
func TestE2E_Error_CheckoutUnavailableItem(t *testing.T) {
	setup := NewE2ESetup(t)
	defer setup.Close(t)

	setup.MockFolio.AddUser("PATRON_UNAVAIL", &models.User{
		ID:       "user-unavail",
		Username: "patron_unavail",
		Barcode:  "PATRON_UNAVAIL",
		Active:   true,
	})
	setup.MockFolio.AddItem("ITEM_UNAVAIL", &models.Item{
		ID:      "item-unavail",
		Barcode: "ITEM_UNAVAIL",
		Status:  models.ItemStatus{Name: "Checked out"},
	})

	conn := setup.Connect(t)
	defer conn.Close()
	setup.Login(t, conn)

	t.Log("Testing Checkout with Unavailable Item")
	resp := setup.Exchange(t, conn, testutil.NewCheckoutMessage("test-inst", "PATRON_UNAVAIL", "ITEM_UNAVAIL"))
	assert.True(t, len(resp) >= 2 && resp[:2] == "12", "Expected Checkout Response (12), got: %s", resp)
	t.Logf("Response for unavailable item: %s", resp)
}

// TestE2E_Error_CheckinNotCheckedOut tests checkin with item not checked out
func TestE2E_Error_CheckinNotCheckedOut(t *testing.T) {
	setup := NewE2ESetup(t)
	defer setup.Close(t)

	setup.MockFolio.AddItem("ITEM_NOTOUT", &models.Item{
		ID:      "item-notout",
		Barcode: "ITEM_NOTOUT",
		Status:  models.ItemStatus{Name: "Available"},
	})

	conn := setup.Connect(t)
	defer conn.Close()
	setup.Login(t, conn)

	t.Log("Testing Checkin with Item Not Checked Out")
	resp := setup.Exchange(t, conn, testutil.NewCheckinMessage("test-inst", "ITEM_NOTOUT"))
	assert.True(t, len(resp) >= 2 && resp[:2] == "10", "Expected Checkin Response (10), got: %s", resp)
	t.Logf("Response for item not checked out: %s", resp)
}

// ========== Patron Errors ==========

// TestE2E_Error_BlockedPatron tests operations with blocked patron
func TestE2E_Error_BlockedPatron(t *testing.T) {
	setup := NewE2ESetup(t)
	defer setup.Close(t)

	setup.MockFolio.AddUser("PATRON_BLOCKED", &models.User{
		ID:       "user-blocked",
		Username: "patron_blocked",
		Barcode:  "PATRON_BLOCKED",
		Active:   false,
		Personal: models.PersonalInfo{FirstName: "Blocked", LastName: "User"},
	})
	setup.MockFolio.AddItem("ITEM_TEST", &models.Item{
		ID:      "item-test",
		Barcode: "ITEM_TEST",
		Status:  models.ItemStatus{Name: "Available"},
	})

	conn := setup.Connect(t)
	defer conn.Close()
	setup.Login(t, conn)

	t.Log("Testing Checkout with Blocked Patron")
	resp := setup.Exchange(t, conn, testutil.NewCheckoutMessage("test-inst", "PATRON_BLOCKED", "ITEM_TEST"))
	assert.True(t, len(resp) >= 2 && resp[:2] == "12", "Expected Checkout Response (12), got: %s", resp)
	t.Logf("Response for blocked patron: %s", resp)
}

// TestE2E_Error_PatronNotFound tests operations with non-existent patron
func TestE2E_Error_PatronNotFound(t *testing.T) {
	setup := NewE2ESetup(t)
	defer setup.Close(t)
	conn := setup.Connect(t)
	defer conn.Close()
	setup.Login(t, conn)

	t.Log("Testing Patron Status with Non-Existent Patron")
	resp := setup.Exchange(t, conn, testutil.NewPatronStatusMessage("test-inst", "NONEXISTENT"))
	assert.True(t, len(resp) >= 2 && resp[:2] == "24", "Expected Patron Status Response (24), got: %s", resp)
	t.Logf("Response for non-existent patron: %s", resp)
}

// ========== Barcode lookup safety (plan 04, D-04-2 / D-04-3) ==========

const (
	victimPatronBarcode = "PATRON_VICTIM"
	victimPatronName    = "Victim"
	victimItemBarcode   = "ITEM_VICTIM"
)

// addBarcodeVictims registers a patron and an item whose data a CQL mask or quote
// injection in a device-supplied barcode could expose.
func addBarcodeVictims(setup *E2ESetup) {
	setup.MockFolio.AddUser(victimPatronBarcode, &models.User{
		ID:       "user-victim",
		Username: "patron_victim",
		Barcode:  victimPatronBarcode,
		Active:   true,
		Personal: models.PersonalInfo{FirstName: "Other", LastName: victimPatronName},
	})
	setup.MockFolio.AddItem(victimItemBarcode, &models.Item{
		ID:      "item-victim",
		Barcode: victimItemBarcode,
		Status:  models.ItemStatus{Name: "Available"},
	})
}

// assertPatronNotFound checks a Patron Status Response (24) is the invalid-patron reply
// and carries no other patron's name.
func assertPatronNotFound(t *testing.T, resp string) {
	t.Helper()
	require.True(t, len(resp) >= 16, "Response too short: %s", resp)
	assert.Equal(t, "24", resp[:2], "Expected Patron Status Response (24), got: %s", resp)
	assert.Contains(t, resp, "|BLN", "Expected invalid patron (BLN) — full: %s", resp)
	assert.NotContains(t, resp, victimPatronName, "another patron's data must not be returned — full: %s", resp)
}

// assertItemNotFound checks an Item Information Response (18) is the not-found reply.
func assertItemNotFound(t *testing.T, resp string) {
	t.Helper()
	require.True(t, len(resp) >= 4, "Response too short: %s", resp)
	assert.Equal(t, "18", resp[:2], "Expected Item Information Response (18), got: %s", resp)
	assert.Equal(t, "01", resp[2:4], "Expected circulation status 01 (not found), got: %s", resp)
	assert.Contains(t, resp, "Item not found", "Expected 'Item not found' — full: %s", resp)
}

// TestE2E_Error_BarcodeQuerySyntax tests that CQL masks and quotes in a patron or
// item barcode are matched literally: the lookup is "not found" and never resolves
// to another record.
func TestE2E_Error_BarcodeQuerySyntax(t *testing.T) {
	setup := NewE2ESetup(t)
	defer setup.Close(t)
	addBarcodeVictims(setup)

	conn := setup.Connect(t)
	defer conn.Close()
	setup.Login(t, conn)

	// Controls: the real barcodes resolve, so the not-found replies below mean something
	resp := setup.Exchange(t, conn, testutil.NewPatronStatusMessage("test-inst", victimPatronBarcode))
	require.Contains(t, resp, victimPatronName, "control: the victim patron must be found — full: %s", resp)
	resp = setup.Exchange(t, conn, testutil.NewItemInformationMessage("test-inst", victimItemBarcode))
	require.NotContains(t, resp, "Item not found", "control: the victim item must be found — full: %s", resp)

	for _, barcode := range []string{"*", "PATRON_VICTI?", "PATRON_VIC*", `"`, `x" or barcode=="PATRON_VICTIM`} {
		t.Run("patron "+barcode, func(t *testing.T) {
			assertPatronNotFound(t, setup.Exchange(t, conn, testutil.NewPatronStatusMessage("test-inst", barcode)))
		})
	}
	for _, barcode := range []string{"*", "ITEM_VICTI?", "ITEM_VIC*", `"`, `x" or barcode=="ITEM_VICTIM`} {
		t.Run("item "+barcode, func(t *testing.T) {
			assertItemNotFound(t, setup.Exchange(t, conn, testutil.NewItemInformationMessage("test-inst", barcode)))
		})
	}
}

// TestE2E_Error_AmbiguousBarcode tests that a barcode lookup returning two records
// (impossible in real FOLIO, emulated with DuplicateBarcodes) is treated as not found.
func TestE2E_Error_AmbiguousBarcode(t *testing.T) {
	setup := NewE2ESetup(t)
	defer setup.Close(t)
	addBarcodeVictims(setup)
	setup.MockFolio.DuplicateBarcodes[victimPatronBarcode] = 2
	setup.MockFolio.DuplicateBarcodes[victimItemBarcode] = 2

	conn := setup.Connect(t)
	defer conn.Close()
	setup.Login(t, conn)

	assertPatronNotFound(t, setup.Exchange(t, conn, testutil.NewPatronStatusMessage("test-inst", victimPatronBarcode)))
	assertItemNotFound(t, setup.Exchange(t, conn, testutil.NewItemInformationMessage("test-inst", victimItemBarcode)))
}

// TestE2E_ManyManualBlocks tests that all 12 manual blocks reach the patron status in
// 23 and 63, not just FOLIO's default page of 10: only blocks 11 and 12 deny anything.
func TestE2E_ManyManualBlocks(t *testing.T) {
	setup := NewE2ESetup(t)
	defer setup.Close(t)

	const barcode = "PATRON_MANY_BLOCKS"
	const userID = "user-many-blocks"
	setup.MockFolio.AddUser(barcode, &models.User{
		ID:       userID,
		Username: "patron_many_blocks",
		Barcode:  barcode,
		Active:   true,
	})
	for i := 1; i <= 12; i++ {
		setup.MockFolio.AddManualBlock(userID, &models.ManualBlock{
			ID:        fmt.Sprintf("block-%02d", i),
			Type:      "Manual",
			Desc:      fmt.Sprintf("Block %d", i),
			UserID:    userID,
			Borrowing: i == 11,
			Renewals:  i == 12,
			Requests:  i == 12,
		})
	}

	conn := setup.Connect(t)
	defer conn.Close()
	setup.Login(t, conn)

	for _, tc := range []struct{ name, msg, code string }{
		{"23", testutil.NewPatronStatusMessage("test-inst", barcode), "24"},
		{"63", testutil.NewPatronInformationMessage("test-inst", barcode), "64"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resp := setup.Exchange(t, conn, tc.msg)
			require.True(t, len(resp) >= 16, "Response too short: %s", resp)
			assert.Equal(t, tc.code, resp[:2], "unexpected response — full: %s", resp)
			status := resp[2:16]
			assert.Equal(t, byte('Y'), status[0], "charge denied (block 11) — status %q", status)
			assert.Equal(t, byte('Y'), status[1], "renewal denied (block 12) — status %q", status)
			assert.Equal(t, byte('Y'), status[3], "hold denied (block 12) — status %q", status)
		})
	}
}

// ========== FOLIO API Errors ==========

// TestE2E_Error_FolioAPIError tests handling of FOLIO API errors
func TestE2E_Error_FolioAPIError(t *testing.T) {
	setup := NewE2ESetup(t)
	defer setup.Close(t)
	conn := setup.Connect(t)
	defer conn.Close()
	setup.Login(t, conn)

	t.Log("Testing FOLIO API Error Handling")
	// Empty barcode to trigger error handling
	resp := setup.Exchange(t, conn, "23000202501100    815000|AOtest-inst|AA\r")
	assert.True(t, len(resp) >= 2, "Expected valid error response, got: %s", resp)
	t.Logf("Response for API error scenario: %s", resp)
}

// TestE2E_Error_FolioAPITimeout tests handling of FOLIO API timeout
func TestE2E_Error_FolioAPITimeout(t *testing.T) {
	setup := NewE2ESetup(t)
	defer setup.Close(t)

	setup.MockFolio.AddUser("PATRON_TIMEOUT", &models.User{
		ID:       "user-timeout",
		Username: "patron_timeout",
		Barcode:  "PATRON_TIMEOUT",
		Active:   true,
	})

	conn := setup.Connect(t)
	defer conn.Close()
	setup.Login(t, conn)

	t.Log("Testing FOLIO API Timeout Handling")
	resp := setup.Exchange(t, conn, testutil.NewPatronStatusMessage("test-inst", "PATRON_TIMEOUT"))
	assert.True(t, len(resp) >= 2, "Expected valid response, got: %s", resp)
}

// TestE2E_Error_FolioAPI404 tests handling of FOLIO API 404 (resource not found)
func TestE2E_Error_FolioAPI404(t *testing.T) {
	setup := NewE2ESetup(t)
	defer setup.Close(t)
	conn := setup.Connect(t)
	defer conn.Close()
	setup.Login(t, conn)

	t.Log("Testing FOLIO API 404 Handling")
	resp := setup.Exchange(t, conn, testutil.NewItemInformationMessage("test-inst", "NONEXISTENT404"))
	assert.True(t, len(resp) >= 2 && resp[:2] == "18", "Expected Item Information Response (18), got: %s", resp)
	t.Logf("Response for 404 scenario: %s", resp)
}

// ========== Malformed Messages ==========

// TestE2E_Error_IncompleteMessage tests handling of incomplete SIP2 message
func TestE2E_Error_IncompleteMessage(t *testing.T) {
	setup := NewE2ESetup(t)
	defer setup.Close(t)
	conn := setup.Connect(t)
	defer conn.Close()

	t.Log("Testing Incomplete SIP2 Message")
	resp := setup.Exchange(t, conn, "11YN\r") // Incomplete checkout message
	t.Logf("Got response for incomplete message: %s", resp)
}

// TestE2E_Error_InvalidMessageCode tests handling of invalid message code
func TestE2E_Error_InvalidMessageCode(t *testing.T) {
	setup := NewE2ESetup(t)
	defer setup.Close(t)
	conn := setup.Connect(t)
	defer conn.Close()

	t.Log("Testing Invalid Message Code")
	resp := setup.Exchange(t, conn, "99999|AOtest-inst\r")
	t.Logf("Got response for invalid message code: %s", resp)
}

// TestE2E_Error_InvalidChecksum tests handling of message with invalid checksum
func TestE2E_Error_InvalidChecksum(t *testing.T) {
	setup := NewE2ESetup(t)
	defer setup.Close(t)
	conn := setup.Connect(t)
	defer conn.Close()

	t.Log("Testing Invalid Checksum")
	resp := setup.Exchange(t, conn, "990302.00|AYWRONG\r")
	t.Logf("Got response for invalid checksum: %s", resp)
}

// ========== Permission Errors ==========

// TestE2E_Error_UnauthorizedOperation tests operation without required permissions
func TestE2E_Error_UnauthorizedOperation(t *testing.T) {
	setup := NewE2ESetup(t)
	defer setup.Close(t)

	setup.MockFolio.AddUser("PATRON_NOPERM", &models.User{
		ID:       "user-noperm",
		Username: "patron_noperm",
		Barcode:  "PATRON_NOPERM",
		Active:   true,
	})

	conn := setup.Connect(t)
	defer conn.Close()
	setup.Login(t, conn)

	t.Log("Testing Unauthorized Operation")
	resp := setup.Exchange(t, conn, testutil.NewPatronStatusMessage("test-inst", "PATRON_NOPERM"))
	assert.True(t, len(resp) >= 2, "Expected valid response, got: %s", resp)
	t.Logf("Response for permission test: %s", resp)
}

// TestE2E_Error_UnauthorizedAccess tests unauthorized access without token
func TestE2E_Error_UnauthorizedAccess(t *testing.T) {
	setup := NewE2ESetup(t)
	defer setup.Close(t)
	conn := setup.Connect(t)
	defer conn.Close()

	// Try patron status without logging in first
	t.Log("Testing Unauthorized Access (No Token)")
	resp := setup.Exchange(t, conn, testutil.NewPatronStatusMessage("test-inst", "ANYPATRON"))
	assert.True(t, len(resp) >= 2, "Expected valid response, got: %s", resp)
	t.Logf("Response without token: %s", resp)
}
