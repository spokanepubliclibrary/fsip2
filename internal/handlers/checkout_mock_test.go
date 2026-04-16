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
	sess := testutil.NewAuthSession(tc, testutil.WithLocationCode("test-service-point-uuid"))
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
		parser.PatronIdentifier: "123456", // matches default session barcode; patron ID comes from session
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
	sess := testutil.NewAuthSession(tc, testutil.WithSessionUser("testuser", "", ""), testutil.WithLocationCode("test-service-point-uuid"))

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
	sess := testutil.NewAuthSession(tc, testutil.WithSessionUser("testuser", "", ""), testutil.WithLocationCode("test-service-point-uuid"))
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
	sess := testutil.NewAuthSession(tc, testutil.WithLocationCode("test-service-point-uuid")) // patronID set → skips GetUserByBarcode
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
		parser.PatronIdentifier: "123456", // matches default session barcode; skips GetUserByBarcode
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
	sess := testutil.NewAuthSession(tc, testutil.WithLocationCode("test-service-point-uuid")) // patronID set
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
		parser.PatronIdentifier: "123456", // matches default session barcode; skips GetUserByBarcode
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

// TestCheckout_UsesCachedPatronID_WhenBarcodeMatches verifies that when the
// session already holds a patron ID and the cached barcode matches the incoming
// request barcode, GetUserByBarcode is NOT called — the cached ID is reused.
func TestCheckout_UsesCachedPatronID_WhenBarcodeMatches(t *testing.T) {
	tc := testutil.NewTenantConfig()
	// Session has PatronID="uuid-1", PatronBarcode="BARCODE-A" (via WithSessionUser).
	sess := testutil.NewAuthSession(tc,
		testutil.WithSessionUser("testuser", "uuid-1", "BARCODE-A"),
		testutil.WithLocationCode("test-service-point-uuid"),
	)
	loan := openLoanWithDueDate()
	item := availableItemNoHoldings("item-ca-001", "ITEM-CA-001")

	mockPatron := &MockPatronClient{}
	mockCirc   := &MockCirculationClient{}
	mockInv    := &MockInventoryClient{}

	// GetUserByBarcode must NOT be called because barcode matches the cached value.
	mockCirc.On("Checkout", mock.Anything, mock.Anything, mock.Anything).Return(loan, nil)
	mockInv.On("GetItemByBarcode", mock.Anything, mock.Anything, "ITEM-CA-001").Return(item, nil)

	h := NewCheckoutHandler(zap.NewNop(), tc)
	injectMocks(h.BaseHandler, mockPatron, mockCirc, mockInv, nil)

	msg := buildTestMsg(parser.CheckoutRequest, map[parser.FieldCode]string{
		parser.InstitutionID:    "TEST-INST",
		parser.PatronIdentifier: "BARCODE-A", // matches cached barcode
		parser.ItemIdentifier:   "ITEM-CA-001",
	})

	resp, err := h.Handle(context.Background(), msg, sess)

	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(resp, "12"))
	assert.Equal(t, byte('1'), resp[2], "ok byte must be '1' — cached patron ID used")

	// Confirm patron lookup was never invoked.
	mockPatron.AssertNotCalled(t, "GetUserByBarcode", mock.Anything, mock.Anything, mock.Anything)
	mockCirc.AssertExpectations(t)
	mockInv.AssertExpectations(t)
}

