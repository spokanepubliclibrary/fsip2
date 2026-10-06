package handlers

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/spokanepubliclibrary/fsip2/internal/config"
	"github.com/spokanepubliclibrary/fsip2/internal/folio/models"
	"github.com/spokanepubliclibrary/fsip2/internal/sip2/parser"
	"github.com/spokanepubliclibrary/fsip2/internal/types"
	"github.com/spokanepubliclibrary/fsip2/tests/testutil"
)

// These are end-to-end regression tests for the hand-rolled response builders in
// fee_paid.go, sc_status.go, item_status_update.go, end_session.go and base.go.
// Those builders used to raw-format every variable field with fmt.Sprintf("|XX%s", v),
// bypassing the protocol layer entirely, so two classes of bad FOLIO/config data
// corrupted the wire message:
//
//  1. control characters (the real reported case was a call number
//     "741.5 KHAN\n741.5 KHAN") break SIP2 message framing and crash clients;
//  2. a literal field delimiter inside a value injects a spurious field separator,
//     silently shifting every following field for the SC.
//
// Each test drives the real handler path (Handle -> build*Response ->
// protocol.BuildPrefixedField) with realistic high-risk values and asserts on the
// fully built response.

// assertNoStrayControlCharsRawBuilders asserts the response carries no CR or LF
// anywhere except a single legitimate trailing message terminator. This is the
// actual client-crash condition. Pass messageDelimiter == "" for the handlers that
// do not call builder.Build() (item_status_update, end_session): their terminator
// is appended later by server/connection.go sendMessage(), so their response must
// contain no CR/LF at all.
func assertNoStrayControlCharsRawBuilders(t *testing.T, resp, messageDelimiter string) {
	t.Helper()

	body := resp
	if messageDelimiter != "" {
		require.True(t, strings.HasSuffix(resp, messageDelimiter),
			"response must end with the configured message terminator")
		body = strings.TrimSuffix(resp, messageDelimiter)
	}

	assert.NotContains(t, body, "\n", "response body must contain no LF (breaks SIP2 framing)")
	assert.NotContains(t, body, "\r", "response body must contain no CR (breaks SIP2 framing)")
}

// rawBuilderDelimiterCount counts structural field separators in a built response.
// Comparing this against an equivalent clean-data response is the structural proof
// that a value containing a literal "|" injected no extra field.
func rawBuilderDelimiterCount(resp string) int {
	return strings.Count(resp, "|")
}

// TestFeePaidHandler_DirtyFolioData_EndToEnd drives the Fee Paid (37 -> 38) success
// path with a FOLIO fee/fine ID containing an embedded newline and an SC-supplied
// transaction ID containing a literal field delimiter.
func TestFeePaidHandler_DirtyFolioData_EndToEnd(t *testing.T) {
	const (
		dirtyFeeFineID = "ff-001\nduplicate-line"
		cleanFeeFineID = "ff-001 duplicate-line"
		dirtyTxnID     = "TXN-9|AOEVIL-INST"
		cleanTxnID     = "TXN-9 AOEVIL-INST"
	)

	run := func(t *testing.T, feeFineID, txnID string) string {
		t.Helper()

		tc := testutil.NewTenantConfig()
		sess := testutil.NewAuthSession(tc, testutil.WithSessionUser("testuser", "", ""))
		user := makeTestUser()

		account := &models.Account{
			ID:        "acc-001",
			FeeFineID: feeFineID,
			Remaining: models.FlexibleFloat(10.00),
		}

		mockPatron := &MockPatronClient{}
		mockFees := &MockFeesClient{}
		mockPatron.On("GetUserByBarcode", mock.Anything, mock.Anything, user.Barcode).Return(user, nil)
		mockFees.On("GetEligibleAccountByID", mock.Anything, mock.Anything, "acc-001").Return(account, nil)
		mockFees.On("PayAccount", mock.Anything, mock.Anything, "acc-001", mock.Anything).
			Return(&models.PaymentResponse{RemainingAmount: "0.00"}, nil)

		h := NewFeePaidHandler(zap.NewNop(), tc)
		injectMocks(h.BaseHandler, mockPatron, nil, nil, mockFees)

		msg := feePaidMsg(user.Barcode, "10.00", map[parser.FieldCode]string{
			parser.FeeIdentifier: "acc-001",
			parser.TransactionID: txnID,
		})

		resp, err := h.Handle(context.Background(), msg, sess)
		require.NoError(t, err)
		require.True(t, strings.HasPrefix(resp, "38"))
		return resp
	}

	dirty := run(t, dirtyFeeFineID, dirtyTxnID)
	clean := run(t, cleanFeeFineID, cleanTxnID)

	// 1. No stray control characters: only the trailing "\r" terminator.
	assertNoStrayControlCharsRawBuilders(t, dirty, "\r")

	// 2. The embedded newline in the FOLIO fee/fine ID comes out space-joined.
	fe, ok := extractSIP2FieldValue(dirty, "|", "FE")
	require.True(t, ok, "FE (fee/fine identifier) field must be present")
	assert.Equal(t, cleanFeeFineID, fe, "embedded LF must be space-joined, not emitted raw")

	// 3. The literal "|" in the transaction ID injected no extra field.
	assert.Equal(t, rawBuilderDelimiterCount(clean), rawBuilderDelimiterCount(dirty),
		"a value containing a literal field delimiter must not change the field count")
	bk, ok := extractSIP2FieldValue(dirty, "|", "BK")
	require.True(t, ok, "BK (transaction ID) field must be present")
	assert.Equal(t, cleanTxnID, bk, "embedded delimiter must be neutralized to a space")
	assert.NotContains(t, dirty, "|AOEVIL-INST", "injected delimiter must not forge an AO field")

	// Numeric sites (FG/FA %.2f) are deliberately still raw — confirm they are intact.
	assert.Contains(t, dirty, "|FG10.00", "FG amount applied must still be emitted")
}

