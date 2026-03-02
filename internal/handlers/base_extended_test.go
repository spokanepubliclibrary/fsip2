package handlers

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/spokanepubliclibrary/fsip2/internal/folio/models"
	"github.com/spokanepubliclibrary/fsip2/internal/sip2/parser"
	"github.com/spokanepubliclibrary/fsip2/internal/types"
	"github.com/spokanepubliclibrary/fsip2/tests/testutil"
	"go.uber.org/zap"
)

// TestGetResponseBuilder verifies getResponseBuilder returns a non-nil builder.
func TestGetResponseBuilder(t *testing.T) {
	tc := testutil.NewTenantConfig()
	h := NewBaseHandler(zap.NewNop(), tc)
	rb := h.getResponseBuilder()
	if rb == nil {
		t.Error("getResponseBuilder() returned nil")
	}
}

// TestBuildErrorResponse_Base verifies the base buildErrorResponse returns "96".
func TestBuildErrorResponse_Base(t *testing.T) {
	tc := testutil.NewTenantConfig()
	h := NewBaseHandler(zap.NewNop(), tc)

	msg := &parser.Message{
		Code:   parser.CheckinRequest,
		Fields: make(map[string]string),
	}
	resp := h.buildErrorResponse(msg)
	if resp != "96" {
		t.Errorf("buildErrorResponse() = %q, want %q", resp, "96")
	}

	// Also test with nil message (used by item_information handler).
	resp2 := h.buildErrorResponse(nil)
	if resp2 != "96" {
		t.Errorf("buildErrorResponse(nil) = %q, want %q", resp2, "96")
	}
}

// TestGetCurrentTimestamp verifies getCurrentTimestamp returns a recent time.
func TestGetCurrentTimestamp(t *testing.T) {
	tc := testutil.NewTenantConfig()
	h := NewBaseHandler(zap.NewNop(), tc)

	before := time.Now()
	ts := h.getCurrentTimestamp()
	after := time.Now()

	if ts.Before(before) || ts.After(after) {
		t.Errorf("getCurrentTimestamp() = %v, expected between %v and %v", ts, before, after)
	}
}

// TestGetFeesClient verifies getFeesClient returns a non-nil client.
func TestGetFeesClient(t *testing.T) {
	tc := testutil.NewTenantConfig()
	h := NewBaseHandler(zap.NewNop(), tc)
	session := testutil.NewSession(tc)

	client := h.getFeesClient(session)
	if client == nil {
		t.Error("getFeesClient() returned nil")
	}
}

// TestLogResponse_ErrorPath verifies logResponse handles non-nil errors.
func TestLogResponse_ErrorPath(t *testing.T) {
	tc := testutil.NewTenantConfig()
	h := NewBaseHandler(zap.NewNop(), tc)
	session := testutil.NewSession(tc)

	// Error path (previously uncovered).
	h.logResponse("24", session, errors.New("test error"))

	// Success path.
	h.logResponse("24", session, nil)
}

// TestFormatPatronName_NilUser verifies formatPatronName handles nil user.
func TestFormatPatronName_NilUser(t *testing.T) {
	tc := testutil.NewTenantConfig()
	h := NewBaseHandler(zap.NewNop(), tc)

	result := h.formatPatronName(nil)
	if result != "" {
		t.Errorf("formatPatronName(nil) = %q, want empty string", result)
	}
}

// TestFormatPatronName_FirstNameOnly verifies formatPatronName when only first name is present.
func TestFormatPatronName_FirstNameOnly(t *testing.T) {
	tc := testutil.NewTenantConfig()
	h := NewBaseHandler(zap.NewNop(), tc)

	user := &models.User{
		Username: "jdoe",
		Personal: models.PersonalInfo{
			FirstName: "Jane",
			LastName:  "",
		},
	}
	result := h.formatPatronName(user)
	if result != "Jane" {
		t.Errorf("formatPatronName() = %q, want %q", result, "Jane")
	}
}

// TestFormatPatronName_UsernameOnly verifies formatPatronName falls back to username.
func TestFormatPatronName_UsernameOnly(t *testing.T) {
	tc := testutil.NewTenantConfig()
	h := NewBaseHandler(zap.NewNop(), tc)

	user := &models.User{
		Username: "fallback_user",
		Personal: models.PersonalInfo{
			FirstName: "",
			LastName:  "",
		},
	}
	result := h.formatPatronName(user)
	if result != "fallback_user" {
		t.Errorf("formatPatronName() = %q, want %q", result, "fallback_user")
	}
}

