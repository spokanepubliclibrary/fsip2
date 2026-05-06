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
	sess.SetLocationCode("SP-001")

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
	sess.SetLocationCode("SP-001")

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
	sess.SetLocationCode("SP-001")

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

// TestCheckinHandle_MissingCP verifies that a checkin request fails immediately
// with ok=0 when the session has no CP (LocationCode) set.
func TestCheckinHandle_MissingCP(t *testing.T) {
	tc := testutil.NewTenantConfig()
	// Authenticated session but NO LocationCode set — simulates missing CP at login
	sess := testutil.NewAuthSession(tc)

	h := NewCheckinHandler(zap.NewNop(), tc)

	msg := buildTestMsg(parser.CheckinRequest, map[parser.FieldCode]string{
		parser.InstitutionID:   "TEST-INST",
		parser.ItemIdentifier:  "ITEM-001",
		parser.CurrentLocation: "SP-001", // AP is present but CP is absent from session
	})

	resp, err := h.Handle(context.Background(), msg, sess)

	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(resp, "10"), "response must start with 10")
	assert.Equal(t, byte('0'), resp[2], "ok byte must be '0' when CP is absent from session")
}

// TestCheckinHandle_APEchoedInResponse verifies that when AP is provided in the 09
// request, the AP value (not the CP/service point value) appears in the 10 response.
func TestCheckinHandle_APEchoedInResponse(t *testing.T) {
	tc := testutil.NewTenantConfig()
	sess := testutil.NewAuthSession(tc)
	// CP holds the actual service point UUID; AP is a different value
	sess.SetLocationCode("service-point-uuid-from-cp")

	item := availableItemNoHoldings("item-004", "ITEM-004")
	loan := closedLoan()

	mockInv := &MockInventoryClient{}
	mockCirc := &MockCirculationClient{}

	mockInv.On("GetItemByBarcode", mock.Anything, mock.Anything, "ITEM-004").Return(item, nil)
	mockCirc.On("Checkin", mock.Anything, mock.Anything, mock.Anything).Return(loan, nil)
	mockCirc.On("GetRequestsByItem", mock.Anything, mock.Anything, item.ID).
		Return(&models.RequestCollection{}, nil)

	h := NewCheckinHandler(zap.NewNop(), tc)
	injectMocks(h.BaseHandler, nil, mockCirc, mockInv, nil)

	msg := buildTestMsg(parser.CheckinRequest, map[parser.FieldCode]string{
		parser.InstitutionID:   "TEST-INST",
		parser.ItemIdentifier:  "ITEM-004",
		parser.CurrentLocation: "ap-location-value", // AP value distinct from CP
	})

	resp, err := h.Handle(context.Background(), msg, sess)

	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(resp, "10"))
	assert.Equal(t, byte('1'), resp[2], "ok byte must be '1' for successful checkin")
	// AP value from request should be echoed in the response
	assert.Contains(t, resp, "APap-location-value", "AP from request must be echoed in 10 response")
	// CP/service-point UUID must NOT appear as the AP value in the response
	assert.NotContains(t, resp, "APservice-point-uuid-from-cp", "CP value must not be echoed as AP in 10 response")

	mockInv.AssertExpectations(t)
	mockCirc.AssertExpectations(t)
}

// TestCheckinHandle_LocalHold_AwaitingPickup_NoRequests verifies CV=01 is set even
// when GetRequestsByItem returns an empty collection, as long as the item status is
// "Awaiting pickup". This is the primary regression test for the CV field bug.
func TestCheckinHandle_LocalHold_AwaitingPickup_NoRequests(t *testing.T) {
	tc := testutil.NewTenantConfig()
	sess := testutil.NewAuthSession(tc)
	sess.SetLocationCode("SP-001")

	item := &models.Item{
		ID:      "item-await-001",
		Barcode: "ITEM-AWAIT-001",
		Status:  models.ItemStatus{Name: "Awaiting pickup"},
		Location: &models.Location{
			ID:   "loc-001",
			Name: "Main Stacks",
		},
		MaterialType: &models.MaterialType{
			ID:   "mt-001",
			Name: "Book",
		},
	}
	loan := closedLoan()

	mockInv := &MockInventoryClient{}
	mockCirc := &MockCirculationClient{}

	mockInv.On("GetItemByBarcode", mock.Anything, mock.Anything, "ITEM-AWAIT-001").Return(item, nil)
	mockCirc.On("Checkin", mock.Anything, mock.Anything, mock.Anything).Return(loan, nil)
	mockCirc.On("GetRequestsByItem", mock.Anything, mock.Anything, item.ID).
		Return(&models.RequestCollection{}, nil)

	h := NewCheckinHandler(zap.NewNop(), tc)
	injectMocks(h.BaseHandler, nil, mockCirc, mockInv, nil)

	msg := buildTestMsg(parser.CheckinRequest, map[parser.FieldCode]string{
		parser.InstitutionID:   "TEST-INST",
		parser.ItemIdentifier:  "ITEM-AWAIT-001",
		parser.CurrentLocation: "SP-001",
	})

	resp, err := h.Handle(context.Background(), msg, sess)

	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(resp, "10"), "response must start with 10")
	assert.Contains(t, resp, "CV01", "CV=01 must be set for Awaiting pickup status")

	mockInv.AssertExpectations(t)
	mockCirc.AssertExpectations(t)
}

