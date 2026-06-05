package handlers

import (
	"context"
	"errors"
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

// openRenewLoanWithDueDate returns a minimal Loan with an open status, a due
// date, and an Item containing an InstanceID so the handler can resolve a title
// without a secondary item lookup.
func openRenewLoanWithDueDate(loanID, itemID, instanceID string) *models.Loan {
	due := time.Now().Add(14 * 24 * time.Hour)
	return &models.Loan{
		ID:      loanID,
		ItemID:  itemID,
		DueDate: &due,
		Item:    &models.Item{InstanceID: instanceID},
	}
}

// TestRenewHandle_TitleFromInstance verifies that when the renewal loan response
// includes an Item with an InstanceID, the handler fetches the instance title and
// includes it in the AJ field of the Renew Response (30).
func TestRenewHandle_TitleFromInstance(t *testing.T) {
	tc   := testutil.NewTenantConfig()
	sess := testutil.NewAuthSession(tc, testutil.WithLocationCode("test-service-point-uuid"))
	loan := openRenewLoanWithDueDate("loan-r-001", "item-r-001", "instance-r-001")

	instance := &models.Instance{ID: "instance-r-001", Title: "Renewed Book Title"}

	mockCirc := &MockCirculationClient{}
	mockInv  := &MockInventoryClient{}

	mockCirc.On("Renew", mock.Anything, mock.Anything, mock.Anything).Return(loan, nil)
	mockInv.On("GetInstanceByID", mock.Anything, mock.Anything, "instance-r-001").Return(instance, nil)

	h := NewRenewHandler(zap.NewNop(), tc)
	injectMocks(h.BaseHandler, nil, mockCirc, mockInv, nil)

	msg := buildTestMsg(parser.RenewRequest, map[parser.FieldCode]string{
		parser.InstitutionID:    "TEST-INST",
		parser.PatronIdentifier: "123456",
		parser.ItemIdentifier:   "ITEM-R-001",
	})

	resp, err := h.Handle(context.Background(), msg, sess)

	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(resp, "30"), "response must start with 30")
	assert.Contains(t, resp, "AJRenewed Book Title")

	mockCirc.AssertExpectations(t)
	mockInv.AssertExpectations(t)
}

// TestRenewHandle_TitleTruncatedTo60Chars verifies that an instance title longer
// than 60 characters is truncated to exactly 60 characters in the AJ field.
func TestRenewHandle_TitleTruncatedTo60Chars(t *testing.T) {
	tc   := testutil.NewTenantConfig()
	sess := testutil.NewAuthSession(tc, testutil.WithLocationCode("test-service-point-uuid"))
	loan := openRenewLoanWithDueDate("loan-r-002", "item-r-002", "instance-r-002")

	longTitle := strings.Repeat("A", 80)
	instance  := &models.Instance{ID: "instance-r-002", Title: longTitle}

	mockCirc := &MockCirculationClient{}
	mockInv  := &MockInventoryClient{}

	mockCirc.On("Renew", mock.Anything, mock.Anything, mock.Anything).Return(loan, nil)
	mockInv.On("GetInstanceByID", mock.Anything, mock.Anything, "instance-r-002").Return(instance, nil)

	h := NewRenewHandler(zap.NewNop(), tc)
	injectMocks(h.BaseHandler, nil, mockCirc, mockInv, nil)

	msg := buildTestMsg(parser.RenewRequest, map[parser.FieldCode]string{
		parser.InstitutionID:    "TEST-INST",
		parser.PatronIdentifier: "123456",
		parser.ItemIdentifier:   "ITEM-R-002",
	})

	resp, err := h.Handle(context.Background(), msg, sess)

	require.NoError(t, err)
	// Response must include exactly 60 'A' chars after AJ, not the full 80.
	assert.Contains(t, resp, "AJ"+strings.Repeat("A", 60))
	assert.NotContains(t, resp, "AJ"+strings.Repeat("A", 61))

	mockCirc.AssertExpectations(t)
	mockInv.AssertExpectations(t)
}

