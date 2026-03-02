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

// TestItemInformationHandle_BarcodeSuccess verifies that a barcode lookup
// succeeds and the response starts with "18".
func TestItemInformationHandle_BarcodeSuccess(t *testing.T) {
	tc := testutil.NewTenantConfig()
	sess := testutil.NewAuthSession(tc)
	item := availableItemNoHoldings("item-ii-001", "ITEM-II-001")

	mockInv := &MockInventoryClient{}
	mockCirc := &MockCirculationClient{}

	mockInv.On("GetItemByBarcode", mock.Anything, mock.Anything, "ITEM-II-001").
		Return(item, nil)
	// GetRequestsByItem is called because CF/CM/CY/DA fields are enabled by default.
	mockCirc.On("GetRequestsByItem", mock.Anything, mock.Anything, item.ID).
		Return(&models.RequestCollection{}, nil)

	h := NewItemInformationHandler(zap.NewNop(), tc)
	injectMocks(h.BaseHandler, nil, mockCirc, mockInv, nil)

	msg := buildTestMsg(parser.ItemInformationRequest, map[parser.FieldCode]string{
		parser.InstitutionID:  "TEST-INST",
		parser.ItemIdentifier: "ITEM-II-001",
	})

	resp, err := h.Handle(context.Background(), msg, sess)

	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(resp, "18"), "response must start with 18")
	mockInv.AssertExpectations(t)
	mockCirc.AssertExpectations(t)
}

// TestItemInformationHandle_BarcodeNotFound verifies that a failed barcode lookup
// returns a degraded "18" response with no error (handler absorbs the error).
func TestItemInformationHandle_BarcodeNotFound(t *testing.T) {
	tc := testutil.NewTenantConfig()
	sess := testutil.NewAuthSession(tc)

	mockInv := &MockInventoryClient{}
	mockInv.On("GetItemByBarcode", mock.Anything, mock.Anything, "UNKNOWN").
		Return(nil, fmt.Errorf("not found"))

	h := NewItemInformationHandler(zap.NewNop(), tc)
	injectMocks(h.BaseHandler, nil, nil, mockInv, nil)

	msg := buildTestMsg(parser.ItemInformationRequest, map[parser.FieldCode]string{
		parser.InstitutionID:  "TEST-INST",
		parser.ItemIdentifier: "UNKNOWN",
	})

	resp, err := h.Handle(context.Background(), msg, sess)

	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(resp, "18"), "degraded response must start with 18")
	mockInv.AssertExpectations(t)
}

// TestItemInformationHandle_UUIDPath_Success verifies that a UUID in the
// ItemIdentifier field triggers instance-level lookup and returns "18".
func TestItemInformationHandle_UUIDPath_Success(t *testing.T) {
	tc := testutil.NewTenantConfig()
	sess := testutil.NewAuthSession(tc)

	instanceUUID := "550e8400-e29b-41d4-a716-446655440000"
	instance := &models.Instance{ID: instanceUUID, Title: "Test Book Title"}

	mockInv := &MockInventoryClient{}
	mockInv.On("GetInstanceByID", mock.Anything, mock.Anything, instanceUUID).
		Return(instance, nil)

	h := NewItemInformationHandler(zap.NewNop(), tc)
	injectMocks(h.BaseHandler, nil, nil, mockInv, nil)

	msg := buildTestMsg(parser.ItemInformationRequest, map[parser.FieldCode]string{
		parser.InstitutionID:  "TEST-INST",
		parser.ItemIdentifier: instanceUUID,
	})

	resp, err := h.Handle(context.Background(), msg, sess)

	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(resp, "18"), "response must start with 18")
	mockInv.AssertExpectations(t)
}

// TestItemInformationHandle_UUIDPath_NotFound verifies that a UUID lookup failure
// returns a degraded "18" response with no error.
func TestItemInformationHandle_UUIDPath_NotFound(t *testing.T) {
	tc := testutil.NewTenantConfig()
	sess := testutil.NewAuthSession(tc)

	instanceUUID := "550e8400-e29b-41d4-a716-446655440000"

	mockInv := &MockInventoryClient{}
	mockInv.On("GetInstanceByID", mock.Anything, mock.Anything, instanceUUID).
		Return(nil, fmt.Errorf("not found"))

	h := NewItemInformationHandler(zap.NewNop(), tc)
	injectMocks(h.BaseHandler, nil, nil, mockInv, nil)

	msg := buildTestMsg(parser.ItemInformationRequest, map[parser.FieldCode]string{
		parser.InstitutionID:  "TEST-INST",
		parser.ItemIdentifier: instanceUUID,
	})

	resp, err := h.Handle(context.Background(), msg, sess)

	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(resp, "18"), "degraded response must start with 18")
	mockInv.AssertExpectations(t)
}

// TestItemInformationHandle_MissingFields verifies that a missing ItemIdentifier
// causes a validation error and a "96" (resend) response.
func TestItemInformationHandle_MissingFields(t *testing.T) {
	tc := testutil.NewTenantConfig()
	sess := testutil.NewAuthSession(tc)

	h := NewItemInformationHandler(zap.NewNop(), tc)

	// No ItemIdentifier field.
	msg := buildTestMsg(parser.ItemInformationRequest, map[parser.FieldCode]string{
		parser.InstitutionID: "TEST-INST",
	})

	resp, err := h.Handle(context.Background(), msg, sess)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "validation failed")
	assert.Equal(t, "96", resp)
}
