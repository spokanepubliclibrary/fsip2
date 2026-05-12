package handlers

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/spokanepubliclibrary/fsip2/internal/config"
	"github.com/spokanepubliclibrary/fsip2/internal/folio/models"
	"github.com/spokanepubliclibrary/fsip2/internal/sip2/parser"
	"github.com/spokanepubliclibrary/fsip2/internal/types"
	"go.uber.org/zap"
)

func newItemHandler(tc *config.TenantConfig) *ItemInformationHandler {
	if tc == nil {
		tc = &config.TenantConfig{
			Tenant:           "test-tenant",
			MessageDelimiter: "\r",
			FieldDelimiter:   "|",
			Charset:          "UTF-8",
			OkapiURL:         "http://localhost:9130",
		}
	}
	return NewItemInformationHandler(zap.NewNop(), tc)
}

func newItemSession(tc *config.TenantConfig) *types.Session {
	return types.NewSession("item-test-session", tc)
}

// TestNewItemInformationHandler verifies the constructor creates a valid handler.
func TestNewItemInformationHandler(t *testing.T) {
	tc := &config.TenantConfig{Tenant: "test"}
	h := NewItemInformationHandler(zap.NewNop(), tc)
	if h == nil {
		t.Fatal("NewItemInformationHandler returned nil")
	}
	if h.BaseHandler == nil {
		t.Error("BaseHandler not set")
	}
	if h.logger == nil {
		t.Error("logger not set")
	}
}

// TestPrepareItemResponseData_NilItem covers the invalid/not-found path.
func TestPrepareItemResponseData_NilItem(t *testing.T) {
	h := newItemHandler(nil)

	data := h.prepareItemResponseData(nil, "INST01", "BARCODE123", nil, nil, "", nil, "", "", false)

	if data.circulationStatus != "01" {
		t.Errorf("circulationStatus = %q, want %q", data.circulationStatus, "01")
	}
	if data.institutionID != "INST01" {
		t.Errorf("institutionID = %q, want %q", data.institutionID, "INST01")
	}
	if data.itemID != "BARCODE123" {
		t.Errorf("itemID = %q, want %q", data.itemID, "BARCODE123")
	}
	if len(data.screenMessage) == 0 || !strings.Contains(data.screenMessage[0], "not found") {
		t.Errorf("screenMessage should contain 'not found', got %v", data.screenMessage)
	}
}

// TestPrepareItemResponseData_ValidItem covers the happy path with a complete item.
func TestPrepareItemResponseData_ValidItem(t *testing.T) {
	tc := &config.TenantConfig{
		Tenant:                   "test-tenant",
		CirculationStatusMapping: map[string]string{},
	}
	h := NewItemInformationHandler(zap.NewNop(), tc)

	instance := &models.Instance{
		ID:    "inst-uuid",
		Title: "The Great Gatsby",
		Contributors: []models.Contributor{
			{Name: "F. Scott Fitzgerald", Primary: true},
		},
	}

	item := &models.Item{
		ID:      "item-uuid",
		Barcode: "12345",
		Status:  models.ItemStatus{Name: "Available"},
		Location: &models.Location{
			Name: "Main Library",
		},
		MaterialType: &models.MaterialType{Name: "book"},
		EffectiveCallNumberComponents: models.CallNumberComponents{
			CallNumber: "PS3525.A84 G7",
		},
		Instance: instance,
	}

	data := h.prepareItemResponseData(item, "INST01", "12345", nil, nil, "", nil, "", "", true)

	if data.circulationStatus != "03" { // Available = 03
		t.Errorf("circulationStatus = %q, want %q", data.circulationStatus, "03")
	}
	if data.title != "The Great Gatsby" {
		t.Errorf("title = %q, want %q", data.title, "The Great Gatsby")
	}
	if data.permanentLocation != "Main Library" {
		t.Errorf("permanentLocation = %q, want %q", data.permanentLocation, "Main Library")
	}
	if data.materialType != "book" {
		t.Errorf("materialType = %q, want %q", data.materialType, "book")
	}
	if data.callNumber != "PS3525.A84 G7" {
		t.Errorf("callNumber = %q, want %q", data.callNumber, "PS3525.A84 G7")
	}
	if data.primaryContributor != "F. Scott Fitzgerald" {
		t.Errorf("primaryContributor = %q, want %q", data.primaryContributor, "F. Scott Fitzgerald")
	}
	if len(data.screenMessage) == 0 || data.screenMessage[0] != "Item found" {
		t.Errorf("screenMessage = %v, want ['Item found']", data.screenMessage)
	}
}

