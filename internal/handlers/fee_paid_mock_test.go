package handlers

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/spokanepubliclibrary/fsip2/internal/folio/models"
	"github.com/spokanepubliclibrary/fsip2/internal/sip2/parser"
	"github.com/spokanepubliclibrary/fsip2/tests/testutil"
)

// feePaidMsg builds a FeePaid request with a specific patron barcode and amount.
func feePaidMsg(barcode, amount string, extraFields ...map[parser.FieldCode]string) *parser.Message {
	fields := map[parser.FieldCode]string{
		parser.InstitutionID:    "TEST-INST",
		parser.PatronIdentifier: barcode,
		parser.FeeAmount:        amount,
	}
	for _, extra := range extraFields {
		for k, v := range extra {
			fields[k] = v
		}
	}
	return buildTestMsg(parser.FeePaidRequest, fields)
}

// TestFeePaidHandle_SinglePayment_Success verifies that providing an account ID
// results in a single successful payment (38Y response).
func TestFeePaidHandle_SinglePayment_Success(t *testing.T) {
	tc := testutil.NewTenantConfig()
	// Empty patron ID → handler calls GetUserByBarcode; user.ID flows to fee calls.
	sess := testutil.NewAuthSession(tc, testutil.WithSessionUser("testuser", "", ""))
	user := makeTestUser()

	account := &models.Account{
		ID:        "acc-001",
		FeeFineID: "ff-001",
		Remaining: models.FlexibleFloat(10.00),
	}
	payResp := &models.PaymentResponse{RemainingAmount: "0.00"}

	mockPatron := &MockPatronClient{}
	mockFees := &MockFeesClient{}

	mockPatron.On("GetUserByBarcode", mock.Anything, mock.Anything, user.Barcode).Return(user, nil)
	mockFees.On("GetEligibleAccountByID", mock.Anything, mock.Anything, "acc-001").Return(account, nil)
	mockFees.On("PayAccount", mock.Anything, mock.Anything, "acc-001", mock.Anything).Return(payResp, nil)

	h := NewFeePaidHandler(zap.NewNop(), tc)
	injectMocks(h.BaseHandler, mockPatron, nil, nil, mockFees)

	msg := feePaidMsg(user.Barcode, "10.00", map[parser.FieldCode]string{
		parser.FeeIdentifier: "acc-001",
	})

	resp, err := h.Handle(context.Background(), msg, sess)

	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(resp, "38"), "response must start with 38")
	assert.Equal(t, byte('Y'), resp[2], "payment accepted flag must be Y")
	assert.Contains(t, resp, "|CGacc-001", "account ID in CG field")
	assert.Contains(t, resp, "Payment accepted")

	mockPatron.AssertExpectations(t)
	mockFees.AssertExpectations(t)
}

// TestFeePaidHandle_BulkPayment_AllSuccess verifies that when no account ID is
// provided and bulk payment is enabled, all accounts are paid successfully.
func TestFeePaidHandle_BulkPayment_AllSuccess(t *testing.T) {
	tc := testutil.NewTenantConfig()
	tc.AcceptBulkPayment = true
	sess := testutil.NewAuthSession(tc, testutil.WithSessionUser("testuser", "", ""))
	user := makeTestUser()

	accounts := &models.AccountCollection{
		Accounts: []models.Account{
			{ID: "acc-1", FeeFineID: "ff-1", Remaining: models.FlexibleFloat(5.00)},
			{ID: "acc-2", FeeFineID: "ff-2", Remaining: models.FlexibleFloat(5.00)},
			{ID: "acc-3", FeeFineID: "ff-3", Remaining: models.FlexibleFloat(5.00)},
		},
	}
	mockPatron := &MockPatronClient{}
	mockFees := &MockFeesClient{}

	mockPatron.On("GetUserByBarcode", mock.Anything, mock.Anything, user.Barcode).Return(user, nil)
	mockFees.On("GetOpenAccountsExcludingSuspended", mock.Anything, mock.Anything, user.ID).
		Return(accounts, nil)
	mockFees.On("PayBulkAccounts", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	h := NewFeePaidHandler(zap.NewNop(), tc)
	injectMocks(h.BaseHandler, mockPatron, nil, nil, mockFees)

	// No FeeIdentifier → bulk payment path.
	msg := feePaidMsg(user.Barcode, "15.00")

	resp, err := h.Handle(context.Background(), msg, sess)

	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(resp, "38"))
	assert.Equal(t, byte('Y'), resp[2], "bulk payment must be accepted")
	assert.Contains(t, resp, "Bulk payment applied")

	mockPatron.AssertExpectations(t)
	mockFees.AssertExpectations(t)
}

