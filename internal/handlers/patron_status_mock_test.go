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

// TestPatronStatusHandle_ValidPatron_NoBlocks_NoFees verifies a successful lookup
// returns |BLY with no fee field and all-space patron status.
func TestPatronStatusHandle_ValidPatron_NoBlocks_NoFees(t *testing.T) {
	tc := testutil.NewTenantConfig()
	sess := testutil.NewAuthSession(tc)
	user := makeTestUser()

	mockPatron := &MockPatronClient{}
	mockFees := &MockFeesClient{}

	mockPatron.On("GetUserByBarcode", mock.Anything, mock.Anything, user.Barcode).Return(user, nil)
	mockPatron.On("GetManualBlocks", mock.Anything, mock.Anything, user.ID).
		Return(&models.ManualBlockCollection{}, nil)
	mockPatron.On("GetAutomatedPatronBlocks", mock.Anything, mock.Anything, user.ID).
		Return(&models.AutomatedPatronBlock{}, nil)
	mockFees.On("GetOpenAccountsExcludingSuspended", mock.Anything, mock.Anything, user.ID).
		Return(&models.AccountCollection{}, nil)

	h := NewPatronStatusHandler(zap.NewNop(), tc)
	injectMocks(h.BaseHandler, mockPatron, nil, nil, mockFees)

	msg := buildTestMsg(parser.PatronStatusRequest, map[parser.FieldCode]string{
		parser.InstitutionID:    "TEST-INST",
		parser.PatronIdentifier: user.Barcode,
	})

	resp, err := h.Handle(context.Background(), msg, sess)

	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(resp, "24"), "response must start with 24")
	assert.Contains(t, resp, "|BLY", "valid patron flag required")
	assert.Contains(t, resp, "|AOTEST-INST", "institution ID in response")
	assert.Contains(t, resp, "|BV0.00", "zero balance still reports BV0.00")
	// All-spaces patron status (no blocks): position 2-15 should all be spaces
	assert.Equal(t, byte(' '), resp[2], "charge privileges bit should be space (no block)")

	mockPatron.AssertExpectations(t)
	mockFees.AssertExpectations(t)
}

// TestPatronStatusHandle_ValidPatron_WithManualBorrowingBlock verifies that a manual block
// with Borrowing=true sets bit 0 of the patron status to 'Y'.
func TestPatronStatusHandle_ValidPatron_WithManualBorrowingBlock(t *testing.T) {
	tc := testutil.NewTenantConfig()
	sess := testutil.NewAuthSession(tc)
	user := makeTestUser()

	mockPatron := &MockPatronClient{}
	mockFees := &MockFeesClient{}

	blocks := &models.ManualBlockCollection{
		ManualBlocks: []models.ManualBlock{
			{Borrowing: true},
		},
	}
	mockPatron.On("GetUserByBarcode", mock.Anything, mock.Anything, user.Barcode).Return(user, nil)
	mockPatron.On("GetManualBlocks", mock.Anything, mock.Anything, user.ID).Return(blocks, nil)
	mockPatron.On("GetAutomatedPatronBlocks", mock.Anything, mock.Anything, user.ID).
		Return(&models.AutomatedPatronBlock{}, nil)
	mockFees.On("GetOpenAccountsExcludingSuspended", mock.Anything, mock.Anything, user.ID).
		Return(&models.AccountCollection{}, nil)

	h := NewPatronStatusHandler(zap.NewNop(), tc)
	injectMocks(h.BaseHandler, mockPatron, nil, nil, mockFees)

	msg := buildTestMsg(parser.PatronStatusRequest, map[parser.FieldCode]string{
		parser.InstitutionID:    "TEST-INST",
		parser.PatronIdentifier: user.Barcode,
	})

	resp, err := h.Handle(context.Background(), msg, sess)

	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(resp, "24"))
	assert.Equal(t, byte('Y'), resp[2], "charge privileges bit should be Y (manual borrowing block)")
	assert.Contains(t, resp, "|BLY", "patron is still valid despite block")

	mockPatron.AssertExpectations(t)
}