// TestGetAuthenticatedFolioClient_NoTenantConfig verifies error when TenantConfig is nil.
func TestGetAuthenticatedFolioClient_NoTenantConfig(t *testing.T) {
	tc := testutil.NewTenantConfig()
	h := NewBaseHandler(zap.NewNop(), tc)

	session := &types.Session{
		ID:           "test",
		TenantConfig: nil,
	}

	_, _, err := h.getAuthenticatedFolioClient(context.Background(), session)
	if err == nil {
		t.Error("Expected error when TenantConfig is nil")
	}
}

// TestGetAuthenticatedFolioClient_WithCachedToken verifies that a valid cached token is returned.
func TestGetAuthenticatedFolioClient_WithCachedToken(t *testing.T) {
	tc := testutil.NewTenantConfig()
	h := NewBaseHandler(zap.NewNop(), tc)
	session := testutil.NewSession(tc)

	// Set a valid, non-expired token.
	expiresAt := time.Now().Add(10 * time.Minute)
	session.SetAuthenticated("user", "pid", "barcode", "test-token-abc", expiresAt)

	_, token, err := h.getAuthenticatedFolioClient(context.Background(), session)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if token != "test-token-abc" {
		t.Errorf("Expected cached token 'test-token-abc', got %q", token)
	}
}

// TestGetFolioClient_SessionWithConfig verifies getFolioClient succeeds with valid config.
func TestGetFolioClient_SessionWithConfig(t *testing.T) {
	tc := testutil.NewTenantConfig()
	h := NewBaseHandler(zap.NewNop(), tc)
	session := testutil.NewSession(tc)

	client, err := h.getFolioClient(session)
	if err != nil {
		t.Errorf("getFolioClient() returned error: %v", err)
	}
	if client == nil {
		t.Error("getFolioClient() returned nil client")
	}
}

// TestFetchItemTitle_NoHoldingsID tests fetchItemTitle when item has no holdings ID.
func TestFetchItemTitle_InventoryClientError(t *testing.T) {
	tc := testutil.NewTenantConfig()
	h := NewBaseHandler(zap.NewNop(), tc)
	session := testutil.NewSession(tc)

	invClient := h.getInventoryClient(session)

	ctx := context.Background()
	// No FOLIO server running - GetItemByBarcode will fail.
	_, err := h.fetchItemTitle(ctx, invClient, "test-token", "NONEXISTENT-BARCODE")
	if err == nil {
		t.Error("Expected error when FOLIO is unavailable")
	}
}

// TestValidateRequiredField_Present tests that present fields pass validation.
func TestValidateRequiredField_Present(t *testing.T) {
	tc := testutil.NewTenantConfig()
	h := NewBaseHandler(zap.NewNop(), tc)

	msg := &parser.Message{
		Code: parser.LoginRequest,
		Fields: map[string]string{
			string(parser.LoginUserID): "testuser",
		},
	}

	err := h.validateRequiredField(msg, parser.LoginUserID, "Login User ID")
	if err != nil {
		t.Errorf("validateRequiredField() returned unexpected error: %v", err)
	}
}

// TestValidateRequiredField_Missing tests that missing fields fail validation.
func TestValidateRequiredField_Missing(t *testing.T) {
	tc := testutil.NewTenantConfig()
	h := NewBaseHandler(zap.NewNop(), tc)

	msg := &parser.Message{
		Code:   parser.LoginRequest,
		Fields: make(map[string]string),
	}

	err := h.validateRequiredField(msg, parser.LoginUserID, "Login User ID")
	if err == nil {
		t.Error("validateRequiredField() expected error for missing field")
	}
	if !strings.Contains(err.Error(), "Login User ID") {
		t.Errorf("Error should mention field name, got: %v", err)
	}
}

// TestRefreshToken_NoCredentials tests refreshToken when no credentials are stored.
func TestRefreshToken_NoCredentials(t *testing.T) {
	tc := testutil.NewTenantConfig()
	h := NewBaseHandler(zap.NewNop(), tc)
	session := testutil.NewSession(tc)
	// No credentials set.

	_, err := h.refreshToken(context.Background(), session)
	if err == nil {
		t.Error("Expected error when no credentials stored")
	}
	if !strings.Contains(err.Error(), "no credentials") {
		t.Errorf("Expected 'no credentials' error, got: %v", err)
	}
}

// TestRefreshToken_WithCredentials_NoServer tests refreshToken with credentials but no FOLIO server.
func TestRefreshToken_WithCredentials_NoServer(t *testing.T) {
	tc := testutil.NewTenantConfig()
	h := NewBaseHandler(zap.NewNop(), tc)
	session := testutil.NewSession(tc)

	// Set expired token + credentials to trigger the refresh path.
	expiredAt := time.Now().Add(-10 * time.Minute)
	session.SetAuthenticated("user", "", "", "old-token", expiredAt)
	session.SetAuthCredentials("password123")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := h.refreshToken(ctx, session)
	if err == nil {
		t.Error("Expected error when FOLIO server unavailable for token refresh")
	}
}