// TestFeePaidHandle_BulkPayment_APIError verifies that when PayBulkAccounts returns
// an error, the handler returns 38N (payment declined) without panicking.
func TestFeePaidHandle_BulkPayment_APIError(t *testing.T) {
	tc := testutil.NewTenantConfig()
	tc.AcceptBulkPayment = true
	sess := testutil.NewAuthSession(tc, testutil.WithSessionUser("testuser", "", ""))
	user := makeTestUser()

	accounts := &models.AccountCollection{
		Accounts: []models.Account{
			{ID: "acc-a", FeeFineID: "ff-a", Remaining: models.FlexibleFloat(5.00)},
			{ID: "acc-b", FeeFineID: "ff-b", Remaining: models.FlexibleFloat(5.00)},
			{ID: "acc-c", FeeFineID: "ff-c", Remaining: models.FlexibleFloat(5.00)},
		},
	}

	mockPatron := &MockPatronClient{}
	mockFees := &MockFeesClient{}

	mockPatron.On("GetUserByBarcode", mock.Anything, mock.Anything, user.Barcode).Return(user, nil)
	mockFees.On("GetOpenAccountsExcludingSuspended", mock.Anything, mock.Anything, user.ID).
		Return(accounts, nil)
	mockFees.On("PayBulkAccounts", mock.Anything, mock.Anything, mock.Anything).
		Return(assert.AnError)

	h := NewFeePaidHandler(zap.NewNop(), tc)
	injectMocks(h.BaseHandler, mockPatron, nil, nil, mockFees)

	msg := feePaidMsg(user.Barcode, "15.00")

	resp, err := h.Handle(context.Background(), msg, sess)

	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(resp, "38"))
	assert.Equal(t, byte('N'), resp[2], "response must be N when PayBulkAccounts errors")

	mockPatron.AssertExpectations(t)
	mockFees.AssertExpectations(t)
}

// TestFeePaidHandle_AccountNotEligible verifies that when GetEligibleAccountByID
// returns nil and bulk payment is disabled, the response indicates failure (38N).
func TestFeePaidHandle_AccountNotEligible(t *testing.T) {
	tc := testutil.NewTenantConfig()
	tc.AcceptBulkPayment = false // no bulk fallback
	sess := testutil.NewAuthSession(tc, testutil.WithSessionUser("testuser", "", ""))
	user := makeTestUser()

	mockPatron := &MockPatronClient{}
	mockFees := &MockFeesClient{}

	mockPatron.On("GetUserByBarcode", mock.Anything, mock.Anything, user.Barcode).Return(user, nil)
	// Returns nil account — not found/not eligible.
	mockFees.On("GetEligibleAccountByID", mock.Anything, mock.Anything, "bad-acc").
		Return(nil, nil)

	h := NewFeePaidHandler(zap.NewNop(), tc)
	injectMocks(h.BaseHandler, mockPatron, nil, nil, mockFees)

	msg := feePaidMsg(user.Barcode, "5.00", map[parser.FieldCode]string{
		parser.FeeIdentifier: "bad-acc",
	})

	resp, err := h.Handle(context.Background(), msg, sess)

	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(resp, "38"))
	assert.Equal(t, byte('N'), resp[2], "response must be N when account is not eligible")

	mockPatron.AssertExpectations(t)
	mockFees.AssertExpectations(t)
}

// TestPaySingleAccount_AccountNotEligible verifies that when GetEligibleAccountByID
// returns (nil, nil) the function returns fallback=true so the caller can attempt bulk payment.
func TestPaySingleAccount_AccountNotEligible(t *testing.T) {
	tc := testutil.NewTenantConfig()
	h := NewFeePaidHandler(zap.NewNop(), tc)

	mockFees := &MockFeesClient{}
	mockFees.On("GetEligibleAccountByID", mock.Anything, mock.Anything, "acc-001").
		Return(nil, nil)

	result, fallback := h.paySingleAccount(
		context.Background(), mockFees, "token", "acc-001",
		5.00, "", "testuser", "cash", false,
	)

	assert.False(t, result.success)
	assert.True(t, fallback, "should signal fallback to bulk payment")
	mockFees.AssertExpectations(t)
}