// TestPrepareItemResponseData_WithDueDate covers the dueDate path.
func TestPrepareItemResponseData_WithDueDate(t *testing.T) {
	h := newItemHandler(nil)

	due := time.Now().Add(7 * 24 * time.Hour)
	item := &models.Item{
		ID:      "item-uuid",
		Barcode: "12345",
		Status:  models.ItemStatus{Name: "Checked out"},
	}

	data := h.prepareItemResponseData(item, "INST01", "12345", &due, nil, "", nil, "", "", true)

	if data.dueDate == "" {
		t.Error("dueDate should be set when dueDate pointer is provided")
	}
}

// TestPrepareItemResponseData_WithRequests covers the requests/hold-queue path.
func TestPrepareItemResponseData_WithRequests(t *testing.T) {
	h := newItemHandler(nil)

	item := &models.Item{
		ID:      "item-uuid",
		Barcode: "12345",
		Status:  models.ItemStatus{Name: "Available"},
	}

	requests := &models.RequestCollection{
		Requests: []models.Request{
			{ID: "req-1", Status: "Open - Not yet filled"},
			{ID: "req-2", Status: "Open - In transit for pickup"},
			{ID: "req-3", Status: "Closed - Filled"},
		},
	}

	data := h.prepareItemResponseData(item, "INST01", "12345", nil, requests, "", nil, "", "", true)

	// 2 open requests
	if data.holdQueueLength != "0002" {
		t.Errorf("holdQueueLength = %q, want %q", data.holdQueueLength, "0002")
	}
}

// TestPrepareItemResponseData_WithHoldShelf covers hold-shelf and requestor fields.
func TestPrepareItemResponseData_WithHoldShelf(t *testing.T) {
	h := newItemHandler(nil)

	item := &models.Item{
		ID:      "item-uuid",
		Barcode: "12345",
		Status:  models.ItemStatus{Name: "Awaiting pickup"},
	}

	holdExpiry := time.Now().Add(3 * 24 * time.Hour)
	data := h.prepareItemResponseData(item, "INST01", "12345", nil, nil, "Branch A",
		&holdExpiry, "P123456", "Smith, Jane", true)

	if data.routingLocation != "Branch A" {
		t.Errorf("routingLocation = %q, want %q", data.routingLocation, "Branch A")
	}
	if data.holdShelfExpiration == "" {
		t.Error("holdShelfExpiration should be set")
	}
	if data.requestorBarcode != "P123456" {
		t.Errorf("requestorBarcode = %q, want %q", data.requestorBarcode, "P123456")
	}
	if data.requestorName != "Smith, Jane" {
		t.Errorf("requestorName = %q, want %q", data.requestorName, "Smith, Jane")
	}
}

// TestPrepareItemResponseData_WithISBNsAndUPCs covers instance identifiers.
func TestPrepareItemResponseData_WithISBNsAndUPCs(t *testing.T) {
	h := newItemHandler(nil)

	const isbnTypeID = "8261054f-be78-422d-bd51-4ed9f33c3422"
	const upcTypeID = "2e8b3b6c-0e7d-4e48-bca2-b0b23b376af5"
	const summaryNoteTypeID = "10e2e11b-450f-45c8-b09b-0f819999966e"

	instance := &models.Instance{
		Title: "Test Book",
		Identifiers: []models.Identifier{
			{IdentifierTypeID: isbnTypeID, Value: "9780062871589"},
			{IdentifierTypeID: upcTypeID, Value: "085391173649"},
		},
		Notes: []models.Note{
			{NoteTypeID: summaryNoteTypeID, Note: "A summary note"},
		},
	}
	item := &models.Item{
		ID:       "item-uuid",
		Barcode:  "12345",
		Status:   models.ItemStatus{Name: "Available"},
		Instance: instance,
	}

	data := h.prepareItemResponseData(item, "INST01", "12345", nil, nil, "", nil, "", "", true)

	if len(data.isbns) != 1 || data.isbns[0] != "9780062871589" {
		t.Errorf("isbns = %v, want ['9780062871589']", data.isbns)
	}
	if len(data.upcs) != 1 || data.upcs[0] != "085391173649" {
		t.Errorf("upcs = %v, want ['085391173649']", data.upcs)
	}
	if data.workDescription != "A summary note" {
		t.Errorf("workDescription = %q, want %q", data.workDescription, "A summary note")
	}
}

