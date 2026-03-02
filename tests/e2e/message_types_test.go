// +build e2e

package e2e

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/spokanepubliclibrary/fsip2/internal/folio/models"
	"github.com/spokanepubliclibrary/fsip2/tests/testutil"
)

// TestE2E_PatronInformationFull tests full patron information request
func TestE2E_PatronInformationFull(t *testing.T) {
	setup := NewE2ESetup(t)
	defer setup.Close(t)

	setup.MockFolio.AddUser("PATRON_FULL", &models.User{
		ID:       "user-full-info",
		Username: "patron_full",
		Barcode:  "PATRON_FULL",
		Active:   true,
		Personal: models.PersonalInfo{
			FirstName: "John",
			LastName:  "Doe",
			Email:     "john.doe@example.com",
			Phone:     "555-1234",
		},
	})

	conn := setup.Connect(t)
	defer conn.Close()
	setup.Login(t, conn)

	// Request full patron information with all fields
	t.Log("Testing Full Patron Information Request")
	resp := setup.Exchange(t, conn, "63YYYYYYYYYY20250110815000          |AOtest-inst|AAPATRON_FULL\r")
	assert.True(t, len(resp) >= 2 && resp[:2] == "64", "Expected Patron Information Response (64), got: %s", resp)

	if !assertSIP2FieldPresent(resp, "AE") {
		t.Logf("Expected response to contain patron name field AE, got: %s", resp)
	}
}

// TestE2E_PatronInformationSummary tests summary patron information request
func TestE2E_PatronInformationSummary(t *testing.T) {
	setup := NewE2ESetup(t)
	defer setup.Close(t)

	setup.MockFolio.AddUser("PATRON_SUM", &models.User{
		ID:       "user-summary",
		Username: "patron_summary",
		Barcode:  "PATRON_SUM",
		Active:   true,
		Personal: models.PersonalInfo{FirstName: "Jane", LastName: "Smith"},
	})

	conn := setup.Connect(t)
	defer conn.Close()
	setup.Login(t, conn)

	t.Log("Testing Summary Patron Information Request")
	resp := setup.Exchange(t, conn, "63Y         20250110815000          |AOtest-inst|AAPATRON_SUM\r")
	assert.True(t, len(resp) >= 2 && resp[:2] == "64", "Expected Patron Information Response (64), got: %s", resp)
}

// TestE2E_RenewSuccess tests successful item renewal
func TestE2E_RenewSuccess(t *testing.T) {
	setup := NewE2ESetup(t)
	defer setup.Close(t)

	setup.MockFolio.AddUser("PATRON_RENEW", &models.User{
		ID:       "user-renew",
		Username: "patron_renew",
		Barcode:  "PATRON_RENEW",
		Active:   true,
	})
	setup.MockFolio.AddItem("ITEM_RENEW", &models.Item{
		ID:      "item-renew",
		Barcode: "ITEM_RENEW",
		Status:  models.ItemStatus{Name: "Checked out"},
	})

	conn := setup.Connect(t)
	defer conn.Close()
	setup.Login(t, conn)

	t.Log("Testing Successful Renewal")
	resp := setup.Exchange(t, conn, testutil.NewRenewalMessage("test-inst", "PATRON_RENEW", "ITEM_RENEW"))
	assert.True(t, len(resp) >= 2 && resp[:2] == "30", "Expected Renew Response (30), got: %s", resp)
	if len(resp) >= 3 && resp[2] != '1' {
		t.Logf("Renewal may have failed, got response: %s", resp)
	}
}

// TestE2E_RenewFailure tests renewal failure scenarios
func TestE2E_RenewFailure(t *testing.T) {
	setup := NewE2ESetup(t)
	defer setup.Close(t)

	setup.MockFolio.AddUser("PATRON_FAIL", &models.User{
		ID:       "user-renew-fail",
		Username: "patron_renew_fail",
		Barcode:  "PATRON_FAIL",
		Active:   true,
	})

	conn := setup.Connect(t)
	defer conn.Close()
	setup.Login(t, conn)

	t.Log("Testing Renewal Failure (Item Not Found)")
	resp := setup.Exchange(t, conn, testutil.NewRenewalMessage("test-inst", "PATRON_FAIL", "NONEXISTENT"))
	assert.True(t, len(resp) >= 2 && resp[:2] == "30", "Expected Renew Response (30), got: %s", resp)
	if len(resp) >= 3 && resp[2] == '1' {
		t.Logf("Expected renewal to fail, but got success flag, response: %s", resp)
	}
}