// TestCheckinHandle_LocalHold_AwaitingPickup_WithHoldRequest verifies CV=01 and that
// CM (hold shelf expiration) and DA (requestor name) are populated when an
// "Open - Awaiting pickup" Hold request is returned.
func TestCheckinHandle_LocalHold_AwaitingPickup_WithHoldRequest(t *testing.T) {
	tc := testutil.NewTenantConfig()
	sess := testutil.NewAuthSession(tc)
	sess.SetLocationCode("SP-001")

	item := &models.Item{
		ID:      "item-hold-002",
		Barcode: "ITEM-HOLD-002",
		Status:  models.ItemStatus{Name: "Awaiting pickup"},
		Location: &models.Location{
			ID:   "loc-001",
			Name: "Main Stacks",
		},
		MaterialType: &models.MaterialType{
			ID:   "mt-001",
			Name: "Book",
		},
	}
	loan := closedLoan()

	expiration := time.Now().Add(7 * 24 * time.Hour)
	holdRequest := models.Request{
		ID:                      "req-hold-002",
		RequestType:             "Hold",
		Status:                  "Open - Awaiting pickup",
		ItemID:                  item.ID,
		PickupServicePointID:    "sp-local-uuid",
		HoldShelfExpirationDate: &expiration,
		Requester: &models.RequestRequester{
			FirstName: "Jane",
			LastName:  "Doe",
		},
	}

	mockInv := &MockInventoryClient{}
	mockCirc := &MockCirculationClient{}

	mockInv.On("GetItemByBarcode", mock.Anything, mock.Anything, "ITEM-HOLD-002").Return(item, nil)
	mockCirc.On("Checkin", mock.Anything, mock.Anything, mock.Anything).Return(loan, nil)
	mockCirc.On("GetRequestsByItem", mock.Anything, mock.Anything, item.ID).
		Return(&models.RequestCollection{Requests: []models.Request{holdRequest}}, nil)
	mockInv.On("GetServicePointByID", mock.Anything, mock.Anything, "sp-local-uuid").
		Return(&models.ServicePoint{ID: "sp-local-uuid", Name: "Main Library", Code: "ML"}, nil)

	h := NewCheckinHandler(zap.NewNop(), tc)
	injectMocks(h.BaseHandler, nil, mockCirc, mockInv, nil)

	msg := buildTestMsg(parser.CheckinRequest, map[parser.FieldCode]string{
		parser.InstitutionID:   "TEST-INST",
		parser.ItemIdentifier:  "ITEM-HOLD-002",
		parser.CurrentLocation: "SP-001",
	})

	resp, err := h.Handle(context.Background(), msg, sess)

	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(resp, "10"), "response must start with 10")
	assert.Contains(t, resp, "CV01", "CV=01 must be set for local hold awaiting pickup")
	assert.Contains(t, resp, "DADoe, Jane", "DA field must contain requestor name in lastName, firstName format")
	assert.Contains(t, resp, "CM", "CM field (hold shelf expiration) must be present")

	mockInv.AssertExpectations(t)
	mockCirc.AssertExpectations(t)
}

// TestCheckinHandle_RemoteHold_InTransit_WithHold verifies CV=02 when item is
// "In transit" and an open Hold request exists for a remote pickup location.
func TestCheckinHandle_RemoteHold_InTransit_WithHold(t *testing.T) {
	tc := testutil.NewTenantConfig()
	sess := testutil.NewAuthSession(tc)
	sess.SetLocationCode("SP-001")

	item := &models.Item{
		ID:                                 "item-transit-003",
		Barcode:                            "ITEM-TRANSIT-003",
		Status:                             models.ItemStatus{Name: "In transit"},
		InTransitDestinationServicePointID: "sp-remote-uuid",
		Location: &models.Location{
			ID:   "loc-001",
			Name: "Main Stacks",
		},
		MaterialType: &models.MaterialType{
			ID:   "mt-001",
			Name: "Book",
		},
	}
	loan := closedLoan()

	holdRequest := models.Request{
		ID:                   "req-transit-003",
		RequestType:          "Hold",
		Status:               "Open - In transit",
		ItemID:               item.ID,
		PickupServicePointID: "sp-remote-uuid",
	}

	mockInv := &MockInventoryClient{}
	mockCirc := &MockCirculationClient{}

	mockInv.On("GetItemByBarcode", mock.Anything, mock.Anything, "ITEM-TRANSIT-003").Return(item, nil)
	mockCirc.On("Checkin", mock.Anything, mock.Anything, mock.Anything).Return(loan, nil)
	mockCirc.On("GetRequestsByItem", mock.Anything, mock.Anything, item.ID).
		Return(&models.RequestCollection{Requests: []models.Request{holdRequest}}, nil)
	mockInv.On("GetServicePointByID", mock.Anything, mock.Anything, "sp-remote-uuid").
		Return(&models.ServicePoint{ID: "sp-remote-uuid", Name: "Branch Library", Code: "BL"}, nil)

	h := NewCheckinHandler(zap.NewNop(), tc)
	injectMocks(h.BaseHandler, nil, mockCirc, mockInv, nil)

	msg := buildTestMsg(parser.CheckinRequest, map[parser.FieldCode]string{
		parser.InstitutionID:   "TEST-INST",
		parser.ItemIdentifier:  "ITEM-TRANSIT-003",
		parser.CurrentLocation: "SP-001",
	})

	resp, err := h.Handle(context.Background(), msg, sess)

	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(resp, "10"), "response must start with 10")
	assert.Contains(t, resp, "CV02", "CV=02 must be set for in-transit item with hold")
	assert.Contains(t, resp, "CTBranch Library", "CT field must contain destination service point name")

	mockInv.AssertExpectations(t)
	mockCirc.AssertExpectations(t)
}