// TestCheckout_RefreshesPatronID_WhenBarcodeMismatches verifies that when the
// session holds a patron ID for BARCODE-A but the request carries BARCODE-B,
// the stale cache is invalidated, GetUserByBarcode IS called with BARCODE-B,
// and the session is updated to the new patron ID and barcode.
func TestCheckout_RefreshesPatronID_WhenBarcodeMismatches(t *testing.T) {
	tc := testutil.NewTenantConfig()
	// Session has stale PatronID="uuid-1" cached for BARCODE-A.
	sess := testutil.NewAuthSession(tc,
		testutil.WithSessionUser("testuser", "uuid-1", "BARCODE-A"),
		testutil.WithLocationCode("test-service-point-uuid"),
	)

	// The fresh lookup for BARCODE-B returns a different user.
	freshUser := &models.User{
		ID:      "uuid-2",
		Barcode: "BARCODE-B",
		Active:  true,
		Personal: models.PersonalInfo{FirstName: "Other", LastName: "Patron"},
	}
	loan := openLoanWithDueDate()
	item := availableItemNoHoldings("item-mm-001", "ITEM-MM-001")

	mockPatron := &MockPatronClient{}
	mockCirc   := &MockCirculationClient{}
	mockInv    := &MockInventoryClient{}

	// GetUserByBarcode MUST be called with BARCODE-B.
	mockPatron.On("GetUserByBarcode", mock.Anything, mock.Anything, "BARCODE-B").Return(freshUser, nil)
	mockCirc.On("Checkout", mock.Anything, mock.Anything, mock.Anything).Return(loan, nil)
	mockInv.On("GetItemByBarcode", mock.Anything, mock.Anything, "ITEM-MM-001").Return(item, nil)

	h := NewCheckoutHandler(zap.NewNop(), tc)
	injectMocks(h.BaseHandler, mockPatron, mockCirc, mockInv, nil)

	msg := buildTestMsg(parser.CheckoutRequest, map[parser.FieldCode]string{
		parser.InstitutionID:    "TEST-INST",
		parser.PatronIdentifier: "BARCODE-B", // mismatches cached BARCODE-A
		parser.ItemIdentifier:   "ITEM-MM-001",
	})

	resp, err := h.Handle(context.Background(), msg, sess)

	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(resp, "12"))
	assert.Equal(t, byte('1'), resp[2], "ok byte must be '1' after refreshed lookup")

	// Session should now hold the new patron ID and barcode.
	assert.Equal(t, "uuid-2", sess.GetPatronID(), "session PatronID should be updated to uuid-2")
	assert.Equal(t, "BARCODE-B", sess.GetPatronBarcode(), "session PatronBarcode should be updated to BARCODE-B")

	mockPatron.AssertExpectations(t)
	mockCirc.AssertExpectations(t)
	mockInv.AssertExpectations(t)
}

// TestCheckout_LooksUpPatron_WhenCacheEmpty verifies that when the session has
// no cached patron ID, GetUserByBarcode is called, the checkout proceeds with
// the returned ID, and the session is populated with the new patron ID/barcode.
func TestCheckout_LooksUpPatron_WhenCacheEmpty(t *testing.T) {
	tc := testutil.NewTenantConfig()
	// Empty userID and barcode → no cached patron ID.
	sess := testutil.NewAuthSession(tc,
		testutil.WithSessionUser("testuser", "", ""),
		testutil.WithLocationCode("test-service-point-uuid"),
	)

	newUser := &models.User{
		ID:      "uuid-fresh",
		Barcode: "BARCODE-A",
		Active:  true,
		Personal: models.PersonalInfo{FirstName: "Fresh", LastName: "User"},
	}
	loan := openLoanWithDueDate()
	item := availableItemNoHoldings("item-ce-001", "ITEM-CE-001")

	mockPatron := &MockPatronClient{}
	mockCirc   := &MockCirculationClient{}
	mockInv    := &MockInventoryClient{}

	// GetUserByBarcode MUST be called because cache is empty.
	mockPatron.On("GetUserByBarcode", mock.Anything, mock.Anything, "BARCODE-A").Return(newUser, nil)
	mockCirc.On("Checkout", mock.Anything, mock.Anything, mock.Anything).Return(loan, nil)
	mockInv.On("GetItemByBarcode", mock.Anything, mock.Anything, "ITEM-CE-001").Return(item, nil)

	h := NewCheckoutHandler(zap.NewNop(), tc)
	injectMocks(h.BaseHandler, mockPatron, mockCirc, mockInv, nil)

	msg := buildTestMsg(parser.CheckoutRequest, map[parser.FieldCode]string{
		parser.InstitutionID:    "TEST-INST",
		parser.PatronIdentifier: "BARCODE-A",
		parser.ItemIdentifier:   "ITEM-CE-001",
	})

	resp, err := h.Handle(context.Background(), msg, sess)

	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(resp, "12"))
	assert.Equal(t, byte('1'), resp[2], "ok byte must be '1' after fresh lookup")

	// Session should be populated with the looked-up patron.
	assert.Equal(t, "uuid-fresh", sess.GetPatronID(), "session PatronID should be set after lookup")
	assert.Equal(t, "BARCODE-A", sess.GetPatronBarcode(), "session PatronBarcode should be set after lookup")

	mockPatron.AssertExpectations(t)
	mockCirc.AssertExpectations(t)
	mockInv.AssertExpectations(t)
}