// TestRenewHandle_TitleFallbackOnInstanceError verifies that when GetInstanceByID
// returns an error, the handler falls back to using the item barcode as the AJ
// title rather than failing the renewal.
func TestRenewHandle_TitleFallbackOnInstanceError(t *testing.T) {
	tc          := testutil.NewTenantConfig()
	sess        := testutil.NewAuthSession(tc, testutil.WithLocationCode("test-service-point-uuid"))
	itemBarcode := "ITEM-R-003"
	loan        := openRenewLoanWithDueDate("loan-r-003", "item-r-003", "instance-r-003")

	mockCirc := &MockCirculationClient{}
	mockInv  := &MockInventoryClient{}

	mockCirc.On("Renew", mock.Anything, mock.Anything, mock.Anything).Return(loan, nil)
	mockInv.On("GetInstanceByID", mock.Anything, mock.Anything, "instance-r-003").
		Return(nil, errors.New("not found"))

	h := NewRenewHandler(zap.NewNop(), tc)
	injectMocks(h.BaseHandler, nil, mockCirc, mockInv, nil)

	msg := buildTestMsg(parser.RenewRequest, map[parser.FieldCode]string{
		parser.InstitutionID:    "TEST-INST",
		parser.PatronIdentifier: "123456",
		parser.ItemIdentifier:   itemBarcode,
	})

	resp, err := h.Handle(context.Background(), msg, sess)

	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(resp, "30"), "response must start with 30")
	// When instance fetch fails, AJ falls back to the item barcode.
	assert.Contains(t, resp, "AJ"+itemBarcode)

	mockCirc.AssertExpectations(t)
	mockInv.AssertExpectations(t)
}

// TestRenewHandle_TitleFallbackNoInstanceID verifies that when the renewal loan
// response has no Item (nil), the handler performs a secondary item lookup via
// GetItemByID. If that item also carries no InstanceID and no HoldingsRecordID,
// the AJ field falls back to the item barcode from the original request.
func TestRenewHandle_TitleFallbackNoInstanceID(t *testing.T) {
	tc          := testutil.NewTenantConfig()
	sess        := testutil.NewAuthSession(tc, testutil.WithLocationCode("test-service-point-uuid"))
	itemBarcode := "ITEM-R-004"

	due  := time.Now().Add(14 * 24 * time.Hour)
	loan := &models.Loan{
		ID:      "loan-r-004",
		ItemID:  "item-r-004",
		DueDate: &due,
		Item:    nil, // no Item on loan — triggers secondary lookup
	}

	// Item lookup returns an item with no InstanceID and no HoldingsRecordID.
	item := &models.Item{
		ID:               "item-r-004",
		InstanceID:       "",
		HoldingsRecordID: "",
	}

	mockCirc := &MockCirculationClient{}
	mockInv  := &MockInventoryClient{}

	mockCirc.On("Renew", mock.Anything, mock.Anything, mock.Anything).Return(loan, nil)
	mockInv.On("GetItemByID", mock.Anything, mock.Anything, "item-r-004").Return(item, nil)

	h := NewRenewHandler(zap.NewNop(), tc)
	injectMocks(h.BaseHandler, nil, mockCirc, mockInv, nil)

	msg := buildTestMsg(parser.RenewRequest, map[parser.FieldCode]string{
		parser.InstitutionID:    "TEST-INST",
		parser.PatronIdentifier: "123456",
		parser.ItemIdentifier:   itemBarcode,
	})

	resp, err := h.Handle(context.Background(), msg, sess)

	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(resp, "30"), "response must start with 30")
	// No instance ID resolved — AJ falls back to the item barcode in the request.
	assert.Contains(t, resp, "AJ"+itemBarcode)

	mockCirc.AssertExpectations(t)
	mockInv.AssertExpectations(t)
}

// TestRenewHandle_TitleExactly60Chars verifies that an instance title of exactly
// 60 characters is NOT truncated — it must appear in the AJ field unchanged.
func TestRenewHandle_TitleExactly60Chars(t *testing.T) {
	tc   := testutil.NewTenantConfig()
	sess := testutil.NewAuthSession(tc, testutil.WithLocationCode("test-service-point-uuid"))
	loan := openRenewLoanWithDueDate("loan-r-005", "item-r-005", "instance-r-005")

	exactTitle := strings.Repeat("B", 60)
	instance   := &models.Instance{ID: "instance-r-005", Title: exactTitle}

	mockCirc := &MockCirculationClient{}
	mockInv  := &MockInventoryClient{}

	mockCirc.On("Renew", mock.Anything, mock.Anything, mock.Anything).Return(loan, nil)
	mockInv.On("GetInstanceByID", mock.Anything, mock.Anything, "instance-r-005").Return(instance, nil)

	h := NewRenewHandler(zap.NewNop(), tc)
	injectMocks(h.BaseHandler, nil, mockCirc, mockInv, nil)

	msg := buildTestMsg(parser.RenewRequest, map[parser.FieldCode]string{
		parser.InstitutionID:    "TEST-INST",
		parser.PatronIdentifier: "123456",
		parser.ItemIdentifier:   "ITEM-R-005",
	})

	resp, err := h.Handle(context.Background(), msg, sess)

	require.NoError(t, err)
	// Exactly 60 chars must appear in the AJ field — no truncation should occur.
	assert.Contains(t, resp, "AJ"+strings.Repeat("B", 60))
	// 61 'B' chars after AJ would indicate the title was not truncated but somehow
	// grew; guard against that as a sanity check.
	assert.NotContains(t, resp, "AJ"+strings.Repeat("B", 61))

	mockCirc.AssertExpectations(t)
	mockInv.AssertExpectations(t)
}