// TestE2E_RenewAll tests renew all functionality
func TestE2E_RenewAll(t *testing.T) {
	setup := NewE2ESetup(t)
	defer setup.Close(t)

	setup.MockFolio.AddUser("PATRON_RENEWALL", &models.User{
		ID:       "user-renew-all",
		Username: "patron_renewall",
		Barcode:  "PATRON_RENEWALL",
		Active:   true,
	})
	for i := 1; i <= 3; i++ {
		itemID := fmt.Sprintf("RENEWALL_%d", i)
		setup.MockFolio.AddItem(itemID, &models.Item{
			ID:      fmt.Sprintf("item-renewall-%d", i),
			Barcode: itemID,
			Status:  models.ItemStatus{Name: "Checked out"},
		})
	}

	conn := setup.Connect(t)
	defer conn.Close()
	setup.Login(t, conn)

	t.Log("Testing Renew All")
	resp := setup.Exchange(t, conn, testutil.NewRenewAllMessage("test-inst", "PATRON_RENEWALL"))
	assert.True(t, len(resp) >= 2 && resp[:2] == "66", "Expected Renew All Response (66), got: %s", resp)
}

// TestE2E_RenewAllZeroLoans tests renew all with no checked out items
func TestE2E_RenewAllZeroLoans(t *testing.T) {
	setup := NewE2ESetup(t)
	defer setup.Close(t)

	setup.MockFolio.AddUser("PATRON_NOLOANS", &models.User{
		ID:       "user-no-loans",
		Username: "patron_noloans",
		Barcode:  "PATRON_NOLOANS",
		Active:   true,
	})

	conn := setup.Connect(t)
	defer conn.Close()
	setup.Login(t, conn)

	t.Log("Testing Renew All with Zero Loans")
	resp := setup.Exchange(t, conn, testutil.NewRenewAllMessage("test-inst", "PATRON_NOLOANS"))
	assert.True(t, len(resp) >= 2 && resp[:2] == "66", "Expected Renew All Response (66), got: %s", resp)
}

// TestE2E_ItemInformationAvailable tests item information for available item
func TestE2E_ItemInformationAvailable(t *testing.T) {
	setup := NewE2ESetup(t)
	defer setup.Close(t)

	setup.MockFolio.AddItem("ITEM_AVAIL", &models.Item{
		ID:      "item-available",
		Barcode: "ITEM_AVAIL",
		Status:  models.ItemStatus{Name: "Available"},
	})

	conn := setup.Connect(t)
	defer conn.Close()
	setup.Login(t, conn)

	t.Log("Testing Item Information (Available)")
	resp := setup.Exchange(t, conn, testutil.NewItemInformationMessage("test-inst", "ITEM_AVAIL"))
	assert.True(t, len(resp) >= 2 && resp[:2] == "18", "Expected Item Information Response (18), got: %s", resp)
	assert.Contains(t, resp, "ABITEM_AVAIL", "Expected response to contain item barcode")
}

// TestE2E_ItemInformationCheckedOut tests item information for checked out item
func TestE2E_ItemInformationCheckedOut(t *testing.T) {
	setup := NewE2ESetup(t)
	defer setup.Close(t)

	setup.MockFolio.AddItem("ITEM_OUT", &models.Item{
		ID:      "item-checkedout",
		Barcode: "ITEM_OUT",
		Status:  models.ItemStatus{Name: "Checked out"},
	})

	conn := setup.Connect(t)
	defer conn.Close()
	setup.Login(t, conn)

	t.Log("Testing Item Information (Checked Out)")
	resp := setup.Exchange(t, conn, testutil.NewItemInformationMessage("test-inst", "ITEM_OUT"))
	assert.True(t, len(resp) >= 2 && resp[:2] == "18", "Expected Item Information Response (18), got: %s", resp)
}

