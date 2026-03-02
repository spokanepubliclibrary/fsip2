package handlers

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/spokanepubliclibrary/fsip2/internal/sip2/parser"
	"github.com/spokanepubliclibrary/fsip2/tests/testutil"
)

func TestItemStatusUpdateHandle_Success(t *testing.T) {
	tc := testutil.NewTenantConfig()
	sess := testutil.NewAuthSession(tc)
	item := testutil.DefaultItem()

	mockInv := &MockInventoryClient{}
	mockInv.On("GetItemByBarcode", mock.Anything, mock.Anything, item.Barcode).
		Return(item, nil)

	h := NewItemStatusUpdateHandler(zap.NewNop(), tc)
	injectMocks(h.BaseHandler, nil, nil, mockInv, nil)

	msg := buildTestMsg(parser.ItemStatusUpdateRequest, map[parser.FieldCode]string{
		parser.InstitutionID:  "TEST-INST",
		parser.ItemIdentifier: item.Barcode,
	})

	resp, err := h.Handle(context.Background(), msg, sess)

	require.NoError(t, err)
	assert.Contains(t, resp, "201")
	assert.Contains(t, resp, "|AFItem properties updated")
	mockInv.AssertExpectations(t)
}

func TestItemStatusUpdateHandle_ItemNotFound(t *testing.T) {
	tc := testutil.NewTenantConfig()
	sess := testutil.NewAuthSession(tc)

	mockInv := &MockInventoryClient{}
	mockInv.On("GetItemByBarcode", mock.Anything, mock.Anything, "MISSING").
		Return(nil, fmt.Errorf("not found"))

	h := NewItemStatusUpdateHandler(zap.NewNop(), tc)
	injectMocks(h.BaseHandler, nil, nil, mockInv, nil)

	msg := buildTestMsg(parser.ItemStatusUpdateRequest, map[parser.FieldCode]string{
		parser.InstitutionID:  "TEST-INST",
		parser.ItemIdentifier: "MISSING",
	})

	resp, err := h.Handle(context.Background(), msg, sess)

	require.NoError(t, err)
	assert.Contains(t, resp, "200")
	assert.Contains(t, resp, "|AFItem properties update failed")
	mockInv.AssertExpectations(t)
}

func TestItemStatusUpdateHandle_MissingFields(t *testing.T) {
	tc := testutil.NewTenantConfig()
	sess := testutil.NewAuthSession(tc)

	h := NewItemStatusUpdateHandler(zap.NewNop(), tc)

	msg := buildTestMsg(parser.ItemStatusUpdateRequest, map[parser.FieldCode]string{
		parser.InstitutionID: "TEST-INST",
		// ItemIdentifier intentionally omitted
	})

	resp, err := h.Handle(context.Background(), msg, sess)

	require.NoError(t, err)
	assert.Contains(t, resp, "200")
}
