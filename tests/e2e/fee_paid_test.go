// +build e2e

package e2e

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/spokanepubliclibrary/fsip2/internal/folio/models"
	"github.com/spokanepubliclibrary/fsip2/tests/testutil"
)

// addBulkPatron registers a patron + three accounts (mixed balances) on the mock server.
// Returns the patron barcode.
func addBulkPatron(setup *E2ESetup, barcode, userID string, accounts []*models.Account) {
	setup.MockFolio.AddUser(barcode, &models.User{
		ID:       userID,
		Username: barcode,
		Barcode:  barcode,
		Active:   true,
	})
	for _, a := range accounts {
		setup.MockFolio.AddAccount(a)
	}
}

// mixedAccounts returns three open accounts totalling $3.00 (matching the bug scenario).
func mixedAccounts(userID string) []*models.Account {
	return []*models.Account{
		{
			ID:            "acc-a",
			UserID:        userID,
			FeeFineID:     "ff-a",
			Remaining:     models.FlexibleFloat(0.50),
			Status:        models.AccountStatus{Name: "Open"},
			PaymentStatus: models.PaymentStatus{Name: "Outstanding"},
		},
		{
			ID:            "acc-b",
			UserID:        userID,
			FeeFineID:     "ff-b",
			Remaining:     models.FlexibleFloat(0.50),
			Status:        models.AccountStatus{Name: "Open"},
			PaymentStatus: models.PaymentStatus{Name: "Outstanding"},
		},
		{
			ID:            "acc-c",
			UserID:        userID,
			FeeFineID:     "ff-c",
			Remaining:     models.FlexibleFloat(2.00),
			Status:        models.AccountStatus{Name: "Open"},
			PaymentStatus: models.PaymentStatus{Name: "Outstanding"},
		},
	}
}

// ========== Bulk Payment Scenarios ==========

// TestE2E_FeePaid_BulkMixedBalances is the bug-scenario regression test:
// three accounts ($0.50, $0.50, $2.00), no CG field, amount=$3.00.
// Expects 38Y response and exactly one POST to /accounts-bulk/pay.
func TestE2E_FeePaid_BulkMixedBalances(t *testing.T) {
	setup := NewE2ESetupBulk(t)
	defer setup.Close(t)

	const barcode = "BULK_PATRON_MIXED"
	const userID = "user-bulk-mixed"
	addBulkPatron(setup, barcode, userID, mixedAccounts(userID))

	conn := setup.Connect(t)
	defer conn.Close()
	setup.Login(t, conn)

	t.Log("Sending Fee Paid (37) with mixed-balance accounts, no CG field")
	resp := setup.Exchange(t, conn, testutil.NewFeePaidMessage("test-inst", barcode, 3.00))

	require.True(t, len(resp) >= 3, "Response too short: %s", resp)
	assert.Equal(t, "38", resp[:2], "Expected Fee Paid Response (38), got: %s", resp)
	assert.Equal(t, byte('Y'), resp[2], "Expected payment accepted (Y), got: %c — full: %s", resp[2], resp)
	assert.Equal(t, 1, setup.MockFolio.BulkPayCallCount, "Expected exactly 1 call to /accounts-bulk/pay")
}

// TestE2E_FeePaid_BulkOverpaymentCapped tests that an amount exceeding total outstanding
// is capped to totalOutstanding before being sent to /accounts-bulk/pay.
func TestE2E_FeePaid_BulkOverpaymentCapped(t *testing.T) {
	setup := NewE2ESetupBulk(t)
	defer setup.Close(t)

	const barcode = "BULK_PATRON_OVER"
	const userID = "user-bulk-over"
	addBulkPatron(setup, barcode, userID, mixedAccounts(userID))

	conn := setup.Connect(t)
	defer conn.Close()
	setup.Login(t, conn)

	t.Log("Sending $5.00 against $3.00 total outstanding — amount must be capped to 3.00")
	resp := setup.Exchange(t, conn, testutil.NewFeePaidMessage("test-inst", barcode, 5.00))

	require.True(t, len(resp) >= 3, "Response too short: %s", resp)
	assert.Equal(t, "38", resp[:2], "Expected Fee Paid Response (38), got: %s", resp)
	assert.Equal(t, byte('Y'), resp[2], "Expected payment accepted (Y), got: %c — full: %s", resp[2], resp)
	require.Equal(t, 1, setup.MockFolio.BulkPayCallCount, "Expected exactly 1 call to /accounts-bulk/pay")
	assert.Equal(t, "3.00", setup.MockFolio.BulkPayRequests[0].Amount,
		"Bulk pay amount must be capped to 3.00 (totalOutstanding), got: %s",
		setup.MockFolio.BulkPayRequests[0].Amount)
}

