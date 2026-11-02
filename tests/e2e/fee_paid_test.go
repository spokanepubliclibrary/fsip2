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
	const accountID = "4f1c9a2e-6b3d-4e8f-a1c5-7d9e2b4f6a80" // was "acc-single-001"; CG must be a UUID

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
	assert.Equal(t, []string{accountID}, setup.MockFolio.PayAccountRequests, "the CG account must be paid via /accounts/{id}/pay")
}

// ========== More accounts than FOLIO's default page (plan 04, F1) ==========

const (
	manyFeesBarcode = "MANY_FEES_PATRON"
	manyFeesUserID  = "user-many-fees"
	manyFeesTotal   = "120.00" // many_fees.json: $1.00 + $2.00 + ... + $15.00
)

// addManyFeesPatron registers a patron owning the 15 open accounts of many_fees.json
// and returns their IDs. The mock serves only 10 records unless a limit is sent.
func addManyFeesPatron(t *testing.T, setup *E2ESetup) []string {
	accounts := testutil.MustLoadAccounts("many_fees")
	require.Len(t, accounts, 15, "many_fees.json must hold 15 accounts")
	ids := make([]string, 0, len(accounts))
	ptrs := make([]*models.Account, 0, len(accounts))
	for i := range accounts {
		accounts[i].UserID = manyFeesUserID
		ids = append(ids, accounts[i].ID)
		ptrs = append(ptrs, &accounts[i])
	}
	addBulkPatron(setup, manyFeesBarcode, manyFeesUserID, ptrs)
	return ids
}

// TestE2E_PatronInformation_ManyFeesTotal tests that SIP 63 BV covers all 15 open
// accounts, not just FOLIO's default page of 10.
func TestE2E_PatronInformation_ManyFeesTotal(t *testing.T) {
	setup := NewE2ESetup(t)
	defer setup.Close(t)
	addManyFeesPatron(t, setup)

	conn := setup.Connect(t)
	defer conn.Close()
	setup.Login(t, conn)

	resp := setup.Exchange(t, conn, testutil.NewPatronInformationMessage("test-inst", manyFeesBarcode))

	require.True(t, len(resp) >= 2, "Response too short: %s", resp)
	assert.Equal(t, "64", resp[:2], "Expected Patron Information Response (64), got: %s", resp)
	assert.Equal(t, manyFeesTotal, findSIP2Field(resp, "BV"), "BV must be the sum of all 15 accounts — full: %s", resp)
}

// TestE2E_FeePaid_BulkManyFees tests that a bulk payment of the full balance pays all
// 15 open accounts with the full amount (no cap at the first page of 10).
func TestE2E_FeePaid_BulkManyFees(t *testing.T) {
	setup := NewE2ESetupBulk(t)
	defer setup.Close(t)
	ids := addManyFeesPatron(t, setup)

	conn := setup.Connect(t)
	defer conn.Close()
	setup.Login(t, conn)

	resp := setup.Exchange(t, conn, testutil.NewFeePaidMessage("test-inst", manyFeesBarcode, 120.00))

	require.True(t, len(resp) >= 3, "Response too short: %s", resp)
	assert.Equal(t, "38", resp[:2], "Expected Fee Paid Response (38), got: %s", resp)
	assert.Equal(t, byte('Y'), resp[2], "Expected payment accepted (Y), got: %c — full: %s", resp[2], resp)
	require.Equal(t, 1, setup.MockFolio.BulkPayCallCount, "Expected exactly 1 call to /accounts-bulk/pay")
	assert.ElementsMatch(t, ids, setup.MockFolio.BulkPayRequests[0].AccountIds, "bulk payment must cover all 15 accounts")
	assert.Equal(t, manyFeesTotal, setup.MockFolio.BulkPayRequests[0].Amount, "bulk payment must carry the full amount")
}

// ========== Single-account ownership and CG validation (D-04-4) ==========

const (
	crossPatronABarcode = "CROSS_PATRON_A"
	crossPatronAUserID  = "user-cross-a"
	crossPatronBBarcode = "CROSS_PATRON_B"
	crossPatronBUserID  = "user-cross-b"
	// patron B's open account, which patron A's kiosk session must never pay
	crossPatronBAccount = "9b7d5c3a-1e2f-4a6b-8c9d-0e1f2a3b4c5d"
)

// addCrossPatrons registers patron A (three open accounts, $3.00) and patron B
// (one open account) on the mock server.
func addCrossPatrons(setup *E2ESetup) {
	addBulkPatron(setup, crossPatronABarcode, crossPatronAUserID, mixedAccounts(crossPatronAUserID))
	addBulkPatron(setup, crossPatronBBarcode, crossPatronBUserID, []*models.Account{{
		ID:            crossPatronBAccount,
		UserID:        crossPatronBUserID,
		FeeFineID:     "ff-cross-b",
		Remaining:     models.FlexibleFloat(10.00),
		Status:        models.AccountStatus{Name: "Open"},
		PaymentStatus: models.PaymentStatus{Name: "Outstanding"},
	}})
}