// TestSCStatusHandler_DirtyConfigData_EndToEnd drives the SC Status (99 -> 98) path
// with an admin-configured library name containing an embedded newline and a
// terminal location (CP from login) containing a literal field delimiter.
func TestSCStatusHandler_DirtyConfigData_EndToEnd(t *testing.T) {
	const (
		dirtyLibraryName = "Spokane Public Library\nDowntown"
		cleanLibraryName = "Spokane Public Library Downtown"
		dirtyTerminal    = "DESK-01|AMFORGED"
		cleanTerminal    = "DESK-01 AMFORGED"
	)

	run := func(t *testing.T, libraryName, terminalLocation string) string {
		t.Helper()

		tc := &config.TenantConfig{
			Tenant:                "test-library",
			InstitutionID:         "SPL",
			LibraryName:           libraryName,
			MessageDelimiter:      "\r",
			FieldDelimiter:        "|",
			Charset:               "UTF-8",
			ErrorDetectionEnabled: true,
		}

		h := NewSCStatusHandler(zap.NewNop(), tc)
		sess := types.NewSession("test-session", tc)
		sess.SetLocationCode(terminalLocation)

		msg := &parser.Message{
			Code:           parser.SCStatus,
			Fields:         make(map[string]string),
			SequenceNumber: "3",
		}

		resp, err := h.Handle(context.Background(), msg, sess)
		require.NoError(t, err)
		require.True(t, strings.HasPrefix(resp, "98"))
		return resp
	}

	dirty := run(t, dirtyLibraryName, dirtyTerminal)
	clean := run(t, cleanLibraryName, cleanTerminal)

	// 1. No stray control characters: only the trailing "\r" terminator.
	assertNoStrayControlCharsRawBuilders(t, dirty, "\r")

	// 2. The embedded newline in LibraryName comes out space-joined.
	am, ok := extractSIP2FieldValue(dirty, "|", "AM")
	require.True(t, ok, "AM (library name) field must be present")
	assert.Equal(t, cleanLibraryName, am, "embedded LF must be space-joined, not emitted raw")

	// 3. The literal "|" in the terminal location injected no extra field.
	assert.Equal(t, rawBuilderDelimiterCount(clean), rawBuilderDelimiterCount(dirty),
		"a value containing a literal field delimiter must not change the field count")
	an, ok := extractSIP2FieldValue(dirty, "|", "AN")
	require.True(t, ok, "AN (terminal location) field must be present")
	assert.Equal(t, cleanTerminal, an, "embedded delimiter must be neutralized to a space")
	assert.NotContains(t, dirty, "|AMFORGED", "injected delimiter must not forge a second AM field")

	// AO is unaffected and still identifies the ACS.
	assert.Contains(t, dirty, "|AOSPL")
}

