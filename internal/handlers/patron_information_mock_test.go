package handlers

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/spokanepubliclibrary/fsip2/internal/folio/models"
	"github.com/spokanepubliclibrary/fsip2/internal/sip2/parser"
	"github.com/spokanepubliclibrary/fsip2/tests/testutil"
)

// patronInfoMsg builds a PatronInformation request message with default fields.
func patronInfoMsg(barcode string) *parser.Message {
	return buildTestMsg(parser.PatronInformationRequest, map[parser.FieldCode]string{
		parser.InstitutionID:    "TEST-INST",
		parser.PatronIdentifier: barcode,
	})
}

// setupPatronInfoMocks sets up the standard minimal mocks for PatronInformation Handle():
// user lookup, holds, loans, accounts, unavailable holds, and blocks.
// Only mocks that are needed to reach the response-building stage are registered.
func setupPatronInfoMocks(mockPatron *MockPatronClient, mockCirc *MockCirculationClient, mockFees *MockFeesClient, user *models.User) {
	mockPatron.On("GetUserByBarcode", mock.Anything, mock.Anything, user.Barcode).Return(user, nil)
	mockPatron.On("GetManualBlocks", mock.Anything, mock.Anything, user.ID).
		Return(&models.ManualBlockCollection{}, nil)
	mockPatron.On("GetAutomatedPatronBlocks", mock.Anything, mock.Anything, user.ID).
		Return(&models.AutomatedPatronBlock{}, nil)
	mockCirc.On("GetAvailableHolds", mock.Anything, mock.Anything, user.ID).
		Return(&models.RequestCollection{}, nil)
	mockCirc.On("GetOpenLoansByUser", mock.Anything, mock.Anything, user.ID).
		Return(&models.LoanCollection{}, nil)
	mockCirc.On("GetUnavailableHolds", mock.Anything, mock.Anything, user.ID).
		Return(&models.RequestCollection{}, nil)
	mockFees.On("GetOpenAccountsExcludingSuspended", mock.Anything, mock.Anything, user.ID).
		Return(&models.AccountCollection{}, nil)
}

// TestPatronInformationHandle_ValidPatron_Minimal verifies a minimal success response:
// starts with 64, contains |BLY and the institution ID.
func TestPatronInformationHandle_ValidPatron_Minimal(t *testing.T) {
	tc := testutil.NewTenantConfig()
	sess := testutil.NewAuthSession(tc)
	user := makeTestUser()

	mockPatron := &MockPatronClient{}
	mockCirc := &MockCirculationClient{}
	mockFees := &MockFeesClient{}
	setupPatronInfoMocks(mockPatron, mockCirc, mockFees, user)

	h := NewPatronInformationHandler(zap.NewNop(), tc)
	injectMocks(h.BaseHandler, mockPatron, mockCirc, nil, mockFees)

	resp, err := h.Handle(context.Background(), patronInfoMsg(user.Barcode), sess)

	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(resp, "64"), "response must start with 64")
	assert.Contains(t, resp, "|BLY", "valid patron flag required")
	assert.Contains(t, resp, "|AOTEST-INST")
	assert.NotContains(t, resp, "|BV", "no fee field for zero balance")

	mockPatron.AssertExpectations(t)
	mockCirc.AssertExpectations(t)
	mockFees.AssertExpectations(t)
}