// TestE2E_FeePaid_BulkPartialPayment tests that a partial payment (less than total
// outstanding) is passed through unchanged to /accounts-bulk/pay.
func TestE2E_FeePaid_BulkPartialPayment(t *testing.T) {
	setup := NewE2ESetupBulk(t)
	defer setup.Close(t)

	const barcode = "BULK_PATRON_PARTIAL"
	const userID = "user-bulk-partial"
	// Three accounts totalling $5.00: $1.00, $2.00, $2.00
	accounts := []*models.Account{
		{
			ID:            "acc-p1",
			UserID:        userID,
			FeeFineID:     "ff-p1",
			Remaining:     models.FlexibleFloat(1.00),
			Status:        models.AccountStatus{Name: "Open"},
			PaymentStatus: models.PaymentStatus{Name: "Outstanding"},
		},
		{
			ID:            "acc-p2",
			UserID:        userID,
			FeeFineID:     "ff-p2",
			Remaining:     models.FlexibleFloat(2.00),
			Status:        models.AccountStatus{Name: "Open"},
			PaymentStatus: models.PaymentStatus{Name: "Outstanding"},
		},
		{
			ID:            "acc-p3",
			UserID:        userID,
			FeeFineID:     "ff-p3",
			Remaining:     models.FlexibleFloat(2.00),
			Status:        models.AccountStatus{Name: "Open"},
			PaymentStatus: models.PaymentStatus{Name: "Outstanding"},
		},
	}
	addBulkPatron(setup, barcode, userID, accounts)

	conn := setup.Connect(t)
	defer conn.Close()
	setup.Login(t, conn)

	t.Log("Sending $3.00 against $5.00 total outstanding — amount passed through unchanged")
	resp := setup.Exchange(t, conn, testutil.NewFeePaidMessage("test-inst", barcode, 3.00))

	require.True(t, len(resp) >= 3, "Response too short: %s", resp)
	assert.Equal(t, "38", resp[:2], "Expected Fee Paid Response (38), got: %s", resp)
	assert.Equal(t, byte('Y'), resp[2], "Expected payment accepted (Y), got: %c — full: %s", resp[2], resp)
	require.Equal(t, 1, setup.MockFolio.BulkPayCallCount, "Expected exactly 1 call to /accounts-bulk/pay")
	assert.Equal(t, "3.00", setup.MockFolio.BulkPayRequests[0].Amount,
		"Bulk pay amount must be 3.00 (partial), got: %s",
		setup.MockFolio.BulkPayRequests[0].Amount)
}

// TestE2E_FeePaid_BulkNoEligibleAccounts tests that when all accounts have
// PaymentStatus="Suspended claim returned", the response is 38N (declined).
func TestE2E_FeePaid_BulkNoEligibleAccounts(t *testing.T) {
	setup := NewE2ESetupBulk(t)
	defer setup.Close(t)

	const barcode = "BULK_PATRON_SUSP"
	const userID = "user-bulk-susp"
	suspendedAccounts := []*models.Account{
		{
			ID:            "acc-s1",
			UserID:        userID,
			FeeFineID:     "ff-s1",
			Remaining:     models.FlexibleFloat(5.00),
			Status:        models.AccountStatus{Name: "Open"},
			PaymentStatus: models.PaymentStatus{Name: "Suspended claim returned"},
		},
		{
			ID:            "acc-s2",
			UserID:        userID,
			FeeFineID:     "ff-s2",
			Remaining:     models.FlexibleFloat(3.00),
			Status:        models.AccountStatus{Name: "Open"},
			PaymentStatus: models.PaymentStatus{Name: "Suspended claim returned"},
		},
	}
	addBulkPatron(setup, barcode, userID, suspendedAccounts)

	conn := setup.Connect(t)
	defer conn.Close()
	setup.Login(t, conn)

	t.Log("All accounts suspended — expect 38N (declined)")
	resp := setup.Exchange(t, conn, testutil.NewFeePaidMessage("test-inst", barcode, 3.00))

	require.True(t, len(resp) >= 3, "Response too short: %s", resp)
	assert.Equal(t, "38", resp[:2], "Expected Fee Paid Response (38), got: %s", resp)
	assert.Equal(t, byte('N'), resp[2], "Expected payment declined (N), got: %c — full: %s", resp[2], resp)
	assert.Equal(t, 0, setup.MockFolio.BulkPayCallCount, "/accounts-bulk/pay must NOT be called")
}