// TestFetchItemTitle_Success verifies the full happy path: item → holdings → instance title.
func TestFetchItemTitle_Success(t *testing.T) {
	tc := testutil.NewTenantConfig()
	h := NewBaseHandler(zap.NewNop(), tc)

	item := testutil.DefaultItem()
	holdings := &models.Holdings{ID: "holdings-001", InstanceID: "instance-001"}
	instance := &models.Instance{ID: "instance-001", Title: "Expected Title"}

	mockInv := &MockInventoryClient{}
	mockInv.On("GetItemByBarcode", mock.Anything, mock.Anything, item.Barcode).Return(item, nil)
	mockInv.On("GetHoldingsByID", mock.Anything, mock.Anything, item.HoldingsRecordID).Return(holdings, nil)
	mockInv.On("GetInstanceByID", mock.Anything, mock.Anything, holdings.InstanceID).Return(instance, nil)

	title, err := h.fetchItemTitle(context.Background(), mockInv, "test-token", item.Barcode)
	require.NoError(t, err)
	assert.Equal(t, "Expected Title", title)
	mockInv.AssertExpectations(t)
}

// TestFetchItemTitle_ItemNotFound verifies error when item barcode lookup fails.
func TestFetchItemTitle_ItemNotFound(t *testing.T) {
	tc := testutil.NewTenantConfig()
	h := NewBaseHandler(zap.NewNop(), tc)

	mockInv := &MockInventoryClient{}
	mockInv.On("GetItemByBarcode", mock.Anything, mock.Anything, "MISSING").
		Return(nil, fmt.Errorf("not found"))

	title, err := h.fetchItemTitle(context.Background(), mockInv, "test-token", "MISSING")
	assert.Error(t, err)
	assert.Empty(t, title)
	mockInv.AssertExpectations(t)
}

// TestFetchItemTitle_NoHoldingsRecordID verifies error when item has no holdings record ID.
func TestFetchItemTitle_NoHoldingsRecordID(t *testing.T) {
	tc := testutil.NewTenantConfig()
	h := NewBaseHandler(zap.NewNop(), tc)

	item := testutil.DefaultItem()
	item.HoldingsRecordID = ""

	mockInv := &MockInventoryClient{}
	mockInv.On("GetItemByBarcode", mock.Anything, mock.Anything, item.Barcode).Return(item, nil)

	title, err := h.fetchItemTitle(context.Background(), mockInv, "test-token", item.Barcode)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no holdings record ID")
	assert.Empty(t, title)
	mockInv.AssertExpectations(t)
}

// TestFetchItemTitle_HoldingsFetchFails verifies error when holdings lookup fails.
func TestFetchItemTitle_HoldingsFetchFails(t *testing.T) {
	tc := testutil.NewTenantConfig()
	h := NewBaseHandler(zap.NewNop(), tc)

	item := testutil.DefaultItem()
	mockInv := &MockInventoryClient{}
	mockInv.On("GetItemByBarcode", mock.Anything, mock.Anything, item.Barcode).Return(item, nil)
	mockInv.On("GetHoldingsByID", mock.Anything, mock.Anything, item.HoldingsRecordID).
		Return(nil, fmt.Errorf("holdings unavailable"))

	title, err := h.fetchItemTitle(context.Background(), mockInv, "test-token", item.Barcode)
	assert.Error(t, err)
	assert.Empty(t, title)
	mockInv.AssertExpectations(t)
}

// TestFetchItemTitle_InstanceFetchFails verifies error when instance lookup fails.
func TestFetchItemTitle_InstanceFetchFails(t *testing.T) {
	tc := testutil.NewTenantConfig()
	h := NewBaseHandler(zap.NewNop(), tc)

	item := testutil.DefaultItem()
	holdings := &models.Holdings{ID: "holdings-001", InstanceID: "instance-001"}

	mockInv := &MockInventoryClient{}
	mockInv.On("GetItemByBarcode", mock.Anything, mock.Anything, item.Barcode).Return(item, nil)
	mockInv.On("GetHoldingsByID", mock.Anything, mock.Anything, item.HoldingsRecordID).Return(holdings, nil)
	mockInv.On("GetInstanceByID", mock.Anything, mock.Anything, holdings.InstanceID).
		Return(nil, fmt.Errorf("instance unavailable"))

	title, err := h.fetchItemTitle(context.Background(), mockInv, "test-token", item.Barcode)
	assert.Error(t, err)
	assert.Empty(t, title)
	mockInv.AssertExpectations(t)
}