// TestPatronStatusHandle_ValidPatron_WithFees verifies that outstanding accounts
// produce a |BV field with the total balance.
func TestPatronStatusHandle_ValidPatron_WithFees(t *testing.T) {
	tc := testutil.NewTenantConfig()
	sess := testutil.NewAuthSession(tc)
	user := makeTestUser()

	mockPatron := &MockPatronClient{}
	mockFees := &MockFeesClient{}

	accounts := &models.AccountCollection{
		Accounts: []models.Account{
			{ID: "acc-1", Remaining: models.FlexibleFloat(10.50)},
			{ID: "acc-2", Remaining: models.FlexibleFloat(5.00)},
		},
	}
	mockPatron.On("GetUserByBarcode", mock.Anything, mock.Anything, user.Barcode).Return(user, nil)
	mockPatron.On("GetManualBlocks", mock.Anything, mock.Anything, user.ID).
		Return(&models.ManualBlockCollection{}, nil)
	mockPatron.On("GetAutomatedPatronBlocks", mock.Anything, mock.Anything, user.ID).
		Return(&models.AutomatedPatronBlock{}, nil)
	mockFees.On("GetOpenAccountsExcludingSuspended", mock.Anything, mock.Anything, user.ID).
		Return(accounts, nil)

	h := NewPatronStatusHandler(zap.NewNop(), tc)
	injectMocks(h.BaseHandler, mockPatron, nil, nil, mockFees)

	msg := buildTestMsg(parser.PatronStatusRequest, map[parser.FieldCode]string{
		parser.InstitutionID:    "TEST-INST",
		parser.PatronIdentifier: user.Barcode,
	})

	resp, err := h.Handle(context.Background(), msg, sess)

	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(resp, "24"))
	assert.Contains(t, resp, "|BLY")
	assert.Contains(t, resp, "|BV15.50", "total outstanding balance in BV field")

	mockPatron.AssertExpectations(t)
	mockFees.AssertExpectations(t)
}

// TestPatronStatusHandle_UserNotFound verifies that when GetUserByBarcode fails
// the response includes |BLN (invalid patron flag).
func TestPatronStatusHandle_UserNotFound(t *testing.T) {
	tc := testutil.NewTenantConfig()
	sess := testutil.NewAuthSession(tc)

	mockPatron := &MockPatronClient{}

	mockPatron.On("GetUserByBarcode", mock.Anything, mock.Anything, "P999").
		Return(nil, assert.AnError)

	h := NewPatronStatusHandler(zap.NewNop(), tc)
	injectMocks(h.BaseHandler, mockPatron, nil, nil, nil)

	msg := buildTestMsg(parser.PatronStatusRequest, map[parser.FieldCode]string{
		parser.InstitutionID:    "TEST-INST",
		parser.PatronIdentifier: "P999",
	})

	resp, err := h.Handle(context.Background(), msg, sess)

	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(resp, "24"))
	assert.Contains(t, resp, "|BLN", "invalid patron flag required for unfound user")
	assert.NotContains(t, resp, "|BLY")

	mockPatron.AssertExpectations(t)
}

// TestPatronStatusHandle_UserInactive verifies that an inactive account returns |BLN.
func TestPatronStatusHandle_UserInactive(t *testing.T) {
	tc := testutil.NewTenantConfig()
	sess := testutil.NewAuthSession(tc)
	user := makeTestUser()
	user.Active = false

	mockPatron := &MockPatronClient{}

	mockPatron.On("GetUserByBarcode", mock.Anything, mock.Anything, user.Barcode).Return(user, nil)

	h := NewPatronStatusHandler(zap.NewNop(), tc)
	injectMocks(h.BaseHandler, mockPatron, nil, nil, nil)

	msg := buildTestMsg(parser.PatronStatusRequest, map[parser.FieldCode]string{
		parser.InstitutionID:    "TEST-INST",
		parser.PatronIdentifier: user.Barcode,
	})

	resp, err := h.Handle(context.Background(), msg, sess)

	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(resp, "24"))
	assert.Contains(t, resp, "|BLN", "inactive patron must return invalid patron flag")

	mockPatron.AssertExpectations(t)
}

