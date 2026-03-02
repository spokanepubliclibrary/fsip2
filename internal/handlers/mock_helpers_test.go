package handlers

import (
	"github.com/spokanepubliclibrary/fsip2/internal/folio/models"
)

// makeTestUser returns an active patron suitable for handler mock tests.
// Avoids loading fixtures (which are relative to the project root and unavailable
// when tests run from internal/handlers/).
func makeTestUser() *models.User {
	return &models.User{
		ID:          "user-1234-abcd",
		Username:    "testuser",
		Barcode:     "P-TEST-001",
		Active:      true,
		// PatronGroup left empty to avoid GetPatronGroupByID calls in patron_information tests.
		Personal: models.PersonalInfo{
			FirstName: "Test",
			LastName:  "Patron",
			Email:     "testpatron@example.com",
		},
	}
}