// TestE2E_FeePaid_BulkDisabled tests that when AcceptBulkPayment=false and no CG field
// is provided, the response is 38N and /accounts-bulk/pay is never called.
func TestE2E_FeePaid_BulkDisabled(t *testing.T) {
	// Use the standard setup (AcceptBulkPayment=false)
	setup := NewE2ESetup(t)
	defer setup.Close(t)

	const barcode = "BULK_PATRON_DISABLED"
	const userID = "user-bulk-disabled"
	addBulkPatron(setup, barcode, userID, mixedAccounts(userID))

	conn := setup.Connect(t)
	defer conn.Close()
	setup.Login(t, conn)

	t.Log("Bulk payment disabled — no CG field — expect 38N")
	resp := setup.Exchange(t, conn, testutil.NewFeePaidMessage("test-inst", barcode, 3.00))

	require.True(t, len(resp) >= 3, "Response too short: %s", resp)
	assert.Equal(t, "38", resp[:2], "Expected Fee Paid Response (38), got: %s", resp)
	assert.Equal(t, byte('N'), resp[2], "Expected payment declined (N), got: %c — full: %s", resp[2], resp)
	assert.Equal(t, 0, setup.MockFolio.BulkPayCallCount, "/accounts-bulk/pay must NOT be called when bulk disabled")
}

// TestE2E_FeePaid_SingleAccountPathUnchanged tests that when a CG (FeeIdentifier)
// field is provided, the single-account path is used via POST /accounts/{id}/pay.
func TestE2E_FeePaid_SingleAccountPathUnchanged(t *testing.T) {
	setup := NewE2ESetupBulk(t)
	defer setup.Close(t)

	const barcode = "SINGLE_PATRON"
	const userID = "user-single"
	const accountID = "acc-single-001"

	setup.MockFolio.AddUser(barcode, &models.User{
		ID:       userID,
		Username: barcode,
		Barcode:  barcode,
		Active:   true,
	})
	setup.MockFolio.AddAccount(&models.Account{
		ID:            accountID,
		UserID:        userID,
		FeeFineID:     "ff-single-001",
		Remaining:     models.FlexibleFloat(10.00),
		Status:        models.AccountStatus{Name: "Open"},
		PaymentStatus: models.PaymentStatus{Name: "Outstanding"},
	})

	conn := setup.Connect(t)
	defer conn.Close()
	setup.Login(t, conn)

	t.Log("CG field provided — single-account path must be used, not bulk")
	// Build a Fee Paid message with CG (FeeIdentifier) field set
	msg := "3701USD20250110    081500|AOtest-inst|AA" + barcode + "|BV10.00|CG" + accountID + "\r"
	resp := setup.Exchange(t, conn, msg)

	require.True(t, len(resp) >= 3, "Response too short: %s", resp)
	assert.Equal(t, "38", resp[:2], "Expected Fee Paid Response (38), got: %s", resp)
	assert.Equal(t, byte('Y'), resp[2], "Expected payment accepted (Y), got: %c — full: %s", resp[2], resp)
	// /accounts-bulk/pay must NOT have been called
	assert.Equal(t, 0, setup.MockFolio.BulkPayCallCount, "/accounts-bulk/pay must NOT be called for single-account path")
}