// TestPatronStatusHandle_AutomatedBlock verifies that an automated block with
// BlockBorrowing=true sets bit 0 of the patron status to 'Y'.
func TestPatronStatusHandle_AutomatedBlock(t *testing.T) {
	tc := testutil.NewTenantConfig()
	sess := testutil.NewAuthSession(tc)
	user := makeTestUser()

	mockPatron := &MockPatronClient{}
	mockFees := &MockFeesClient{}

	automatedBlocks := &models.AutomatedPatronBlock{
		AutomatedPatronBlocks: []models.AutomatedBlock{
			{BlockBorrowing: true},
		},
	}
	mockPatron.On("GetUserByBarcode", mock.Anything, mock.Anything, user.Barcode).Return(user, nil)
	mockPatron.On("GetManualBlocks", mock.Anything, mock.Anything, user.ID).
		Return(&models.ManualBlockCollection{}, nil)
	mockPatron.On("GetAutomatedPatronBlocks", mock.Anything, mock.Anything, user.ID).
		Return(automatedBlocks, nil)
	mockFees.On("GetOpenAccountsExcludingSuspended", mock.Anything, mock.Anything, user.ID).
		Return(&models.AccountCollection{}, nil)

	h := NewPatronStatusHandler(zap.NewNop(), tc)
	injectMocks(h.BaseHandler, mockPatron, nil, nil, mockFees)

	msg := buildTestMsg(parser.PatronStatusRequest, map[parser.FieldCode]string{
		parser.InstitutionID:    "TEST-INST",
		parser.PatronIdentifier: user.Barcode,
	})

	resp, err := h.Handle(context.Background(), msg, sess)

	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(resp, "24"))
	assert.Equal(t, byte('Y'), resp[2], "charge privileges bit should be Y (automated block)")
	assert.Contains(t, resp, "|BLY")

	mockPatron.AssertExpectations(t)
	mockFees.AssertExpectations(t)
}

// ─── Delimiter / control-character injection regression (Patron Status 24) ───────

// TestPatronStatusHandle_DelimiterAndControlChars_EndToEnd is the end-to-end
// regression test for the Patron Status (24) response: a FOLIO patron name
// carrying a raw control character must not break SIP2's line-oriented message
// framing, and one carrying a literal field delimiter must not inject a spurious
// field. It drives the real handler path (Handle ->
// buildPatronStatusResponse -> protocol.BuildPrefixedField ->
// ResponseBuilder.Build) rather than testing the sanitizer in isolation.
func TestPatronStatusHandle_DelimiterAndControlChars_EndToEnd(t *testing.T) {
	tc := testutil.NewTenantConfig()

	// dirty holds the values as FOLIO might actually store them: an embedded
	// newline in the surname and an embedded '|' in the given name.
	dirty := makeTestUser()
	dirty.Personal.LastName = "O'Brien\nJr"
	dirty.Personal.FirstName = "Mary|Anne"

	// clean holds the same values already sanitized; the two responses must have
	// the same number of field delimiters, which is the structural proof that the
	// embedded '|' did not create an extra field.
	clean := makeTestUser()
	clean.Personal.LastName = "O'Brien Jr"
	clean.Personal.FirstName = "Mary Anne"

	run := func(u *models.User) string {
		t.Helper()
		sess := testutil.NewAuthSession(tc)

		mockPatron := &MockPatronClient{}
		mockFees := &MockFeesClient{}
		mockPatron.On("GetUserByBarcode", mock.Anything, mock.Anything, u.Barcode).Return(u, nil)
		mockPatron.On("GetManualBlocks", mock.Anything, mock.Anything, u.ID).
			Return(&models.ManualBlockCollection{}, nil)
		mockPatron.On("GetAutomatedPatronBlocks", mock.Anything, mock.Anything, u.ID).
			Return(&models.AutomatedPatronBlock{}, nil)
		mockFees.On("GetOpenAccountsExcludingSuspended", mock.Anything, mock.Anything, u.ID).
			Return(&models.AccountCollection{}, nil)

		h := NewPatronStatusHandler(zap.NewNop(), tc)
		injectMocks(h.BaseHandler, mockPatron, nil, nil, mockFees)

		msg := buildTestMsg(parser.PatronStatusRequest, map[parser.FieldCode]string{
			parser.InstitutionID:    "TEST-INST",
			parser.PatronIdentifier: u.Barcode,
		})

		resp, err := h.Handle(context.Background(), msg, sess)
		require.NoError(t, err)
		return resp
	}

	dirtyResp := run(dirty)
	cleanResp := run(clean)

	// 1. Framing: no control characters other than the trailing terminator.
	assertPatronFramingClean(t, dirtyResp)

	// 2. The embedded newline is space-joined, and 3. the embedded '|' does not
	//    split AE into two fields (the value survives intact).
	ae, ok := extractSIP2FieldValue(dirtyResp, tc.FieldDelimiter, "AE")
	require.True(t, ok, "AE (personal name) must be present")
	assert.Equal(t, "O'Brien Jr, Mary Anne", ae,
		"embedded newline must become a space and embedded '|' must not split the field")

	// 3. Structural proof: identical delimiter count to the pre-sanitized equivalent.
	assert.Equal(t,
		patronResponseDelimiterCount(cleanResp, tc.FieldDelimiter),
		patronResponseDelimiterCount(dirtyResp, tc.FieldDelimiter),
		"an embedded field delimiter must not add a field boundary to the response")
}