// TestRenewHandle_EmptyTitleFallsBackToBarcode verifies that when GetInstanceByID
// succeeds but returns an instance with an empty Title, the AJ field falls back to
// the item barcode from the original SIP2 request.
func TestRenewHandle_EmptyTitleFallsBackToBarcode(t *testing.T) {
	tc          := testutil.NewTenantConfig()
	sess        := testutil.NewAuthSession(tc, testutil.WithLocationCode("test-service-point-uuid"))
	itemBarcode := "ITEM-R-006"
	loan        := openRenewLoanWithDueDate("loan-r-006", "item-r-006", "instance-r-006")

	instance := &models.Instance{ID: "instance-r-empty", Title: ""}

	mockCirc := &MockCirculationClient{}
	mockInv  := &MockInventoryClient{}

	mockCirc.On("Renew", mock.Anything, mock.Anything, mock.Anything).Return(loan, nil)
	mockInv.On("GetInstanceByID", mock.Anything, mock.Anything, "instance-r-006").Return(instance, nil)

	h := NewRenewHandler(zap.NewNop(), tc)
	injectMocks(h.BaseHandler, nil, mockCirc, mockInv, nil)

	msg := buildTestMsg(parser.RenewRequest, map[parser.FieldCode]string{
		parser.InstitutionID:    "TEST-INST",
		parser.PatronIdentifier: "123456",
		parser.ItemIdentifier:   itemBarcode,
	})

	resp, err := h.Handle(context.Background(), msg, sess)

	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(resp, "30"), "response must start with 30")
	// Empty title from instance — AJ must fall back to the item barcode.
	assert.Contains(t, resp, "AJ"+itemBarcode)

	mockCirc.AssertExpectations(t)
	mockInv.AssertExpectations(t)
}

// TestRenewHandle_UnicodeTitleTruncatedCorrectly verifies that truncation of a
// Unicode instance title happens at a rune boundary rather than a byte boundary.
// Each Japanese character '本' is 3 UTF-8 bytes; truncating at byte 60 would
// split a rune.  The handler must truncate to 60 *runes*, not 60 bytes.
func TestRenewHandle_UnicodeTitleTruncatedCorrectly(t *testing.T) {
	tc   := testutil.NewTenantConfig()
	sess := testutil.NewAuthSession(tc, testutil.WithLocationCode("test-service-point-uuid"))
	loan := openRenewLoanWithDueDate("loan-r-007", "item-r-007", "instance-r-007")

	unicodeTitle := strings.Repeat("本", 80)
	instance     := &models.Instance{ID: "instance-r-007", Title: unicodeTitle}

	mockCirc := &MockCirculationClient{}
	mockInv  := &MockInventoryClient{}

	mockCirc.On("Renew", mock.Anything, mock.Anything, mock.Anything).Return(loan, nil)
	mockInv.On("GetInstanceByID", mock.Anything, mock.Anything, "instance-r-007").Return(instance, nil)

	h := NewRenewHandler(zap.NewNop(), tc)
	injectMocks(h.BaseHandler, nil, mockCirc, mockInv, nil)

	msg := buildTestMsg(parser.RenewRequest, map[parser.FieldCode]string{
		parser.InstitutionID:    "TEST-INST",
		parser.PatronIdentifier: "123456",
		parser.ItemIdentifier:   "ITEM-R-007",
	})

	resp, err := h.Handle(context.Background(), msg, sess)

	require.NoError(t, err)
	// After rune-aware truncation to 60, exactly 60 '本' chars must follow AJ.
	assert.Contains(t, resp, "AJ"+strings.Repeat("本", 60))
	// 61 copies would mean truncation did not occur correctly.
	assert.NotContains(t, resp, strings.Repeat("本", 61))

	mockCirc.AssertExpectations(t)
	mockInv.AssertExpectations(t)
}
