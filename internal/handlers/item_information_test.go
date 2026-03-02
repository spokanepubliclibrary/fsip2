package handlers

import (
	"testing"

	"github.com/spokanepubliclibrary/fsip2/internal/config"
	"github.com/spokanepubliclibrary/fsip2/internal/folio/models"
)

func TestIsUUID(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{
			name:  "Valid UUID lowercase",
			input: "550e8400-e29b-41d4-a716-446655440000",
			want:  true,
		},
		{
			name:  "Valid UUID uppercase",
			input: "550E8400-E29B-41D4-A716-446655440000",
			want:  true,
		},
		{
			name:  "Valid UUID mixed case",
			input: "550e8400-E29B-41d4-A716-446655440000",
			want:  true,
		},
		{
			name:  "Invalid - barcode (numbers only)",
			input: "1234567890",
			want:  false,
		},
		{
			name:  "Invalid - barcode (alphanumeric no dashes)",
			input: "ABC123DEF456",
			want:  false,
		},
		{
			name:  "Invalid - too short",
			input: "550e8400-e29b-41d4",
			want:  false,
		},
		{
			name:  "Invalid - wrong format (missing dashes)",
			input: "550e8400e29b41d4a716446655440000",
			want:  false,
		},
		{
			name:  "Invalid - wrong segment lengths",
			input: "550e840-e29b-41d4-a716-446655440000",
			want:  false,
		},
		{
			name:  "Invalid - contains non-hex characters",
			input: "550e8400-e29b-41d4-a716-44665544000g",
			want:  false,
		},
		{
			name:  "Empty string",
			input: "",
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isUUID(tt.input)
			if got != tt.want {
				t.Errorf("isUUID(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestGetPrimaryContributor(t *testing.T) {
	tests := []struct {
		name     string
		instance *models.Instance
		want     string
	}{
		{
			name: "Instance with primary contributor",
			instance: &models.Instance{
				Contributors: []models.Contributor{
					{Name: "Secondary Author", Primary: false},
					{Name: "Primary Author", Primary: true},
					{Name: "Another Author", Primary: false},
				},
			},
			want: "Primary Author",
		},
		{
			name: "Instance with no primary contributor",
			instance: &models.Instance{
				Contributors: []models.Contributor{
					{Name: "Author One", Primary: false},
					{Name: "Author Two", Primary: false},
				},
			},
			want: "",
		},
		{
			name: "Instance with no contributors",
			instance: &models.Instance{
				Contributors: []models.Contributor{},
			},
			want: "",
		},
		{
			name:     "Nil instance",
			instance: nil,
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got string
			if tt.instance != nil {
				got = getPrimaryContributor(tt.instance)
			}
			if got != tt.want {
				t.Errorf("getPrimaryContributor() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGetSummaryNote(t *testing.T) {
	const summaryNoteTypeID = "10e2e11b-450f-45c8-b09b-0f819999966e"
	const otherNoteTypeID = "12345678-1234-1234-1234-123456789012"

	tests := []struct {
		name     string
		instance *models.Instance
		want     string
	}{
		{
			name: "Instance with summary note",
			instance: &models.Instance{
				Notes: []models.Note{
					{NoteTypeID: otherNoteTypeID, Note: "Other note"},
					{NoteTypeID: summaryNoteTypeID, Note: "This is a summary note"},
					{NoteTypeID: otherNoteTypeID, Note: "Another note"},
				},
			},
			want: "This is a summary note",
		},
		{
			name: "Instance with no summary note",
			instance: &models.Instance{
				Notes: []models.Note{
					{NoteTypeID: otherNoteTypeID, Note: "Other note"},
				},
			},
			want: "",
		},
		{
			name: "Instance with no notes",
			instance: &models.Instance{
				Notes: []models.Note{},
			},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getSummaryNote(tt.instance)
			if got != tt.want {
				t.Errorf("getSummaryNote() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGetISBNs(t *testing.T) {
	const isbnTypeID = "8261054f-be78-422d-bd51-4ed9f33c3422"
	const otherTypeID = "12345678-1234-1234-1234-123456789012"

	tests := []struct {
		name     string
		instance *models.Instance
		want     []string
	}{
		{
			name: "Instance with multiple ISBNs",
			instance: &models.Instance{
				Identifiers: []models.Identifier{
					{IdentifierTypeID: isbnTypeID, Value: "9780062871589"},
					{IdentifierTypeID: otherTypeID, Value: "12345"},
					{IdentifierTypeID: isbnTypeID, Value: "0062871587"},
				},
			},
			want: []string{"9780062871589", "0062871587"},
		},
		{
			name: "Instance with no ISBNs",
			instance: &models.Instance{
				Identifiers: []models.Identifier{
					{IdentifierTypeID: otherTypeID, Value: "12345"},
				},
			},
			want: []string{},
		},
		{
			name: "Instance with no identifiers",
			instance: &models.Instance{
				Identifiers: []models.Identifier{},
			},
			want: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getISBNs(tt.instance)
			if len(got) != len(tt.want) {
				t.Errorf("getISBNs() returned %d ISBNs, want %d", len(got), len(tt.want))
				return
			}
			for i, isbn := range got {
				if isbn != tt.want[i] {
					t.Errorf("getISBNs()[%d] = %q, want %q", i, isbn, tt.want[i])
				}
			}
		})
	}
}

func TestGetUPCs(t *testing.T) {
	const upcTypeID = "2e8b3b6c-0e7d-4e48-bca2-b0b23b376af5"
	const otherTypeID = "12345678-1234-1234-1234-123456789012"

	tests := []struct {
		name     string
		instance *models.Instance
		want     []string
	}{
		{
			name: "Instance with UPCs",
			instance: &models.Instance{
				Identifiers: []models.Identifier{
					{IdentifierTypeID: upcTypeID, Value: "085391173649"},
					{IdentifierTypeID: otherTypeID, Value: "12345"},
					{IdentifierTypeID: upcTypeID, Value: "123456789012"},
				},
			},
			want: []string{"085391173649", "123456789012"},
		},
		{
			name: "Instance with no UPCs",
			instance: &models.Instance{
				Identifiers: []models.Identifier{
					{IdentifierTypeID: otherTypeID, Value: "12345"},
				},
			},
			want: []string{},
		},
		{
			name: "Instance with no identifiers",
			instance: &models.Instance{
				Identifiers: []models.Identifier{},
			},
			want: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getUPCs(tt.instance)
			if len(got) != len(tt.want) {
				t.Errorf("getUPCs() returned %d UPCs, want %d", len(got), len(tt.want))
				return
			}
			for i, upc := range got {
				if upc != tt.want[i] {
					t.Errorf("getUPCs()[%d] = %q, want %q", i, upc, tt.want[i])
				}
			}
		})
	}
}

func TestCirculationStatusMapping(t *testing.T) {
	tests := []struct {
		name        string
		folioStatus string
		want        string
	}{
		{
			name:        "Available status",
			folioStatus: "Available",
			want:        "03",
		},
		{
			name:        "Checked out status",
			folioStatus: "Checked out",
			want:        "04",
		},
		{
			name:        "In process status",
			folioStatus: "In process",
			want:        "06",
		},
		{
			name:        "Awaiting pickup status",
			folioStatus: "Awaiting pickup",
			want:        "08",
		},
		{
			name:        "In transit status",
			folioStatus: "In transit",
			want:        "10",
		},
		{
			name:        "Claimed returned status",
			folioStatus: "Claimed returned",
			want:        "11",
		},
		{
			name:        "Lost and paid status",
			folioStatus: "Lost and paid",
			want:        "12",
		},
		{
			name:        "Aged to lost status",
			folioStatus: "Aged to lost",
			want:        "12",
		},
		{
			name:        "Declared lost status",
			folioStatus: "Declared lost",
			want:        "12",
		},
		{
			name:        "Missing status",
			folioStatus: "Missing",
			want:        "13",
		},
		{
			name:        "Withdrawn status",
			folioStatus: "Withdrawn",
			want:        "01",
		},
		{
			name:        "On order status",
			folioStatus: "On order",
			want:        "02",
		},
		{
			name:        "Paged status",
			folioStatus: "Paged",
			want:        "08",
		},
		{
			name:        "Unknown status defaults to Other",
			folioStatus: "Some Unknown Status",
			want:        "01",
		},
	}

	// Create a test tenant config with default mappings (empty)
	tenantConfig := &config.TenantConfig{
		CirculationStatusMapping: map[string]string{},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tenantConfig.MapCirculationStatus(tt.folioStatus)
			if got != tt.want {
				t.Errorf("MapCirculationStatus(%q) = %v, want %v", tt.folioStatus, got, tt.want)
			}
		})
	}
}