// TestPrepareItemResponseData_NilRequests covers the nil-requests path for holdQueueLength.
func TestPrepareItemResponseData_NilRequests(t *testing.T) {
	h := newItemHandler(nil)

	item := &models.Item{
		ID:      "item-uuid",
		Barcode: "12345",
		Status:  models.ItemStatus{Name: "Available"},
	}

	data := h.prepareItemResponseData(item, "INST01", "12345", nil, nil, "", nil, "", "", true)

	if data.holdQueueLength != "0000" {
		t.Errorf("holdQueueLength = %q, want %q", data.holdQueueLength, "0000")
	}
}

// TestBuildItemInformationResponse_NilItem tests building response for unfound item.
func TestBuildItemInformationResponse_NilItem(t *testing.T) {
	tc := &config.TenantConfig{
		Tenant:           "test-tenant",
		MessageDelimiter: "\r",
		FieldDelimiter:   "|",
		Charset:          "UTF-8",
	}
	h := NewItemInformationHandler(zap.NewNop(), tc)
	session := types.NewSession("test", tc)

	resp := h.buildItemInformationResponse(session, nil, "INST01", "BARCODE123", nil, nil, "", nil, "", "", false)

	if !strings.HasPrefix(resp, "18") {
		t.Errorf("Response should start with '18', got: %s", resp[:5])
	}
}

// TestBuildItemInformationResponse_ValidItem tests building response for a found item.
func TestBuildItemInformationResponse_ValidItem(t *testing.T) {
	tc := &config.TenantConfig{
		Tenant:                   "test-tenant",
		MessageDelimiter:         "\r",
		FieldDelimiter:           "|",
		Charset:                  "UTF-8",
		CirculationStatusMapping: map[string]string{},
	}
	h := NewItemInformationHandler(zap.NewNop(), tc)
	session := types.NewSession("test", tc)

	item := &models.Item{
		ID:      "item-uuid",
		Barcode: "12345",
		Status:  models.ItemStatus{Name: "Available"},
		Instance: &models.Instance{
			Title: "Test Title",
		},
		Location: &models.Location{
			Name: "Main Floor",
		},
	}

	resp := h.buildItemInformationResponse(session, item, "INST01", "12345", nil, nil, "", nil, "", "", true)

	if !strings.HasPrefix(resp, "18") {
		t.Errorf("Response should start with '18', got: %s", resp[:5])
	}
	if !strings.Contains(resp, "INST01") {
		t.Errorf("Response should contain institution ID")
	}
	if !strings.Contains(resp, "12345") {
		t.Errorf("Response should contain item barcode")
	}
}

// TestPrepareInstanceResponseData_NilInstance tests the not-found path for instance.
func TestPrepareInstanceResponseData_NilInstance(t *testing.T) {
	h := newItemHandler(nil)

	data := h.prepareInstanceResponseData(nil, "INST01", "some-uuid", false)

	if data.circulationStatus != "01" {
		t.Errorf("circulationStatus = %q, want %q", data.circulationStatus, "01")
	}
	if len(data.screenMessage) == 0 || !strings.Contains(data.screenMessage[0], "not found") {
		t.Errorf("screenMessage should contain 'not found', got %v", data.screenMessage)
	}
}

