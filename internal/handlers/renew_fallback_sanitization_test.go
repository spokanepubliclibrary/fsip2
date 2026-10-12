package handlers

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/spokanepubliclibrary/fsip2/internal/config"
	"github.com/spokanepubliclibrary/fsip2/internal/sip2/parser"
	"github.com/spokanepubliclibrary/fsip2/tests/testutil"
)

// End-to-end regression tests for the three last-resort fallback responses in
// renew.go and renew_all.go: the branches taken when the ResponseBuilder itself
// returns an error. They were the last raw field writes in the repository, and
// they were the easiest to miss — their format strings do not begin with the
// field delimiter, so the field codes sit mid-string after the fixed-length
// header and a grep anchored on `"|` never saw them.
//
// They are also the worst place for a raw write: the values are client-supplied
// identifiers plus wrapped error text, i.e. the least predictable data the server
// handles. Each test asserts the three properties that matter on the wire:
//
//  1. no CR/LF anywhere (these fallbacks do not append the message terminator —
//     server/connection.go sendMessage does — so the body must be entirely free
//     of control characters);
//  2. an embedded newline comes out space-joined rather than raw;
//  3. a value containing a literal field delimiter injects NO extra field, proven
//     structurally by comparing the separator count against an equivalent
//     clean-data response rather than by substring absence.
//
// The fixed-length header and its positional timestamp must survive untouched,
// so each test also pins the header bytes.

// renewFallbackTenantConfig returns a tenant config that forces
// ResponseBuilder.Build to fail, which is the only way into these fallback
// branches. Build looks up the charset encoder when error detection is enabled,
// and an unsupported charset makes that lookup return an error.
func renewFallbackTenantConfig(t *testing.T) *config.TenantConfig {
	t.Helper()

	tc := testutil.NewTenantConfig()
	require.True(t, tc.ErrorDetectionEnabled,
		"error detection must be on, otherwise Build never reaches the encoder lookup")
	tc.Charset = "NO-SUCH-CHARSET-FORCES-BUILDER-ERROR"
	return tc
}

// assertRenewFallbackShape pins the parts of the response that are positional
// rather than delimiter-framed: the message code, the status flags and the
// generated timestamp (whose format carries deliberate internal spaces). It also
// confirms the fallback path really was taken — the ResponseBuilder appends AY/AZ,
// and these hand-built responses do not.
func assertRenewFallbackShape(t *testing.T, resp, codeAndFlags string, timestampLen int) {
	t.Helper()

	require.True(t, strings.HasPrefix(resp, codeAndFlags),
		"fallback response must keep its fixed-length header verbatim, got %q", resp)
	assert.NotContains(t, resp, "|AY", "AY would mean the ResponseBuilder succeeded, not the fallback")
	assert.NotContains(t, resp, "|AZ", "AZ would mean the ResponseBuilder succeeded, not the fallback")

	header, _, _ := strings.Cut(resp, "|")
	assert.Len(t, header, len(codeAndFlags)+timestampLen,
		"header + timestamp must be exactly the documented fixed length (no sanitizer may touch it)")
}

// TestRenewHandler_FallbackErrorResponse_DirtyData_EndToEnd drives
// buildRenewResponseWithError's fallback (renew.go), which carries four
// data-carrying fields: AO, AA, AB and AF (a wrapped error message).
func TestRenewHandler_FallbackErrorResponse_DirtyData_EndToEnd(t *testing.T) {
	const (
		dirtyPatron = "P123|AOFORGED-INST"
		cleanPatron = "P123 AOFORGED-INST"
		dirtyItem   = "ITEM-1|ABFORGED-ITEM"
		cleanItem   = "ITEM-1 ABFORGED-ITEM"
		dirtyError  = "renew failed: item 741.5 KHAN\n741.5 KHAN is not renewable"
		cleanError  = "renew failed: item 741.5 KHAN 741.5 KHAN is not renewable"
	)

	run := func(t *testing.T, patron, item, errorMessage string) string {
		t.Helper()

		tc := renewFallbackTenantConfig(t)
		h := NewRenewHandler(zap.NewNop(), tc)
		msg := &parser.Message{
			Code:           parser.RenewRequest,
			SequenceNumber: "3",
			Fields:         map[string]string{},
		}

		return h.buildRenewResponseWithError(false, "TEST-INST", patron, item, errorMessage, msg, tc)
	}

	dirty := run(t, dirtyPatron, dirtyItem, dirtyError)
	clean := run(t, cleanPatron, cleanItem, cleanError)

	// "30NN  " plus an 18-character SIP2 date/time stamp ("YYYYMMDD    HHMMSS").
	assertRenewFallbackShape(t, dirty, "30NN  ", 18)

	// 1. No control characters survive anywhere in the message.
	assertNoStrayControlCharsRawBuilders(t, dirty, "")

	// 2. The embedded newline in the wrapped error text comes out space-joined.
	af, ok := extractSIP2FieldValue(dirty, "|", "AF")
	require.True(t, ok, "AF (screen message) field must be present")
	assert.Equal(t, cleanError, af, "embedded LF must be space-joined, not emitted raw")

	// 3. The literal "|" in the patron and item identifiers injected no extra field.
	assert.Equal(t, rawBuilderDelimiterCount(clean), rawBuilderDelimiterCount(dirty),
		"a value containing a literal field delimiter must not change the field count")

	aa, ok := extractSIP2FieldValue(dirty, "|", "AA")
	require.True(t, ok, "AA (patron identifier) field must be present")
	assert.Equal(t, cleanPatron, aa, "embedded delimiter must be neutralized to a space")

	ab, ok := extractSIP2FieldValue(dirty, "|", "AB")
	require.True(t, ok, "AB (item identifier) field must be present")
	assert.Equal(t, cleanItem, ab, "embedded delimiter must be neutralized to a space")

	ao, ok := extractSIP2FieldValue(dirty, "|", "AO")
	require.True(t, ok, "AO (institution ID) field must be present")
	assert.Equal(t, "TEST-INST", ao, "clean institution ID must be byte-identical")
}

