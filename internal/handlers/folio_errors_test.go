package handlers

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/spokanepubliclibrary/fsip2/internal/folio"
)

func TestExtractFolioErrorMessage_BareHTTPError(t *testing.T) {
	err := &folio.HTTPError{StatusCode: 422, Body: `{"message": "Item not found"}`}
	msg := ExtractFolioErrorMessage(err, "fallback")
	assert.Equal(t, "Item not found", msg)
}

func TestExtractFolioErrorMessage_WrappedHTTPError(t *testing.T) {
	httpErr := &folio.HTTPError{StatusCode: 422, Body: `{"errors": [{"message": "Validation failed"}]}`}
	wrapped := fmt.Errorf("checkout failed: %w", httpErr)
	msg := ExtractFolioErrorMessage(wrapped, "fallback")
	assert.Equal(t, "Validation failed", msg)
}

func TestExtractFolioErrorMessage_NonHTTPError(t *testing.T) {
	msg := ExtractFolioErrorMessage(assert.AnError, "fallback")
	assert.Equal(t, "fallback", msg)
}

func TestExtractFolioErrorMessage_EmptyBodyHTTPError(t *testing.T) {
	err := &folio.HTTPError{StatusCode: 500, Body: ""}
	msg := ExtractFolioErrorMessage(err, "fallback")
	assert.Equal(t, "Unknown error", msg)
}

func TestExtractFolioErrorMessage_NilError(t *testing.T) {
	msg := ExtractFolioErrorMessage(nil, "fallback")
	assert.Equal(t, "fallback", msg)
}