// TestPatronInformationHandle_ValidPatron_WithLoans verifies that 2 open loans
// result in a charged-items count of 0002 in the fixed-length section.
func TestPatronInformationHandle_ValidPatron_WithLoans(t *testing.T) {
	tc := testutil.NewTenantConfig()
	sess := testutil.NewAuthSession(tc)
	user := makeTestUser()

	mockPatron := &MockPatronClient{}
	mockCirc := &MockCirculationClient{}
	mockInv := &MockInventoryClient{}
	mockFees := &MockFeesClient{}

	futureDue := time.Now().Add(7 * 24 * time.Hour)
	loans := &models.LoanCollection{
		Loans: []models.Loan{
			{ID: "loan-1", ItemID: "item-aaa", DueDate: &futureDue},
			{ID: "loan-2", ItemID: "item-bbb", DueDate: &futureDue},
		},
	}
	item := &models.Item{ID: "item-x", Barcode: "ITEM-X"}

	mockPatron.On("GetUserByBarcode", mock.Anything, mock.Anything, user.Barcode).Return(user, nil)
	mockPatron.On("GetManualBlocks", mock.Anything, mock.Anything, user.ID).
		Return(&models.ManualBlockCollection{}, nil)
	mockPatron.On("GetAutomatedPatronBlocks", mock.Anything, mock.Anything, user.ID).
		Return(&models.AutomatedPatronBlock{}, nil)
	mockCirc.On("GetAvailableHolds", mock.Anything, mock.Anything, user.ID).
		Return(&models.RequestCollection{}, nil)
	mockCirc.On("GetOpenLoansByUser", mock.Anything, mock.Anything, user.ID).
		Return(loans, nil)
	mockCirc.On("GetUnavailableHolds", mock.Anything, mock.Anything, user.ID).
		Return(&models.RequestCollection{}, nil)
	mockInv.On("GetItemByID", mock.Anything, mock.Anything, "item-aaa").Return(item, nil)
	mockInv.On("GetItemByID", mock.Anything, mock.Anything, "item-bbb").Return(item, nil)
	mockFees.On("GetOpenAccountsExcludingSuspended", mock.Anything, mock.Anything, user.ID).
		Return(&models.AccountCollection{}, nil)

	h := NewPatronInformationHandler(zap.NewNop(), tc)
	injectMocks(h.BaseHandler, mockPatron, mockCirc, mockInv, mockFees)

	resp, err := h.Handle(context.Background(), patronInfoMsg(user.Barcode), sess)

	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(resp, "64"))
	assert.Contains(t, resp, "|BLY")
	// Fixed field: charged_items_count (4 digits) = "0002"
	assert.Contains(t, resp, "0002", "charged items count must be 2")

	mockPatron.AssertExpectations(t)
	mockCirc.AssertExpectations(t)
	mockInv.AssertExpectations(t)
	mockFees.AssertExpectations(t)
}

// TestPatronInformationHandle_ValidPatron_WithHolds verifies that 1 available hold
// results in a hold-items count of 0001 in the fixed-length section.
func TestPatronInformationHandle_ValidPatron_WithHolds(t *testing.T) {
	tc := testutil.NewTenantConfig()
	sess := testutil.NewAuthSession(tc)
	user := makeTestUser()

	mockPatron := &MockPatronClient{}
	mockCirc := &MockCirculationClient{}
	mockFees := &MockFeesClient{}

	holds := &models.RequestCollection{
		Requests: []models.Request{
			{ID: "req-1", RequestType: "Hold", Status: "Open - Awaiting pickup"},
		},
	}

	mockPatron.On("GetUserByBarcode", mock.Anything, mock.Anything, user.Barcode).Return(user, nil)
	mockPatron.On("GetManualBlocks", mock.Anything, mock.Anything, user.ID).
		Return(&models.ManualBlockCollection{}, nil)
	mockPatron.On("GetAutomatedPatronBlocks", mock.Anything, mock.Anything, user.ID).
		Return(&models.AutomatedPatronBlock{}, nil)
	mockCirc.On("GetAvailableHolds", mock.Anything, mock.Anything, user.ID).
		Return(holds, nil)
	mockCirc.On("GetOpenLoansByUser", mock.Anything, mock.Anything, user.ID).
		Return(&models.LoanCollection{}, nil)
	mockCirc.On("GetUnavailableHolds", mock.Anything, mock.Anything, user.ID).
		Return(&models.RequestCollection{}, nil)
	mockFees.On("GetOpenAccountsExcludingSuspended", mock.Anything, mock.Anything, user.ID).
		Return(&models.AccountCollection{}, nil)

	h := NewPatronInformationHandler(zap.NewNop(), tc)
	injectMocks(h.BaseHandler, mockPatron, mockCirc, nil, mockFees)

	resp, err := h.Handle(context.Background(), patronInfoMsg(user.Barcode), sess)

	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(resp, "64"))
	assert.Contains(t, resp, "|BLY")
	// Fixed field: hold_items_count = "0001"
	assert.Contains(t, resp, "0001", "hold items count must be 1")

	mockPatron.AssertExpectations(t)
	mockCirc.AssertExpectations(t)
	mockFees.AssertExpectations(t)
}