// TestPrepareInstanceResponseData_ValidInstance tests the found path.
func TestPrepareInstanceResponseData_ValidInstance(t *testing.T) {
	h := newItemHandler(nil)

	const isbnTypeID = "8261054f-be78-422d-bd51-4ed9f33c3422"
	instance := &models.Instance{
		ID:    "inst-uuid",
		Title: "A Very Long Title That May Or May Not Be Truncated By The Function",
		Contributors: []models.Contributor{
			{Name: "Author One", Primary: true},
		},
		Identifiers: []models.Identifier{
			{IdentifierTypeID: isbnTypeID, Value: "978-0-06-287158-9"},
		},
	}

	data := h.prepareInstanceResponseData(instance, "INST01", "inst-uuid", true)

	if data.institutionID != "INST01" {
		t.Errorf("institutionID = %q, want %q", data.institutionID, "INST01")
	}
	if data.itemID != "inst-uuid" {
		t.Errorf("itemID = %q, want %q", data.itemID, "inst-uuid")
	}
	if data.title == "" {
		t.Error("title should not be empty")
	}
	if data.primaryContributor != "Author One" {
		t.Errorf("primaryContributor = %q, want %q", data.primaryContributor, "Author One")
	}
	if len(data.isbns) != 1 {
		t.Errorf("isbns = %v, want 1 ISBN", data.isbns)
	}
	if len(data.screenMessage) == 0 {
		t.Error("screenMessage should not be empty for valid instance")
	}
}

// TestBuildInstanceInformationResponse_NilInstance tests the not-found path.
func TestBuildInstanceInformationResponse_NilInstance(t *testing.T) {
	tc := &config.TenantConfig{
		Tenant:           "test-tenant",
		MessageDelimiter: "\r",
		FieldDelimiter:   "|",
		Charset:          "UTF-8",
	}
	h := NewItemInformationHandler(zap.NewNop(), tc)
	session := types.NewSession("test", tc)

	resp := h.buildInstanceInformationResponse(session, nil, "INST01", "some-uuid", false)
	if !strings.HasPrefix(resp, "18") {
		t.Errorf("Response should start with '18', got %s", resp[:5])
	}
}

// TestBuildInstanceInformationResponse_ValidInstance tests the found path.
func TestBuildInstanceInformationResponse_ValidInstance(t *testing.T) {
	tc := &config.TenantConfig{
		Tenant:           "test-tenant",
		MessageDelimiter: "\r",
		FieldDelimiter:   "|",
		Charset:          "UTF-8",
	}
	h := NewItemInformationHandler(zap.NewNop(), tc)
	session := types.NewSession("test", tc)

	instance := &models.Instance{
		ID:    "inst-uuid",
		Title: "Bibliographic Title",
	}

	resp := h.buildInstanceInformationResponse(session, instance, "INST01", "inst-uuid", true)
	if !strings.HasPrefix(resp, "18") {
		t.Errorf("Response should start with '18', got %s", resp[:5])
	}
	if !strings.Contains(resp, "INST01") {
		t.Errorf("Response should contain institution ID")
	}
}


// TestItemInformationHandle_MissingItemIdentifier covers the missing-field validation path.
func TestItemInformationHandle_MissingItemIdentifier(t *testing.T) {
	tc := &config.TenantConfig{
		Tenant:           "test-tenant",
		MessageDelimiter: "\r",
		FieldDelimiter:   "|",
	}
	h := NewItemInformationHandler(zap.NewNop(), tc)
	session := types.NewSession("test", tc)

	msg := &parser.Message{
		Code: parser.ItemInformationRequest,
		Fields: map[string]string{
			string(parser.InstitutionID): "INST01",
			// ItemIdentifier is missing
		},
	}

	resp, err := h.Handle(context.Background(), msg, session)
	if resp != "96" {
		t.Errorf("Expected '96' for missing field, got %q", resp)
	}
	if err == nil {
		t.Error("Expected validation error for missing item identifier")
	}
}

// TestItemInformationHandle_NoAuthToken covers the no-auth-token path.
// With no token, getAuthenticatedFolioClient fails → buildItemInformationResponse(nil, ..., false).
func TestItemInformationHandle_NoAuthToken(t *testing.T) {
	tc := &config.TenantConfig{
		Tenant:           "test-tenant",
		MessageDelimiter: "\r",
		FieldDelimiter:   "|",
		Charset:          "UTF-8",
		OkapiURL:         "http://localhost:9130",
	}
	h := NewItemInformationHandler(zap.NewNop(), tc)
	session := types.NewSession("test", tc) // No auth token

	msg := &parser.Message{
		Code: parser.ItemInformationRequest,
		Fields: map[string]string{
			string(parser.InstitutionID):  "INST01",
			string(parser.ItemIdentifier): "BARCODE123",
		},
	}

	resp, err := h.Handle(context.Background(), msg, session)
	if err != nil {
		t.Errorf("Unexpected error from Handle(): %v", err)
	}
	if !strings.HasPrefix(resp, "18") {
		t.Errorf("Expected item information response (18), got: %s", resp[:minInt(5, len(resp))])
	}
}

