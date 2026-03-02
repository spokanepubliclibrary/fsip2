package handlers

import (
	"context"
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

// availableItemNoHoldings builds a minimal item that avoids extra inventory
// lookups: location and material type are pre-populated, no holdings record ID.
func availableItemNoHoldings(id, barcode string) *models.Item {
	return &models.Item{
		ID:      id,
		Barcode: barcode,
		Status:  models.ItemStatus{Name: "Available"},
		Location: &models.Location{
			ID:   "loc-001",
			Name: "Main Stacks",
		},
		MaterialType: &models.MaterialType{
			ID:   "mt-001",
			Name: "Book",
		},
		// HoldingsRecordID intentionally empty — avoids holdings/instance lookups.
	}
}

// closedLoan returns a minimal Loan sufficient for a checkin response.
func closedLoan() *models.Loan {
	return &models.Loan{ID: "loan-001"}
}

// TestCheckinHandle_Success verifies a full success path: item not claimed returned,
// checkin succeeds, response starts with "10" and ok byte is '1'.
func TestCheckinHandle_Success(t *testing.T) {
	tc := testutil.NewTenantConfig()
	sess := testutil.NewAuthSession(tc)

	item := availableItemNoHoldings("item-001", "ITEM-001")
	loan := closedLoan()

	mockInv := &MockInventoryClient{}
	mockCirc := &MockCirculationClient{}

	// GetItemByBarcode is called twice:
	//   1. Claimed-returned status check in Handle()
	//   2. Item fetch inside fetchCheckinResponseData()
	mockInv.On("GetItemByBarcode", mock.Anything, mock.Anything, "ITEM-001").Return(item, nil)
	mockCirc.On("Checkin", mock.Anything, mock.Anything, mock.Anything).Return(loan, nil)
	mockCirc.On("GetRequestsByItem", mock.Anything, mock.Anything, item.ID).
		Return(&models.RequestCollection{}, nil)

	h := NewCheckinHandler(zap.NewNop(), tc)
	injectMocks(h.BaseHandler, nil, mockCirc, mockInv, nil)

	msg := buildTestMsg(parser.CheckinRequest, map[parser.FieldCode]string{
		parser.InstitutionID:   "TEST-INST",
		parser.ItemIdentifier:  "ITEM-001",
		parser.CurrentLocation: "SP-001",
	})

	resp, err := h.Handle(context.Background(), msg, sess)

	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(resp, "10"), "response must start with 10")
	assert.Equal(t, byte('1'), resp[2], "ok byte must be '1' for successful checkin")

	mockInv.AssertExpectations(t)
	mockCirc.AssertExpectations(t)
}

// TestCheckinHandle_CheckinFails verifies that a Checkin API error produces
// an ok=0 response.
func TestCheckinHandle_CheckinFails(t *testing.T) {
	tc := testutil.NewTenantConfig()
	sess := testutil.NewAuthSession(tc)

	item := availableItemNoHoldings("item-002", "ITEM-002")

	mockInv := &MockInventoryClient{}
	mockCirc := &MockCirculationClient{}

	mockInv.On("GetItemByBarcode", mock.Anything, mock.Anything, "ITEM-002").Return(item, nil)
	mockCirc.On("Checkin", mock.Anything, mock.Anything, mock.Anything).
		Return(nil, assert.AnError)

	h := NewCheckinHandler(zap.NewNop(), tc)
	injectMocks(h.BaseHandler, nil, mockCirc, mockInv, nil)

	msg := buildTestMsg(parser.CheckinRequest, map[parser.FieldCode]string{
		parser.InstitutionID:   "TEST-INST",
		parser.ItemIdentifier:  "ITEM-002",
		parser.CurrentLocation: "SP-001",
	})

	resp, err := h.Handle(context.Background(), msg, sess)

	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(resp, "10"))
	assert.Equal(t, byte('0'), resp[2], "ok byte must be '0' for failed checkin")

	mockInv.AssertExpectations(t)
	mockCirc.AssertExpectations(t)
}

// TestCheckinHandle_FetchResponseData_ItemWithHoldings verifies that when an item
// has a HoldingsRecordID, the holdings→instance chain is followed and the instance
// title is included in the response.
func TestCheckinHandle_FetchResponseData_ItemWithHoldings(t *testing.T) {
	tc := testutil.NewTenantConfig()
	sess := testutil.NewAuthSession(tc)

	itemWithHoldings := &models.Item{
		ID:               "item-003",
		Barcode:          "ITEM-003",
		Status:           models.ItemStatus{Name: "Available"},
		HoldingsRecordID: "holdings-abc",
		Location:         &models.Location{Name: "Main Stacks"},
		MaterialType:     &models.MaterialType{Name: "Book"},
	}
	holdings := &models.Holdings{ID: "holdings-abc", InstanceID: "instance-xyz"}
	instance := &models.Instance{ID: "instance-xyz", Title: "A Great Novel"}
	loan := closedLoan()

	mockInv := &MockInventoryClient{}
	mockCirc := &MockCirculationClient{}

	mockInv.On("GetItemByBarcode", mock.Anything, mock.Anything, "ITEM-003").Return(itemWithHoldings, nil)
	mockCirc.On("Checkin", mock.Anything, mock.Anything, mock.Anything).Return(loan, nil)
	mockInv.On("GetHoldingsByID", mock.Anything, mock.Anything, "holdings-abc").Return(holdings, nil)
	mockInv.On("GetInstanceByID", mock.Anything, mock.Anything, "instance-xyz").Return(instance, nil)
	mockCirc.On("GetRequestsByItem", mock.Anything, mock.Anything, itemWithHoldings.ID).
		Return(&models.RequestCollection{}, nil)

	h := NewCheckinHandler(zap.NewNop(), tc)
	injectMocks(h.BaseHandler, nil, mockCirc, mockInv, nil)

	msg := buildTestMsg(parser.CheckinRequest, map[parser.FieldCode]string{
		parser.InstitutionID:   "TEST-INST",
		parser.ItemIdentifier:  "ITEM-003",
		parser.CurrentLocation: "SP-001",
	})

	resp, err := h.Handle(context.Background(), msg, sess)

	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(resp, "10"))
	assert.Equal(t, byte('1'), resp[2], "ok byte must be '1'")
	assert.Contains(t, resp, "A Great Novel", "instance title must appear in response")

	mockInv.AssertExpectations(t)
	mockCirc.AssertExpectations(t)
}
