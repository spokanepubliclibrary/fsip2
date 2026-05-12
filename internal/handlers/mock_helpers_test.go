package handlers

import (
	"github.com/spokanepubliclibrary/fsip2/internal/folio/models"
)

// availableItemWithCheckinNotes builds a minimal available item that includes
// one or more "Check in" type circulation notes, used to test AG field output.
func availableItemWithCheckinNotes(id, barcode string, notes ...string) *models.Item {
	circulationNotes := make([]models.CirculationNote, 0, len(notes))
	for _, n := range notes {
		circulationNotes = append(circulationNotes, models.CirculationNote{
			NoteType: "Check in",
			Note:     n,
		})
	}
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
		CirculationNotes: circulationNotes,
	}
}

// mockRequester builds a RequestRequester with the given name fields.
// preferredFirstName is accepted for call-site compatibility but ignored —
// RequestRequester has no PreferredFirstName field; preferred-name logic now
// lives at the Handle level via GetUserByID.
func mockRequesterWithPreferredName(lastName, firstName, _ string) *models.RequestRequester {
	return &models.RequestRequester{
		LastName:  lastName,
		FirstName: firstName,
	}
}

// boolPtr is a convenience helper that returns a pointer to a bool literal,
// which is required when setting *bool fields in config.FieldConfiguration.
func boolPtr(b bool) *bool { return &b }

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