// TestCheckinHandle_InTransit_NoHold verifies CV=04 when item is "In transit"
// with no open hold or recall — item returning to home location.
func TestCheckinHandle_InTransit_NoHold(t *testing.T) {
	tc := testutil.NewTenantConfig()
	sess := testutil.NewAuthSession(tc)
	sess.SetLocationCode("SP-001")

	item := &models.Item{
		ID:      "item-transit-004",
		Barcode: "ITEM-TRANSIT-004",
		Status:  models.ItemStatus{Name: "In transit"},
		Location: &models.Location{
			ID:   "loc-001",
			Name: "Main Stacks",
		},
		MaterialType: &models.MaterialType{
			ID:   "mt-001",
			Name: "Book",
		},
	}
	loan := closedLoan()

	mockInv := &MockInventoryClient{}
	mockCirc := &MockCirculationClient{}

	mockInv.On("GetItemByBarcode", mock.Anything, mock.Anything, "ITEM-TRANSIT-004").Return(item, nil)
	mockCirc.On("Checkin", mock.Anything, mock.Anything, mock.Anything).Return(loan, nil)
	mockCirc.On("GetRequestsByItem", mock.Anything, mock.Anything, item.ID).
		Return(&models.RequestCollection{}, nil)

	h := NewCheckinHandler(zap.NewNop(), tc)
	injectMocks(h.BaseHandler, nil, mockCirc, mockInv, nil)

	msg := buildTestMsg(parser.CheckinRequest, map[parser.FieldCode]string{
		parser.InstitutionID:   "TEST-INST",
		parser.ItemIdentifier:  "ITEM-TRANSIT-004",
		parser.CurrentLocation: "SP-001",
	})

	resp, err := h.Handle(context.Background(), msg, sess)

	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(resp, "10"), "response must start with 10")
	assert.Contains(t, resp, "CV04", "CV=04 must be set for in-transit item with no hold")

	mockInv.AssertExpectations(t)
	mockCirc.AssertExpectations(t)
}

// TestCheckinHandle_Available_NoAlert verifies no CV field appears in the response
// when the item returns to "Available" status with no open requests.
func TestCheckinHandle_Available_NoAlert(t *testing.T) {
	tc := testutil.NewTenantConfig()
	sess := testutil.NewAuthSession(tc)
	sess.SetLocationCode("SP-001")

	item := availableItemNoHoldings("item-avail-005", "ITEM-AVAIL-005")
	loan := closedLoan()

	mockInv := &MockInventoryClient{}
	mockCirc := &MockCirculationClient{}

	mockInv.On("GetItemByBarcode", mock.Anything, mock.Anything, "ITEM-AVAIL-005").Return(item, nil)
	mockCirc.On("Checkin", mock.Anything, mock.Anything, mock.Anything).Return(loan, nil)
	mockCirc.On("GetRequestsByItem", mock.Anything, mock.Anything, item.ID).
		Return(&models.RequestCollection{}, nil)

	h := NewCheckinHandler(zap.NewNop(), tc)
	injectMocks(h.BaseHandler, nil, mockCirc, mockInv, nil)

	msg := buildTestMsg(parser.CheckinRequest, map[parser.FieldCode]string{
		parser.InstitutionID:   "TEST-INST",
		parser.ItemIdentifier:  "ITEM-AVAIL-005",
		parser.CurrentLocation: "SP-001",
	})

	resp, err := h.Handle(context.Background(), msg, sess)

	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(resp, "10"), "response must start with 10")
	assert.Equal(t, byte('1'), resp[2], "ok byte must be '1' for successful checkin")
	// The builder emits CV as an empty field when there is no alert; verify none of the
	// alert type values (01, 02, 04) are present.
	assert.NotContains(t, resp, "CV01", "CV=01 must not be set for an Available item")
	assert.NotContains(t, resp, "CV02", "CV=02 must not be set for an Available item")
	assert.NotContains(t, resp, "CV04", "CV=04 must not be set for an Available item")

	mockInv.AssertExpectations(t)
	mockCirc.AssertExpectations(t)
}