// feePaidWithCG builds a Fee Paid (37) message for patron barcode with a CG field.
func feePaidWithCG(barcode, amount, cg string) string {
	return "3701USD20250110    081500|AOtest-inst|AA" + barcode + "|BV" + amount + "|CG" + cg + "\r"
}

// TestE2E_FeePaid_CrossPatronAccount tests that a CG naming another patron's open
// account is treated as not eligible: that account is never paid, and the AA
// patron's own fees are paid in bulk (bulk on) or the payment is declined (bulk off).
func TestE2E_FeePaid_CrossPatronAccount(t *testing.T) {
	t.Run("bulk enabled pays AA patron", func(t *testing.T) {
		setup := NewE2ESetupBulk(t)
		defer setup.Close(t)
		addCrossPatrons(setup)

		conn := setup.Connect(t)
		defer conn.Close()
		setup.Login(t, conn)

		resp := setup.Exchange(t, conn, feePaidWithCG(crossPatronABarcode, "3.00", crossPatronBAccount))

		require.True(t, len(resp) >= 3, "Response too short: %s", resp)
		assert.Equal(t, "38", resp[:2], "Expected Fee Paid Response (38), got: %s", resp)
		assert.Equal(t, byte('Y'), resp[2], "Expected bulk fallback accepted (Y), got: %c — full: %s", resp[2], resp)
		assert.NotContains(t, setup.MockFolio.PayAccountRequests, crossPatronBAccount, "patron B's account must never be paid")
		require.Equal(t, 1, setup.MockFolio.BulkPayCallCount, "Expected exactly 1 call to /accounts-bulk/pay")
		assert.ElementsMatch(t, []string{"acc-a", "acc-b", "acc-c"}, setup.MockFolio.BulkPayRequests[0].AccountIds,
			"bulk fallback must pay only patron A's accounts")
	})

	t.Run("bulk disabled declines", func(t *testing.T) {
		setup := NewE2ESetup(t)
		defer setup.Close(t)
		addCrossPatrons(setup)

		conn := setup.Connect(t)
		defer conn.Close()
		setup.Login(t, conn)

		resp := setup.Exchange(t, conn, feePaidWithCG(crossPatronABarcode, "3.00", crossPatronBAccount))

		require.True(t, len(resp) >= 3, "Response too short: %s", resp)
		assert.Equal(t, "38", resp[:2], "Expected Fee Paid Response (38), got: %s", resp)
		assert.Equal(t, byte('N'), resp[2], "Expected payment declined (N), got: %c — full: %s", resp[2], resp)
		assert.Empty(t, setup.MockFolio.PayAccountRequests, "no account may be paid")
		assert.Equal(t, 0, setup.MockFolio.BulkPayCallCount, "/accounts-bulk/pay must NOT be called when bulk disabled")
	})
}

// TestE2E_FeePaid_NonUUIDCGFallsBackToBulk tests that a CG that is not a UUID
// (a CQL mask or a quote-injection attempt) never pays a single account and
// falls back to bulk payment of the AA patron's own fees.
func TestE2E_FeePaid_NonUUIDCGFallsBackToBulk(t *testing.T) {
	for _, cg := range []string{"*", `x" or id=="y`} {
		t.Run(cg, func(t *testing.T) {
			setup := NewE2ESetupBulk(t)
			defer setup.Close(t)
			addCrossPatrons(setup)

			conn := setup.Connect(t)
			defer conn.Close()
			setup.Login(t, conn)

			resp := setup.Exchange(t, conn, feePaidWithCG(crossPatronABarcode, "3.00", cg))

			require.True(t, len(resp) >= 3, "Response too short: %s", resp)
			assert.Equal(t, "38", resp[:2], "Expected Fee Paid Response (38), got: %s", resp)
			assert.Equal(t, byte('Y'), resp[2], "Expected bulk fallback accepted (Y), got: %c — full: %s", resp[2], resp)
			assert.Empty(t, setup.MockFolio.PayAccountRequests, "no single account may be paid for a non-UUID CG")
			require.Equal(t, 1, setup.MockFolio.BulkPayCallCount, "Expected exactly 1 call to /accounts-bulk/pay")
			assert.ElementsMatch(t, []string{"acc-a", "acc-b", "acc-c"}, setup.MockFolio.BulkPayRequests[0].AccountIds,
				"bulk fallback must pay only patron A's accounts")
		})
	}
}
