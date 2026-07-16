package handlers

import (
	"errors"

	"github.com/spokanepubliclibrary/fsip2/internal/folio"
)

// ExtractFolioErrorMessage returns an AF-ready (screen message) string for a FOLIO
// API error. If err wraps a *folio.HTTPError (directly or via fmt.Errorf %w), the
// FOLIO API's own error message is returned via HTTPError.ParseErrorMessage();
// otherwise fallback is returned.
func ExtractFolioErrorMessage(err error, fallback string) string {
	var httpErr *folio.HTTPError
	if errors.As(err, &httpErr) {
		if msg := httpErr.ParseErrorMessage(); msg != "" {
			return msg
		}
	}
	return fallback
}