// TestItemInformationHandle_UUIDAsBarcodeNoAuth tests the UUID detection path with no auth.
func TestItemInformationHandle_UUIDAsBarcodeNoAuth(t *testing.T) {
	tc := &config.TenantConfig{
		Tenant:           "test-tenant",
		MessageDelimiter: "\r",
		FieldDelimiter:   "|",
		Charset:          "UTF-8",
		OkapiURL:         "http://localhost:9130",
	}
	h := NewItemInformationHandler(zap.NewNop(), tc)
	session := types.NewSession("test", tc) // No auth token

	// UUID as item identifier → triggers instance lookup path
	msg := &parser.Message{
		Code: parser.ItemInformationRequest,
		Fields: map[string]string{
			string(parser.InstitutionID):  "INST01",
			string(parser.ItemIdentifier): "550e8400-e29b-41d4-a716-446655440000",
		},
	}

	resp, err := h.Handle(context.Background(), msg, session)
	if err != nil {
		t.Errorf("Unexpected error from Handle(): %v", err)
	}
	if !strings.HasPrefix(resp, "18") {
		t.Errorf("Expected item information response (18), got: %s", resp[:minInt(5, len(resp))])
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// makeAGTestConfig builds a TenantConfig for message "17" with the AG field
// configured according to enabled.
func makeAGTestConfig(agEnabled bool) *config.TenantConfig {
	return &config.TenantConfig{
		Tenant:                   "test-tenant",
		MessageDelimiter:         "\r",
		FieldDelimiter:           "|",
		Charset:                  "UTF-8",
		CirculationStatusMapping: map[string]string{},
		SupportedMessages: []config.MessageSupport{
			{
				Code:    "17",
				Enabled: true,
				Fields: []config.FieldConfiguration{
					{
						Code:    "AG",
						Enabled: agEnabled,
					},
				},
			},
		},
	}
}

// TestBuildItemInformationResponse_AG_EnabledWithNotes verifies that when AG is
// enabled and the item has checkin notes, each note appears as an AG field in
// the serialised SIP2 response.
func TestBuildItemInformationResponse_AG_EnabledWithNotes(t *testing.T) {
	tc := makeAGTestConfig(true)
	h := NewItemInformationHandler(zap.NewNop(), tc)
	session := types.NewSession("ag-test", tc)

	item := availableItemWithCheckinNotes("item-ag-001", "ITEM-AG-001",
		"Handle with care", "Check spine condition")

	resp := h.buildItemInformationResponse(
		session, item, "INST01", "ITEM-AG-001",
		nil, nil, "", nil, "", "", true,
	)

	if !strings.HasPrefix(resp, "18") {
		t.Errorf("Response should start with '18', got: %s", resp[:minInt(5, len(resp))])
	}
	if !strings.Contains(resp, "AGHandle with care") {
		t.Errorf("Response should contain AGHandle with care\nfull response: %s", resp)
	}
	if !strings.Contains(resp, "AGCheck spine condition") {
		t.Errorf("Response should contain AGCheck spine condition\nfull response: %s", resp)
	}
}

// TestBuildItemInformationResponse_AG_EnabledNoNotes verifies that when AG is
// enabled but the item has no checkin notes, no AG field appears in the response.
func TestBuildItemInformationResponse_AG_EnabledNoNotes(t *testing.T) {
	tc := makeAGTestConfig(true)
	h := NewItemInformationHandler(zap.NewNop(), tc)
	session := types.NewSession("ag-test", tc)

	item := availableItemNoHoldings("item-ag-002", "ITEM-AG-002")

	resp := h.buildItemInformationResponse(
		session, item, "INST01", "ITEM-AG-002",
		nil, nil, "", nil, "", "", true,
	)

	if !strings.HasPrefix(resp, "18") {
		t.Errorf("Response should start with '18', got: %s", resp[:minInt(5, len(resp))])
	}
	if strings.Contains(resp, "|AG") {
		t.Errorf("Response should not contain AG field when item has no checkin notes\nfull response: %s", resp)
	}
}

// TestBuildItemInformationResponse_AG_DisabledWithNotes verifies that when AG is
// disabled in config, no AG field appears in the response even if the item has
// checkin notes.
func TestBuildItemInformationResponse_AG_DisabledWithNotes(t *testing.T) {
	tc := makeAGTestConfig(false)
	h := NewItemInformationHandler(zap.NewNop(), tc)
	session := types.NewSession("ag-test", tc)

	item := availableItemWithCheckinNotes("item-ag-003", "ITEM-AG-003", "Handle with care")

	resp := h.buildItemInformationResponse(
		session, item, "INST01", "ITEM-AG-003",
		nil, nil, "", nil, "", "", true,
	)

	if !strings.HasPrefix(resp, "18") {
		t.Errorf("Response should start with '18', got: %s", resp[:minInt(5, len(resp))])
	}
	if strings.Contains(resp, "|AG") {
		t.Errorf("Response should not contain AG field when AG is disabled\nfull response: %s", resp)
	}
}

// TestBuildItemInformationResponse_AG_MultipleNotes verifies that when AG is
// enabled and the item has two checkin notes, both notes appear as separate AG
// fields in the serialised SIP2 response.
func TestBuildItemInformationResponse_AG_MultipleNotes(t *testing.T) {
	tc := makeAGTestConfig(true)
	h := NewItemInformationHandler(zap.NewNop(), tc)
	session := types.NewSession("ag-test", tc)

	item := availableItemWithCheckinNotes("item-ag-004", "ITEM-AG-004",
		"Handle with care", "Check spine condition")

	resp := h.buildItemInformationResponse(
		session, item, "INST01", "ITEM-AG-004",
		nil, nil, "", nil, "", "", true,
	)

	if !strings.HasPrefix(resp, "18") {
		t.Errorf("Response should start with '18', got: %s", resp[:minInt(5, len(resp))])
	}
	if !strings.Contains(resp, "AGHandle with care") {
		t.Errorf("Response should contain AGHandle with care\nfull response: %s", resp)
	}
	if !strings.Contains(resp, "AGCheck spine condition") {
		t.Errorf("Response should contain AGCheck spine condition\nfull response: %s", resp)
	}
}

// TestBuildItemInformationResponse_AG_MixedNoteTypes verifies that when AG is
// enabled and the item has both a "Check in" note and a "Check out" note, only
// the check-in note appears as an AG field — the checkout note must not appear.
func TestBuildItemInformationResponse_AG_MixedNoteTypes(t *testing.T) {
	tc := makeAGTestConfig(true)
	h := NewItemInformationHandler(zap.NewNop(), tc)
	session := types.NewSession("ag-test", tc)

	item := &models.Item{
		ID:      "item-ag-005",
		Barcode: "ITEM-AG-005",
		Status:  models.ItemStatus{Name: "Available"},
		Location: &models.Location{
			ID:   "loc-001",
			Name: "Main Stacks",
		},
		MaterialType: &models.MaterialType{
			ID:   "mt-001",
			Name: "Book",
		},
		CirculationNotes: []models.CirculationNote{
			{NoteType: "Check in", Note: "Inspect for damage"},
			{NoteType: "Check out", Note: "Remind patron about DVD"},
		},
	}

	resp := h.buildItemInformationResponse(
		session, item, "INST01", "ITEM-AG-005",
		nil, nil, "", nil, "", "", true,
	)

	if !strings.HasPrefix(resp, "18") {
		t.Errorf("Response should start with '18', got: %s", resp[:minInt(5, len(resp))])
	}
	if !strings.Contains(resp, "AGInspect for damage") {
		t.Errorf("Response should contain AGInspect for damage\nfull response: %s", resp)
	}
	if strings.Contains(resp, "AGRemind patron about DVD") {
		t.Errorf("Response should not contain checkout note as AG field\nfull response: %s", resp)
	}
}

// makeDATestConfig builds a TenantConfig with message "17" / field "DA" configured
// with the supplied preferredFirstName pointer (nil → field absent, defaults to true).
func makeDATestConfig(preferredFirstName *bool) *config.TenantConfig {
	return &config.TenantConfig{
		Tenant:                   "test-tenant",
		MessageDelimiter:         "\r",
		FieldDelimiter:           "|",
		Charset:                  "UTF-8",
		CirculationStatusMapping: map[string]string{},
		SupportedMessages: []config.MessageSupport{
			{
				Code:    "17",
				Enabled: true,
				Fields: []config.FieldConfiguration{
					{
						Code:               "DA",
						Enabled:            true,
						PreferredFirstName: preferredFirstName,
					},
				},
			},
		},
	}
}

// TestIsPreferredFirstNameEnabled_DA verifies that IsPreferredFirstNameEnabled
// returns the correct value for message "17" / field "DA" under each config variant.
func TestIsPreferredFirstNameEnabled_DA(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name               string
		preferredFirstName *bool
		want               bool
	}{
		{
			name:               "nil (absent) defaults to false",
			preferredFirstName: nil,
			want:               false,
		},
		{
			name:               "explicit true returns true",
			preferredFirstName: boolPtr(true),
			want:               true,
		},
		{
			name:               "explicit false returns false",
			preferredFirstName: boolPtr(false),
			want:               false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tc := makeDATestConfig(tt.preferredFirstName)
			got := tc.IsPreferredFirstNameEnabled("17", "DA")
			if got != tt.want {
				t.Errorf("IsPreferredFirstNameEnabled(17, DA) = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestFormatRequestorName_PreferredFirstName verifies formatRequestorName (one-arg form).
// Preferred-name selection now happens at the Handle level via GetUserByID; this
// function only formats whatever RequestRequester data is present.
func TestFormatRequestorName_PreferredFirstName(t *testing.T) {
	t.Parallel()

	h := newItemHandler(nil)

	tests := []struct {
		name      string
		requester *models.RequestRequester
		want      string
	}{
		{
			name:      "nil requester returns empty string",
			requester: nil,
			want:      "",
		},
		{
			name:      "no LastName returns empty string",
			requester: mockRequesterWithPreferredName("", "Jane", ""),
			want:      "",
		},
		{
			name:      "LastName and FirstName returns last-comma-first",
			requester: mockRequesterWithPreferredName("Smith", "Jane", ""),
			want:      "Smith, Jane",
		},
		{
			name:      "LastName only returns LastName",
			requester: mockRequesterWithPreferredName("Smith", "", ""),
			want:      "Smith",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := h.formatRequestorName(tt.requester)
			if got != tt.want {
				t.Errorf("formatRequestorName() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestBuildItemInformationResponse_DA_NameInResponse verifies that when a
// pre-computed requestorName is passed to buildItemInformationResponse the DA
// field value appears in the serialised SIP2 response string.
func TestBuildItemInformationResponse_DA_NameInResponse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		requestorName string
		wantInResp    string
		wantAbsent    string
	}{
		{
			name:          "preferred name appears in DA field",
			requestorName: "Smith, Janie",
			wantInResp:    "Smith, Janie",
		},
		{
			name:          "regular first name appears in DA field",
			requestorName: "Smith, Jane",
			wantInResp:    "Smith, Jane",
		},
		{
			name:          "empty requestorName produces no DA field",
			requestorName: "",
			wantAbsent:    "DASmith",
		},
	}

	item := &models.Item{
		ID:      "item-uuid",
		Barcode: "ITEM001",
		Status:  models.ItemStatus{Name: "Awaiting pickup"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tc := makeDATestConfig(boolPtr(true))
			h := NewItemInformationHandler(zap.NewNop(), tc)
			session := types.NewSession("da-resp-test", tc)

			resp := h.buildItemInformationResponse(
				session, item, "INST01", "ITEM001",
				nil, nil, "", nil, "P-DA-001", tt.requestorName, true,
			)

			if tt.wantInResp != "" && !strings.Contains(resp, tt.wantInResp) {
				t.Errorf("DA field: response does not contain %q\nfull response: %s",
					tt.wantInResp, resp)
			}
			if tt.wantAbsent != "" && strings.Contains(resp, tt.wantAbsent) {
				t.Errorf("DA field: response should not contain %q\nfull response: %s",
					tt.wantAbsent, resp)
			}
		})
	}
}