// TestItemStatusUpdateHandler_DirtyData_EndToEnd drives the Item Status Update
// (19 -> 20) path with an item identifier carrying both an embedded newline and a
// literal field delimiter. This handler does not call builder.Build(), so it emits
// no AY/AZ and no terminator (that is appended later by server/connection.go
// sendMessage()); the response must therefore contain no CR/LF at all.
func TestItemStatusUpdateHandler_DirtyData_EndToEnd(t *testing.T) {
	const (
		dirtyBarcode = "ITEM-55|AOFORGED\nsecond-line"
		cleanBarcode = "ITEM-55 AOFORGED second-line"
	)

	run := func(t *testing.T, barcode string) string {
		t.Helper()

		tc := testutil.NewTenantConfig()
		sess := testutil.NewAuthSession(tc)

		mockInv := &MockInventoryClient{}
		mockInv.On("GetItemByBarcode", mock.Anything, mock.Anything, barcode).
			Return(testutil.DefaultItem(), nil)

		h := NewItemStatusUpdateHandler(zap.NewNop(), tc)
		injectMocks(h.BaseHandler, nil, nil, mockInv, nil)

		msg := buildTestMsg(parser.ItemStatusUpdateRequest, map[parser.FieldCode]string{
			parser.InstitutionID:  "TEST-INST",
			parser.ItemIdentifier: barcode,
		})

		resp, err := h.Handle(context.Background(), msg, sess)
		require.NoError(t, err)
		require.True(t, strings.HasPrefix(resp, "20"))
		mockInv.AssertExpectations(t)
		return resp
	}

	dirty := run(t, dirtyBarcode)
	clean := run(t, cleanBarcode)

	// 1. No control characters at all — no terminator is added by this handler.
	assertNoStrayControlCharsRawBuilders(t, dirty, "")

	// 2. The embedded newline comes out space-joined (AB and the AJ title echo).
	ab, ok := extractSIP2FieldValue(dirty, "|", "AB")
	require.True(t, ok, "AB (item identifier) field must be present")
	assert.Equal(t, cleanBarcode, ab, "embedded LF must be space-joined, not emitted raw")
	aj, ok := extractSIP2FieldValue(dirty, "|", "AJ")
	require.True(t, ok, "AJ (title identifier) field must be present")
	assert.Equal(t, cleanBarcode, aj)

	// 3. The literal "|" injected no extra field.
	assert.Equal(t, rawBuilderDelimiterCount(clean), rawBuilderDelimiterCount(dirty),
		"a value containing a literal field delimiter must not change the field count")
	assert.NotContains(t, dirty, "|AOFORGED", "injected delimiter must not forge a second AO field")
	assert.Contains(t, dirty, "|AOTEST-INST")
	assert.Contains(t, dirty, "|AFItem properties updated")
}

// TestEndSessionHandler_DirtyData_EndToEnd drives the End Patron Session
// (35 -> 36) path with a patron identifier carrying an embedded newline and an
// institution ID carrying a literal field delimiter. Like item_status_update, this
// handler emits no AY/AZ and no terminator.
func TestEndSessionHandler_DirtyData_EndToEnd(t *testing.T) {
	const (
		dirtyPatronID = "P-TEST-001\nP-TEST-002"
		cleanPatronID = "P-TEST-001 P-TEST-002"
		dirtyInstID   = "TEST-INST|AAFORGED"
		cleanInstID   = "TEST-INST AAFORGED"
	)

	run := func(t *testing.T, patronID, instID string) string {
		t.Helper()

		tc := testutil.NewTenantConfig()
		sess := testutil.NewAuthSession(tc)

		h := NewEndSessionHandler(zap.NewNop(), tc)

		msg := buildTestMsg(parser.EndPatronSessionRequest, map[parser.FieldCode]string{
			parser.InstitutionID:    instID,
			parser.PatronIdentifier: patronID,
		})

		resp, err := h.Handle(context.Background(), msg, sess)
		require.NoError(t, err)
		require.True(t, strings.HasPrefix(resp, "36Y"))
		return resp
	}

	dirty := run(t, dirtyPatronID, dirtyInstID)
	clean := run(t, cleanPatronID, cleanInstID)

	// 1. No control characters at all — no terminator is added by this handler.
	assertNoStrayControlCharsRawBuilders(t, dirty, "")

	// 2. The embedded newline in the patron identifier comes out space-joined.
	aa, ok := extractSIP2FieldValue(dirty, "|", "AA")
	require.True(t, ok, "AA (patron identifier) field must be present")
	assert.Equal(t, cleanPatronID, aa, "embedded LF must be space-joined, not emitted raw")

	// 3. The literal "|" in the institution ID injected no extra field.
	assert.Equal(t, rawBuilderDelimiterCount(clean), rawBuilderDelimiterCount(dirty),
		"a value containing a literal field delimiter must not change the field count")
	ao, ok := extractSIP2FieldValue(dirty, "|", "AO")
	require.True(t, ok, "AO (institution ID) field must be present")
	assert.Equal(t, cleanInstID, ao, "embedded delimiter must be neutralized to a space")
	assert.NotContains(t, dirty, "|AAFORGED", "injected delimiter must not forge a second AA field")
	assert.Contains(t, dirty, "|AFSession ended successfully")
}