// TestRenewHandler_FallbackResponse_DirtyData_EndToEnd drives buildRenewResponse's
// fallback (renew.go), the same shape with a constant AF message: AO, AA and AB
// carry data.
func TestRenewHandler_FallbackResponse_DirtyData_EndToEnd(t *testing.T) {
	const (
		dirtyPatron = "P123|AOFORGED-INST"
		cleanPatron = "P123 AOFORGED-INST"
		dirtyItem   = "ITEM-1\nITEM-1"
		cleanItem   = "ITEM-1 ITEM-1"
	)

	run := func(t *testing.T, patron, item string) string {
		t.Helper()

		tc := renewFallbackTenantConfig(t)
		h := NewRenewHandler(zap.NewNop(), tc)
		msg := &parser.Message{
			Code:           parser.RenewRequest,
			SequenceNumber: "7",
			Fields:         map[string]string{},
		}

		return h.buildRenewResponse(false, "TEST-INST", patron, item, time.Time{}, "Some Title", msg, tc)
	}

	dirty := run(t, dirtyPatron, dirtyItem)
	clean := run(t, cleanPatron, cleanItem)

	assertRenewFallbackShape(t, dirty, "30NN  ", 18)
	assertNoStrayControlCharsRawBuilders(t, dirty, "")

	// The embedded newline in the item identifier comes out space-joined.
	ab, ok := extractSIP2FieldValue(dirty, "|", "AB")
	require.True(t, ok, "AB (item identifier) field must be present")
	assert.Equal(t, cleanItem, ab, "embedded LF must be space-joined, not emitted raw")

	// The literal "|" in the patron identifier injected no extra field.
	assert.Equal(t, rawBuilderDelimiterCount(clean), rawBuilderDelimiterCount(dirty),
		"a value containing a literal field delimiter must not change the field count")

	aa, ok := extractSIP2FieldValue(dirty, "|", "AA")
	require.True(t, ok, "AA (patron identifier) field must be present")
	assert.Equal(t, cleanPatron, aa, "embedded delimiter must be neutralized to a space")

	// The constant AF text is unchanged by the conversion.
	af, ok := extractSIP2FieldValue(dirty, "|", "AF")
	require.True(t, ok, "AF (screen message) field must be present")
	assert.Equal(t, "Renewal failed - internal error", af)
}

// TestRenewAllHandler_FallbackErrorResponse_DirtyData_EndToEnd drives
// buildErrorResponseWithMessage's fallback (renew_all.go), the concatenation form
// of the same bug: AO, AA and AF carry data.
func TestRenewAllHandler_FallbackErrorResponse_DirtyData_EndToEnd(t *testing.T) {
	const (
		dirtyPatron = "P123|AOFORGED-INST"
		cleanPatron = "P123 AOFORGED-INST"
		dirtyError  = "renew all failed\nFOLIO returned 500"
		cleanError  = "renew all failed FOLIO returned 500"
	)

	run := func(t *testing.T, patron, errorMessage string) string {
		t.Helper()

		tc := renewFallbackTenantConfig(t)
		sess := testutil.NewAuthSession(tc)
		h := NewRenewAllHandler(zap.NewNop(), tc)
		msg := &parser.Message{
			Code:           parser.RenewAllRequest,
			SequenceNumber: "1",
			Fields: map[string]string{
				string(parser.InstitutionID):    "TEST-INST",
				string(parser.PatronIdentifier): patron,
			},
		}

		return h.buildErrorResponseWithMessage(msg, sess, errorMessage)
	}

	dirty := run(t, dirtyPatron, dirtyError)
	clean := run(t, cleanPatron, cleanError)

	// "66N00000000" plus an 18-character timestamp ("20060102    150405").
	assertRenewFallbackShape(t, dirty, "66N00000000", 18)
	assertNoStrayControlCharsRawBuilders(t, dirty, "")

	af, ok := extractSIP2FieldValue(dirty, "|", "AF")
	require.True(t, ok, "AF (screen message) field must be present")
	assert.Equal(t, cleanError, af, "embedded LF must be space-joined, not emitted raw")

	assert.Equal(t, rawBuilderDelimiterCount(clean), rawBuilderDelimiterCount(dirty),
		"a value containing a literal field delimiter must not change the field count")

	aa, ok := extractSIP2FieldValue(dirty, "|", "AA")
	require.True(t, ok, "AA (patron identifier) field must be present")
	assert.Equal(t, cleanPatron, aa, "embedded delimiter must be neutralized to a space")

	ao, ok := extractSIP2FieldValue(dirty, "|", "AO")
	require.True(t, ok, "AO (institution ID) field must be present")
	assert.Equal(t, "TEST-INST", ao, "clean institution ID must be byte-identical")
}