// TestPatronInformationHandle_ValidPatron_WithFees verifies that outstanding fees
// produce a |BV field with the total balance.
func TestPatronInformationHandle_ValidPatron_WithFees(t *testing.T) {
	tc := testutil.NewTenantConfig()
	sess := testutil.NewAuthSession(tc)
	user := makeTestUser()

	mockPatron := &MockPatronClient{}
	mockCirc := &MockCirculationClient{}
	mockFees := &MockFeesClient{}

	accounts := &models.AccountCollection{
		Accounts: []models.Account{
			{ID: "fee-1", Remaining: models.FlexibleFloat(5.00)},
		},
	}

	mockPatron.On("GetUserByBarcode", mock.Anything, mock.Anything, user.Barcode).Return(user, nil)
	mockPatron.On("GetManualBlocks", mock.Anything, mock.Anything, user.ID).
		Return(&models.ManualBlockCollection{}, nil)
	mockPatron.On("GetAutomatedPatronBlocks", mock.Anything, mock.Anything, user.ID).
		Return(&models.AutomatedPatronBlock{}, nil)
	mockCirc.On("GetAvailableHolds", mock.Anything, mock.Anything, user.ID).
		Return(&models.RequestCollection{}, nil)
	mockCirc.On("GetOpenLoansByUser", mock.Anything, mock.Anything, user.ID).
		Return(&models.LoanCollection{}, nil)
	mockCirc.On("GetUnavailableHolds", mock.Anything, mock.Anything, user.ID).
		Return(&models.RequestCollection{}, nil)
	mockFees.On("GetOpenAccountsExcludingSuspended", mock.Anything, mock.Anything, user.ID).
		Return(accounts, nil)

	h := NewPatronInformationHandler(zap.NewNop(), tc)
	injectMocks(h.BaseHandler, mockPatron, mockCirc, nil, mockFees)

	resp, err := h.Handle(context.Background(), patronInfoMsg(user.Barcode), sess)

	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(resp, "64"))
	assert.Contains(t, resp, "|BLY")
	assert.Contains(t, resp, "|BV5.00", "fee balance in BV field")

	mockPatron.AssertExpectations(t)
	mockCirc.AssertExpectations(t)
	mockFees.AssertExpectations(t)
}

// TestPatronInformationHandle_UserNotFound verifies that a failed patron lookup
// returns |BLN.
func TestPatronInformationHandle_UserNotFound(t *testing.T) {
	tc := testutil.NewTenantConfig()
	sess := testutil.NewAuthSession(tc)

	mockPatron := &MockPatronClient{}

	mockPatron.On("GetUserByBarcode", mock.Anything, mock.Anything, "P999").
		Return(nil, assert.AnError)

	h := NewPatronInformationHandler(zap.NewNop(), tc)
	injectMocks(h.BaseHandler, mockPatron, nil, nil, nil)

	resp, err := h.Handle(context.Background(), patronInfoMsg("P999"), sess)

	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(resp, "64"))
	assert.Contains(t, resp, "|BLN", "invalid patron flag required for unfound user")
	assert.NotContains(t, resp, "|BLY")

	mockPatron.AssertExpectations(t)
}