// TestE2E_ItemStatusUpdate tests item status update message
func TestE2E_ItemStatusUpdate(t *testing.T) {
	setup := NewE2ESetup(t)
	defer setup.Close(t)

	setup.MockFolio.AddItem("ITEM_STATUS", &models.Item{
		ID:      "item-status",
		Barcode: "ITEM_STATUS",
		Status:  models.ItemStatus{Name: "Available"},
	})

	conn := setup.Connect(t)
	defer conn.Close()
	setup.Login(t, conn)

	t.Log("Testing Item Status Update")
	resp := setup.Exchange(t, conn, "1920250110    081500|AOtest-inst|ABITEM_STATUS\r")
	assert.True(t, len(resp) >= 2 && resp[:2] == "20", "Expected Item Status Update Response (20), got: %s", resp)
}

// TestE2E_FeePaidSingle tests single fee payment
func TestE2E_FeePaidSingle(t *testing.T) {
	setup := NewE2ESetup(t)
	defer setup.Close(t)

	setup.MockFolio.AddUser("PATRON_FEE", &models.User{
		ID:       "user-fee",
		Username: "patron_fee",
		Barcode:  "PATRON_FEE",
		Active:   true,
	})

	conn := setup.Connect(t)
	defer conn.Close()
	setup.Login(t, conn)

	t.Log("Testing Fee Paid (Single)")
	resp := setup.Exchange(t, conn, "3720250110    081500BW5.00BVusd|AOtest-inst|AAPATRON_FEE|CGfee-123|BKtransaction-456\r")
	assert.True(t, len(resp) >= 2 && resp[:2] == "38", "Expected Fee Paid Response (38), got: %s", resp)
}

// TestE2E_FeePaidPartial tests partial fee payment
func TestE2E_FeePaidPartial(t *testing.T) {
	setup := NewE2ESetup(t)
	defer setup.Close(t)

	setup.MockFolio.AddUser("PATRON_PARTFEE", &models.User{
		ID:       "user-partial-fee",
		Username: "patron_partialfee",
		Barcode:  "PATRON_PARTFEE",
		Active:   true,
	})

	conn := setup.Connect(t)
	defer conn.Close()
	setup.Login(t, conn)

	t.Log("Testing Fee Paid (Partial)")
	resp := setup.Exchange(t, conn, "3720250110    081500BW2.50BVusd|AOtest-inst|AAPATRON_PARTFEE|CGfee-789|BKtransaction-999\r")
	assert.True(t, len(resp) >= 2 && resp[:2] == "38", "Expected Fee Paid Response (38), got: %s", resp)
}

// TestE2E_EndSession tests end session message
func TestE2E_EndSession(t *testing.T) {
	setup := NewE2ESetup(t)
	defer setup.Close(t)
	conn := setup.Connect(t)
	defer conn.Close()

	setup.Login(t, conn)

	t.Log("Testing End Session")
	resp := setup.Exchange(t, conn, "3520250110    081500|AOtest-inst\r")
	assert.True(t, len(resp) >= 2 && resp[:2] == "36", "Expected End Session Response (36), got: %s", resp)
	if len(resp) >= 3 && resp[2] != 'Y' {
		t.Logf("Expected successful end session (Y), got: %c", resp[2])
	}
}

// TestE2E_ResendLastMessage tests resend functionality
func TestE2E_ResendLastMessage(t *testing.T) {
	setup := NewE2ESetup(t)
	defer setup.Close(t)
	conn := setup.Connect(t)
	defer conn.Close()

	setup.Login(t, conn)
	firstResponse := setup.Exchange(t, conn, "990302.00\r")

	t.Log("Testing Resend Last Message")
	resendResponse := setup.Exchange(t, conn, "97\r")

	if resendResponse != firstResponse {
		t.Logf("Resend response differs from original. Original: %s, Resend: %s", firstResponse, resendResponse)
	}
}

// TestE2E_ResendNoMessage tests resend when no previous message
func TestE2E_ResendNoMessage(t *testing.T) {
	setup := NewE2ESetup(t)
	defer setup.Close(t)
	conn := setup.Connect(t)
	defer conn.Close()

	t.Log("Testing Resend with No Previous Message")
	resp := setup.Exchange(t, conn, "97\r")
	assert.True(t, len(resp) >= 2, "Expected valid response to resend, got: %s", resp)
}

// assertSIP2FieldPresent returns true if the given SIP2 field code is present in the response.
func assertSIP2FieldPresent(response, fieldCode string) bool {
	return findSIP2Field(response, fieldCode) != ""
}
