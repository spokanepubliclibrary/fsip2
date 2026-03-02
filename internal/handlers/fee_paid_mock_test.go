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
	payResp := &models.PaymentResponse{RemainingAmount: "0.00"}

	mockPatron := &MockPatronClient{}
	mockFees := &MockFeesClient{}

	mockPatron.On("GetUserByBarcode", mock.Anything, mock.Anything, user.Barcode).Return(user, nil)
	mockFees.On("GetOpenAccountsExcludingSuspended", mock.Anything, mock.Anything, user.ID).
		Return(accounts, nil)
	mockFees.On("PayAccount", mock.Anything, mock.Anything, "acc-1", mock.Anything).Return(payResp, nil)
	mockFees.On("PayAccount", mock.Anything, mock.Anything, "acc-2", mock.Anything).Return(payResp, nil)
	mockFees.On("PayAccount", mock.Anything, mock.Anything, "acc-3", mock.Anything).Return(payResp, nil)

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

// TestFeePaidHandle_BulkPayment_PartialFailure verifies that when one of three bulk
// accounts fails to pay, the response still indicates success (at least one paid)
// but mentions staff details.
func TestFeePaidHandle_BulkPayment_PartialFailure(t *testing.T) {
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
	payResp := &models.PaymentResponse{RemainingAmount: "0.00"}

	mockPatron := &MockPatronClient{}
	mockFees := &MockFeesClient{}

	mockPatron.On("GetUserByBarcode", mock.Anything, mock.Anything, user.Barcode).Return(user, nil)
	mockFees.On("GetOpenAccountsExcludingSuspended", mock.Anything, mock.Anything, user.ID).
		Return(accounts, nil)
	mockFees.On("PayAccount", mock.Anything, mock.Anything, "acc-a", mock.Anything).Return(payResp, nil)
	mockFees.On("PayAccount", mock.Anything, mock.Anything, "acc-b", mock.Anything).
		Return(nil, assert.AnError) // one failure
	mockFees.On("PayAccount", mock.Anything, mock.Anything, "acc-c", mock.Anything).Return(payResp, nil)

	h := NewFeePaidHandler(zap.NewNop(), tc)
	injectMocks(h.BaseHandler, mockPatron, nil, nil, mockFees)

	msg := feePaidMsg(user.Barcode, "15.00")

	resp, err := h.Handle(context.Background(), msg, sess)

	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(resp, "38"))
	assert.Equal(t, byte('Y'), resp[2], "response should be Y when at least one payment succeeded")
	assert.Contains(t, resp, "see staff for details", "partial failure message expected")

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
