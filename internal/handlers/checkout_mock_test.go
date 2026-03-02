package handlers

import (
	"context"
	"fmt"
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

// openLoanWithDueDate returns a minimal Loan with an open status and a due date.
func openLoanWithDueDate() *models.Loan {
	due := time.Now().Add(14 * 24 * time.Hour)
	return &models.Loan{
		ID:      "loan-co-001",
		DueDate: &due,
	}
}

// TestCheckoutHandle_Success verifies a full success path: session already carries
// a patron ID so the barcode lookup is skipped, checkout succeeds, item details
// are fetched. Response starts with "12" and ok byte is '1'.
func TestCheckoutHandle_Success(t *testing.T) {
	tc := testutil.NewTenantConfig()
	// NewAuthSession populates patronID = "user-123"; handler skips GetUserByBarcode.
	sess := testutil.NewAuthSession(tc)
	loan := openLoanWithDueDate()
	item := availableItemNoHoldings("item-co-001", "ITEM-CO-001")

	mockCirc := &MockCirculationClient{}
	mockInv := &MockInventoryClient{}

	mockCirc.On("Checkout", mock.Anything, mock.Anything, mock.Anything).Return(loan, nil)
	// Checkout handler fetches item by barcode for title lookup after success.
	mockInv.On("GetItemByBarcode", mock.Anything, mock.Anything, "ITEM-CO-001").Return(item, nil)

	h := NewCheckoutHandler(zap.NewNop(), tc)
	injectMocks(h.BaseHandler, nil, mockCirc, mockInv, nil)

	msg := buildTestMsg(parser.CheckoutRequest, map[parser.FieldCode]string{
		parser.InstitutionID:    "TEST-INST",
		parser.PatronIdentifier: "PATRON-001", // barcode; patron ID comes from session
		parser.ItemIdentifier:   "ITEM-CO-001",
	})

	resp, err := h.Handle(context.Background(), msg, sess)

	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(resp, "12"), "response must start with 12")
	// SIP2 Checkout Response (12): ok is '1' (success) or '0' (failure).
	assert.Equal(t, byte('1'), resp[2], "ok byte must be '1' for successful checkout")

	mockCirc.AssertExpectations(t)
	mockInv.AssertExpectations(t)
}

// TestCheckoutHandle_UserNotFound verifies that a failed patron barcode lookup
// produces an ok=0 response. The session has no pre-set patron ID so the
// handler performs the lookup.
func TestCheckoutHandle_UserNotFound(t *testing.T) {
	tc := testutil.NewTenantConfig()
	// Empty patron ID → handler calls GetUserByBarcode.
	sess := testutil.NewAuthSession(tc, testutil.WithSessionUser("testuser", "", ""))

	mockPatron := &MockPatronClient{}

	mockPatron.On("GetUserByBarcode", mock.Anything, mock.Anything, "P-BAD").
		Return(nil, assert.AnError)

	h := NewCheckoutHandler(zap.NewNop(), tc)
	injectMocks(h.BaseHandler, mockPatron, nil, nil, nil)

	msg := buildTestMsg(parser.CheckoutRequest, map[parser.FieldCode]string{
		parser.InstitutionID:    "TEST-INST",
		parser.PatronIdentifier: "P-BAD",
		parser.ItemIdentifier:   "ITEM-001",
	})

	resp, err := h.Handle(context.Background(), msg, sess)

	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(resp, "12"))
	assert.Equal(t, byte('0'), resp[2], "ok byte must be '0' when patron not found")

	mockPatron.AssertExpectations(t)
}

// TestCheckoutHandle_CheckoutPolicyViolation verifies that a Checkout API error
// (e.g. policy violation) produces an ok=0 response.
func TestCheckoutHandle_CheckoutPolicyViolation(t *testing.T) {
	tc := testutil.NewTenantConfig()
	// Empty patron ID → handler calls GetUserByBarcode first.
	sess := testutil.NewAuthSession(tc, testutil.WithSessionUser("testuser", "", ""))
	user := makeTestUser()

	mockPatron := &MockPatronClient{}
	mockCirc := &MockCirculationClient{}

	mockPatron.On("GetUserByBarcode", mock.Anything, mock.Anything, user.Barcode).Return(user, nil)
	mockCirc.On("Checkout", mock.Anything, mock.Anything, mock.Anything).
		Return(nil, assert.AnError)

	h := NewCheckoutHandler(zap.NewNop(), tc)
	injectMocks(h.BaseHandler, mockPatron, mockCirc, nil, nil)

	msg := buildTestMsg(parser.CheckoutRequest, map[parser.FieldCode]string{
		parser.InstitutionID:    "TEST-INST",
		parser.PatronIdentifier: user.Barcode,
		parser.ItemIdentifier:   "ITEM-001",
	})

	resp, err := h.Handle(context.Background(), msg, sess)

	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(resp, "12"))
	assert.Equal(t, byte('0'), resp[2], "ok byte must be '0' when checkout API fails")

	mockPatron.AssertExpectations(t)
	mockCirc.AssertExpectations(t)
}