// TestPaySingleAccount_PaymentFails verifies that when PayAccount returns an error,
// the function returns success=false and fallback=false (hard failure, no fallback).
func TestPaySingleAccount_PaymentFails(t *testing.T) {
	tc := testutil.NewTenantConfig()
	h := NewFeePaidHandler(zap.NewNop(), tc)
	account := testutil.DefaultAccount()

	mockFees := &MockFeesClient{}
	mockFees.On("GetEligibleAccountByID", mock.Anything, mock.Anything, account.ID).
		Return(account, nil)
	mockFees.On("PayAccount", mock.Anything, mock.Anything, account.ID, mock.Anything).
		Return(nil, fmt.Errorf("payment gateway error"))

	result, fallback := h.paySingleAccount(
		context.Background(), mockFees, "token", account.ID,
		float64(account.Remaining), "", "testuser", "cash", false,
	)

	assert.False(t, result.success)
	assert.False(t, fallback, "hard payment failure should not trigger fallback")
	mockFees.AssertExpectations(t)
}

// TestPaySingleAccount_EligibilityCheckFails verifies that when GetEligibleAccountByID
// returns an error (not nil, nil), the function returns success=false and fallback=false.
func TestPaySingleAccount_EligibilityCheckFails(t *testing.T) {
	tc := testutil.NewTenantConfig()
	h := NewFeePaidHandler(zap.NewNop(), tc)

	mockFees := &MockFeesClient{}
	mockFees.On("GetEligibleAccountByID", mock.Anything, mock.Anything, "acc-001").
		Return(nil, fmt.Errorf("FOLIO unavailable"))

	result, fallback := h.paySingleAccount(
		context.Background(), mockFees, "token", "acc-001",
		5.00, "", "testuser", "cash", false,
	)

	assert.False(t, result.success)
	assert.False(t, fallback)
	mockFees.AssertExpectations(t)
}

// TestBulkPayment_BugScenario_MixedBalances verifies that 3 accounts with mixed
// balances ($0.50, $0.50, $2.00) are all submitted in one PayBulkAccounts call
// when no CG field is present, and the response is accepted.
func TestBulkPayment_BugScenario_MixedBalances(t *testing.T) {
	tc := testutil.NewTenantConfig()
	tc.AcceptBulkPayment = true
	sess := testutil.NewAuthSession(tc, testutil.WithSessionUser("testuser", "", ""))
	user := makeTestUser()

	accounts := &models.AccountCollection{
		Accounts: []models.Account{
			{ID: "acc-x1", FeeFineID: "ff-x1", Remaining: models.FlexibleFloat(0.50)},
			{ID: "acc-x2", FeeFineID: "ff-x2", Remaining: models.FlexibleFloat(0.50)},
			{ID: "acc-x3", FeeFineID: "ff-x3", Remaining: models.FlexibleFloat(2.00)},
		},
	}

	mockPatron := &MockPatronClient{}
	mockFees := &MockFeesClient{}

	mockPatron.On("GetUserByBarcode", mock.Anything, mock.Anything, user.Barcode).Return(user, nil)
	mockFees.On("GetOpenAccountsExcludingSuspended", mock.Anything, mock.Anything, user.ID).
		Return(accounts, nil)
	mockFees.On("PayBulkAccounts", mock.Anything, mock.Anything, mock.MatchedBy(func(p *models.Payment) bool {
		return p.Amount == "3.00" &&
			len(p.AccountIds) == 3 &&
			p.AccountIds[0] == "acc-x1" &&
			p.AccountIds[1] == "acc-x2" &&
			p.AccountIds[2] == "acc-x3"
	})).Return(nil)

	h := NewFeePaidHandler(zap.NewNop(), tc)
	injectMocks(h.BaseHandler, mockPatron, nil, nil, mockFees)

	// No CG field — goes directly to bulk path. Client sends $3.00.
	msg := feePaidMsg(user.Barcode, "3.00")

	resp, err := h.Handle(context.Background(), msg, sess)

	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(resp, "38"))
	assert.Equal(t, byte('Y'), resp[2], "payment must be accepted")
	assert.Contains(t, resp, "Bulk payment applied")

	mockPatron.AssertExpectations(t)
	mockFees.AssertExpectations(t)
}