// TestCheckoutHandle_ItemFetchFailsAfterCheckout verifies that when item details
// cannot be fetched after a successful checkout, the handler still returns ok=1
// with the item barcode used as the fallback title.
func TestCheckoutHandle_ItemFetchFailsAfterCheckout(t *testing.T) {
	tc   := testutil.NewTenantConfig()
	sess := testutil.NewAuthSession(tc) // patronID set → skips GetUserByBarcode
	loan := openLoanWithDueDate()

	mockCirc := &MockCirculationClient{}
	mockInv  := &MockInventoryClient{}

	mockCirc.On("Checkout", mock.Anything, mock.Anything, mock.Anything).Return(loan, nil)
	mockInv.On("GetItemByBarcode", mock.Anything, mock.Anything, "ITEM-001").
		Return(nil, fmt.Errorf("inventory unavailable"))

	h := NewCheckoutHandler(zap.NewNop(), tc)
	injectMocks(h.BaseHandler, nil, mockCirc, mockInv, nil)

	msg := buildTestMsg(parser.CheckoutRequest, map[parser.FieldCode]string{
		parser.InstitutionID:    "TEST-INST",
		parser.PatronIdentifier: "P-001",
		parser.ItemIdentifier:   "ITEM-001",
	})

	resp, err := h.Handle(context.Background(), msg, sess)

	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(resp, "12"))
	assert.Equal(t, byte('1'), resp[2], "checkout succeeded even though item fetch failed")

	mockCirc.AssertExpectations(t)
	mockInv.AssertExpectations(t)
}

// TestCheckoutHandle_TitleFromHoldings verifies that when an item has a holdings
// record, the handler walks holdings → instance to retrieve the title and includes
// it in the checkout response.
func TestCheckoutHandle_TitleFromHoldings(t *testing.T) {
	tc   := testutil.NewTenantConfig()
	sess := testutil.NewAuthSession(tc) // patronID set
	loan := openLoanWithDueDate()

	item := &models.Item{
		ID:               "item-h-001",
		Barcode:          "ITEM-H-001",
		HoldingsRecordID: "holdings-h-001",
		Status:           models.ItemStatus{Name: "Available"},
	}
	holdings := &models.Holdings{ID: "holdings-h-001", InstanceID: "instance-h-001"}
	instance := &models.Instance{ID: "instance-h-001", Title: "Test Book Title"}

	mockCirc := &MockCirculationClient{}
	mockInv  := &MockInventoryClient{}

	mockCirc.On("Checkout", mock.Anything, mock.Anything, mock.Anything).Return(loan, nil)
	mockInv.On("GetItemByBarcode", mock.Anything, mock.Anything, "ITEM-H-001").Return(item, nil)
	mockInv.On("GetHoldingsByID", mock.Anything, mock.Anything, "holdings-h-001").Return(holdings, nil)
	mockInv.On("GetInstanceByID", mock.Anything, mock.Anything, "instance-h-001").Return(instance, nil)

	h := NewCheckoutHandler(zap.NewNop(), tc)
	injectMocks(h.BaseHandler, nil, mockCirc, mockInv, nil)

	msg := buildTestMsg(parser.CheckoutRequest, map[parser.FieldCode]string{
		parser.InstitutionID:    "TEST-INST",
		parser.PatronIdentifier: "P-001",
		parser.ItemIdentifier:   "ITEM-H-001",
	})

	resp, err := h.Handle(context.Background(), msg, sess)

	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(resp, "12"))
	assert.Equal(t, byte('1'), resp[2], "checkout succeeded with title from instance")
	assert.Contains(t, resp, "Test Book Title")

	mockCirc.AssertExpectations(t)
	mockInv.AssertExpectations(t)
}