// TestBulkPayment_OverPayment_Capped verifies that when the client sends more than
// the total outstanding balance, the amount is capped to totalOutstanding before
// calling PayBulkAccounts.
func TestBulkPayment_OverPayment_Capped(t *testing.T) {
	tc := testutil.NewTenantConfig()
	tc.AcceptBulkPayment = true
	sess := testutil.NewAuthSession(tc, testutil.WithSessionUser("testuser", "", ""))
	user := makeTestUser()

	// Total outstanding = $3.00; client sends $5.00.
	accounts := &models.AccountCollection{
		Accounts: []models.Account{
			{ID: "acc-1", FeeFineID: "ff-1", Remaining: models.FlexibleFloat(1.00)},
			{ID: "acc-2", FeeFineID: "ff-2", Remaining: models.FlexibleFloat(1.00)},
			{ID: "acc-3", FeeFineID: "ff-3", Remaining: models.FlexibleFloat(1.00)},
		},
	}

	mockPatron := &MockPatronClient{}
	mockFees := &MockFeesClient{}

	mockPatron.On("GetUserByBarcode", mock.Anything, mock.Anything, user.Barcode).Return(user, nil)
	mockFees.On("GetOpenAccountsExcludingSuspended", mock.Anything, mock.Anything, user.ID).
		Return(accounts, nil)
	// Amount must be capped to "3.00", not "5.00".
	mockFees.On("PayBulkAccounts", mock.Anything, mock.Anything, mock.MatchedBy(func(p *models.Payment) bool {
		return p.Amount == "3.00"
	})).Return(nil)

	h := NewFeePaidHandler(zap.NewNop(), tc)
	injectMocks(h.BaseHandler, mockPatron, nil, nil, mockFees)

	msg := feePaidMsg(user.Barcode, "5.00")

	resp, err := h.Handle(context.Background(), msg, sess)

	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(resp, "38"))
	assert.Equal(t, byte('Y'), resp[2], "capped bulk payment must be accepted")

	mockPatron.AssertExpectations(t)
	mockFees.AssertExpectations(t)
}

// TestBulkPayment_NoEligibleAccounts verifies that when no open non-suspended accounts
// exist, PayBulkAccounts is NOT called and the response indicates payment declined.
func TestBulkPayment_NoEligibleAccounts(t *testing.T) {
	tc := testutil.NewTenantConfig()
	tc.AcceptBulkPayment = true
	sess := testutil.NewAuthSession(tc, testutil.WithSessionUser("testuser", "", ""))
	user := makeTestUser()

	emptyAccounts := &models.AccountCollection{Accounts: []models.Account{}}

	mockPatron := &MockPatronClient{}
	mockFees := &MockFeesClient{}

	mockPatron.On("GetUserByBarcode", mock.Anything, mock.Anything, user.Barcode).Return(user, nil)
	mockFees.On("GetOpenAccountsExcludingSuspended", mock.Anything, mock.Anything, user.ID).
		Return(emptyAccounts, nil)
	// PayBulkAccounts must NOT be called.

	h := NewFeePaidHandler(zap.NewNop(), tc)
	injectMocks(h.BaseHandler, mockPatron, nil, nil, mockFees)

	msg := feePaidMsg(user.Barcode, "5.00")

	resp, err := h.Handle(context.Background(), msg, sess)

	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(resp, "38"))
	assert.Equal(t, byte('N'), resp[2], "response must be N when no eligible accounts exist")

	mockPatron.AssertExpectations(t)
	mockFees.AssertExpectations(t) // verifies PayBulkAccounts was never called
}

// TestBulkPayment_SingleEligibleAccount verifies that a single open account is
// submitted correctly via PayBulkAccounts with exactly one account ID.
func TestBulkPayment_SingleEligibleAccount(t *testing.T) {
	tc := testutil.NewTenantConfig()
	tc.AcceptBulkPayment = true
	sess := testutil.NewAuthSession(tc, testutil.WithSessionUser("testuser", "", ""))
	user := makeTestUser()

	accounts := &models.AccountCollection{
		Accounts: []models.Account{
			{ID: "solo-acc", FeeFineID: "solo-ff", Remaining: models.FlexibleFloat(7.50)},
		},
	}

	mockPatron := &MockPatronClient{}
	mockFees := &MockFeesClient{}

	mockPatron.On("GetUserByBarcode", mock.Anything, mock.Anything, user.Barcode).Return(user, nil)
	mockFees.On("GetOpenAccountsExcludingSuspended", mock.Anything, mock.Anything, user.ID).
		Return(accounts, nil)
	mockFees.On("PayBulkAccounts", mock.Anything, mock.Anything, mock.MatchedBy(func(p *models.Payment) bool {
		return len(p.AccountIds) == 1 && p.AccountIds[0] == "solo-acc"
	})).Return(nil)

	h := NewFeePaidHandler(zap.NewNop(), tc)
	injectMocks(h.BaseHandler, mockPatron, nil, nil, mockFees)

	// No CG field → bulk path.
	msg := feePaidMsg(user.Barcode, "7.50")

	resp, err := h.Handle(context.Background(), msg, sess)

	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(resp, "38"))
	assert.Equal(t, byte('Y'), resp[2], "single-account bulk payment must be accepted")

	mockPatron.AssertExpectations(t)
	mockFees.AssertExpectations(t)
}

// TestBulkPayment_FallbackFromCG verifies that when a CG account lookup returns nil
// (not eligible) and acceptBulkPayment is true, the handler falls through to PayBulkAccounts.
func TestBulkPayment_FallbackFromCG(t *testing.T) {
	tc := testutil.NewTenantConfig()
	tc.AcceptBulkPayment = true
	sess := testutil.NewAuthSession(tc, testutil.WithSessionUser("testuser", "", ""))
	user := makeTestUser()

	accounts := &models.AccountCollection{
		Accounts: []models.Account{
			{ID: "acc-fallback", FeeFineID: "ff-fallback", Remaining: models.FlexibleFloat(4.00)},
		},
	}

	mockPatron := &MockPatronClient{}
	mockFees := &MockFeesClient{}

	mockPatron.On("GetUserByBarcode", mock.Anything, mock.Anything, user.Barcode).Return(user, nil)
	// CG lookup returns nil — account not eligible, triggers fallback.
	mockFees.On("GetEligibleAccountByID", mock.Anything, mock.Anything, "cg-bad").
		Return(nil, nil)
	mockFees.On("GetOpenAccountsExcludingSuspended", mock.Anything, mock.Anything, user.ID).
		Return(accounts, nil)
	mockFees.On("PayBulkAccounts", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	h := NewFeePaidHandler(zap.NewNop(), tc)
	injectMocks(h.BaseHandler, mockPatron, nil, nil, mockFees)

	// CG field present but account not eligible → should fall back to bulk.
	msg := feePaidMsg(user.Barcode, "4.00", map[parser.FieldCode]string{
		parser.FeeIdentifier: "cg-bad",
	})

	resp, err := h.Handle(context.Background(), msg, sess)

	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(resp, "38"))
	assert.Equal(t, byte('Y'), resp[2], "fallback bulk payment must be accepted")

	mockPatron.AssertExpectations(t)
	mockFees.AssertExpectations(t)
}

// TestBulkPayment_FallbackDisabled verifies that when a CG account lookup fails
// and acceptBulkPayment is false, the response is declined and PayBulkAccounts is NOT called.
func TestBulkPayment_FallbackDisabled(t *testing.T) {
	tc := testutil.NewTenantConfig()
	tc.AcceptBulkPayment = false
	sess := testutil.NewAuthSession(tc, testutil.WithSessionUser("testuser", "", ""))
	user := makeTestUser()

	mockPatron := &MockPatronClient{}
	mockFees := &MockFeesClient{}

	mockPatron.On("GetUserByBarcode", mock.Anything, mock.Anything, user.Barcode).Return(user, nil)
	// CG lookup returns nil — not eligible.
	mockFees.On("GetEligibleAccountByID", mock.Anything, mock.Anything, "cg-bad").
		Return(nil, nil)
	// PayBulkAccounts must NOT be called.

	h := NewFeePaidHandler(zap.NewNop(), tc)
	injectMocks(h.BaseHandler, mockPatron, nil, nil, mockFees)

	msg := feePaidMsg(user.Barcode, "4.00", map[parser.FieldCode]string{
		parser.FeeIdentifier: "cg-bad",
	})

	resp, err := h.Handle(context.Background(), msg, sess)

	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(resp, "38"))
	assert.Equal(t, byte('N'), resp[2], "response must be N when bulk fallback is disabled")

	mockPatron.AssertExpectations(t)
	mockFees.AssertExpectations(t) // verifies PayBulkAccounts was never called
}
