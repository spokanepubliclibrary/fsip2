package handlers

import (
	"context"
	"strings"
	"testing"

	"github.com/spokanepubliclibrary/fsip2/internal/folio/models"
	"github.com/spokanepubliclibrary/fsip2/internal/sip2/parser"
	"github.com/spokanepubliclibrary/fsip2/internal/types"
	"github.com/spokanepubliclibrary/fsip2/tests/testutil"
	"go.uber.org/zap"
)

// TestCheckinHandler_BuildCheckinResponse tests the basic response building
func TestCheckinHandler_BuildCheckinResponse(t *testing.T) {
	tenantConfig := testutil.NewTenantConfig()
	tenantConfig.Timezone = "America/New_York"

	logger := zap.NewNop()
	handler := NewCheckinHandler(logger, tenantConfig)

	msg := &parser.Message{
		Code:           parser.CheckinRequest,
		SequenceNumber: "0",
		Fields:         make(map[string]string),
	}

	session := &types.Session{
		TenantConfig: tenantConfig,
	}

	tests := []struct {
		name            string
		ok              bool
		institutionID   string
		itemIdentifier  string
		currentLocation string
		expectedPrefix  string
	}{
		{
			name:            "Success",
			ok:              true,
			institutionID:   "INST01",
			itemIdentifier:  "123456789",
			currentLocation: "CIRC",
			expectedPrefix:  "101",
		},
		{
			name:            "Failure",
			ok:              false,
			institutionID:   "INST01",
			itemIdentifier:  "123456789",
			currentLocation: "CIRC",
			expectedPrefix:  "100",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := handler.buildCheckinResponse(tt.ok, tt.institutionID, tt.itemIdentifier, tt.currentLocation, msg, session)

			// Should start with "10" (message code)
			if !strings.HasPrefix(response, "10") {
				t.Errorf("Expected response to start with '10', got: %s", response)
			}

			// Should contain the expected ok value
			if !strings.HasPrefix(response, tt.expectedPrefix) {
				t.Errorf("Expected response to start with '%s', got: %s", tt.expectedPrefix, response[:3])
			}

			// Should contain institution ID
			if !strings.Contains(response, "AO"+tt.institutionID) {
				t.Errorf("Expected response to contain 'AO%s', got: %s", tt.institutionID, response)
			}

			// Should contain item identifier
			if !strings.Contains(response, "AB"+tt.itemIdentifier) {
				t.Errorf("Expected response to contain 'AB%s', got: %s", tt.itemIdentifier, response)
			}

			// Should contain current location if provided
			if tt.currentLocation != "" && !strings.Contains(response, "AP"+tt.currentLocation) {
				t.Errorf("Expected response to contain 'AP%s', got: %s", tt.currentLocation, response)
			}
		})
	}
}

// TestCheckinHandler_BuildCheckinResponseWithData tests enhanced response building with all fields
func TestCheckinHandler_BuildCheckinResponseWithData(t *testing.T) {
	tenantConfig := testutil.NewTenantConfig(testutil.WithErrorDetection(false))
	tenantConfig.Timezone = "America/New_York"

	logger := zap.NewNop()
	handler := NewCheckinHandler(logger, tenantConfig)
	session := types.NewSession("test-session", tenantConfig)

	tests := []struct {
		name              string
		data              *checkinResponseData
		expectedFields    map[string]string
		notExpectedFields []string
	}{
		{
			name: "Complete checkin with all fields",
			data: &checkinResponseData{
				ok:                  true,
				institutionID:       "INST01",
				itemBarcode:         "123456789",
				currentLocation:     "CIRC",
				permanentLocation:   "Main Library",
				title:               "The Great Gatsby",
				materialType:        "book",
				mediaTypeCode:       "001",
				callNumber:          "PS3525.A84 G7 2004",
				alertType:           "01",
				destinationLocation: "Reference Desk",
				sequenceNumber:      "0",
			},
			expectedFields: map[string]string{
				"AO": "INST01",             // Institution ID
				"AB": "123456789",          // Item barcode
				"AQ": "Main Library",       // Permanent location
				"AP": "CIRC",               // Current location
				"AJ": "The Great Gatsby",   // Title
				"CH": "book",               // Material type
				"CK": "001",                // Media type code
				"CS": "PS3525.A84 G7 2004", // Call number
				"CV": "01",                 // Alert type
				"CT": "Reference Desk",     // Destination location
			},
			notExpectedFields: []string{},
		},
		{
			name: "Minimal checkin without optional fields",
			data: &checkinResponseData{
				ok:                true,
				institutionID:     "INST01",
				itemBarcode:       "123456789",
				permanentLocation: "",
				sequenceNumber:    "0",
			},
			expectedFields: map[string]string{
				"AO": "INST01",    // Institution ID
				"AB": "123456789", // Item barcode
				"AQ": "",          // Permanent location (always included, even if blank)
				"AJ": "123456789", // Title (fallback to barcode)
			},
			notExpectedFields: []string{"CH", "CK", "CS", "CV", "CT", "AP"},
		},
		{
			name: "Checkin with hold (alert type 01)",
			data: &checkinResponseData{
				ok:                  true,
				institutionID:       "INST01",
				itemBarcode:         "123456789",
				permanentLocation:   "Main Library",
				title:               "Book Title",
				alertType:           "01",
				destinationLocation: "Hold Shelf",
				sequenceNumber:      "0",
			},
			expectedFields: map[string]string{
				"CV": "01",         // Alert type - hold exists
				"CT": "Hold Shelf", // Destination
			},
			notExpectedFields: []string{},
		},
		{
			name: "Checkin in transit with hold (alert type 02)",
			data: &checkinResponseData{
				ok:                  true,
				institutionID:       "INST01",
				itemBarcode:         "987654321",
				permanentLocation:   "Branch Library",
				alertType:           "02",
				destinationLocation: "Branch A",
				sequenceNumber:      "0",
			},
			expectedFields: map[string]string{
				"CV": "02",       // Alert type - in transit with hold
				"CT": "Branch A", // Destination
			},
			notExpectedFields: []string{},
		},
		{
			name: "Checkin in transit only (alert type 04)",
			data: &checkinResponseData{
				ok:                  true,
				institutionID:       "INST01",
				itemBarcode:         "555555555",
				permanentLocation:   "Storage",
				alertType:           "04",
				destinationLocation: "Main Library",
				sequenceNumber:      "0",
			},
			expectedFields: map[string]string{
				"CV": "04",           // Alert type - in transit only
				"CT": "Main Library", // Destination
			},
			notExpectedFields: []string{},
		},
		{
			name: "Checkin with long title (truncation test)",
			data: &checkinResponseData{
				ok:                true,
				institutionID:     "INST01",
				itemBarcode:       "123456789",
				permanentLocation: "Main",
				title:             "This is a very long title that should be truncated to sixty",
				sequenceNumber:    "0",
			},
			expectedFields: map[string]string{
				"AJ": "This is a very long title that should be truncated to sixty",
			},
			notExpectedFields: []string{},
		},
		{
			name: "Checkin with various material types",
			data: &checkinResponseData{
				ok:                true,
				institutionID:     "INST01",
				itemBarcode:       "111111111",
				permanentLocation: "Media Center",
				materialType:      "dvd",
				mediaTypeCode:     "006",
				sequenceNumber:    "0",
			},
			expectedFields: map[string]string{
				"CH": "dvd", // Material type
				"CK": "006", // Media type (CD/DVD)
			},
			notExpectedFields: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := handler.buildCheckinResponseWithData(tt.data, session)

			// Should start with "10" (message code)
			if !strings.HasPrefix(response, "10") {
				t.Errorf("Expected response to start with '10', got: %s", response)
			}

			// Check for expected fields
			for fieldCode, expectedValue := range tt.expectedFields {
				expectedField := fieldCode + expectedValue
				if !strings.Contains(response, expectedField) {
					t.Errorf("Expected response to contain '%s', got: %s", expectedField, response)
				}
			}

			// Check that not expected fields are absent
			for _, fieldCode := range tt.notExpectedFields {
				// Check that the field code doesn't appear (except in the message delimiter area)
				parts := strings.Split(response, "|")
				for _, part := range parts {
					if strings.HasPrefix(part, fieldCode) && len(part) > 2 {
						t.Errorf("Did not expect field '%s' in response, but found it: %s", fieldCode, response)
					}
				}
			}

			// Check alert flag in fixed fields
			if len(response) >= 7 {
				alertFlag := string(response[5])
				expectedAlertFlag := "N"
				if tt.data.alertType != "" {
					expectedAlertFlag = "Y"
				}
				if alertFlag != expectedAlertFlag {
					t.Errorf("Expected alert flag '%s', got '%s' in response: %s", expectedAlertFlag, alertFlag, response[:20])
				}
			}

			// Check for screen message
			if tt.data.ok {
				if !strings.Contains(response, "AFCheckin successful") {
					t.Errorf("Expected success message in response: %s", response)
				}
			} else {
				if !strings.Contains(response, "AFCheckin failed") {
					t.Errorf("Expected failure message in response: %s", response)
				}
			}
		})
	}
}

// TestCheckinHandler_TruncateString tests the title truncation function
func TestTruncateString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxLen   int
		expected string
	}{
		{
			name:     "Short string - no truncation",
			input:    "Short Title",
			maxLen:   60,
			expected: "Short Title",
		},
		{
			name:     "Exactly 60 characters",
			input:    "This title is exactly sixty characters long for testing it",
			maxLen:   60,
			expected: "This title is exactly sixty characters long for testing it",
		},
		{
			name:     "Long string - needs truncation",
			input:    "This is a very long title that exceeds sixty characters and needs to be truncated properly",
			maxLen:   60,
			expected: "This is a very long title that exceeds sixty characters and ",
		},
		{
			name:     "Empty string",
			input:    "",
			maxLen:   60,
			expected: "",
		},
		{
			name:     "Unicode characters",
			input:    "Café résumé naïve 日本語 中文",
			maxLen:   20,
			expected: "Café résumé naïve 日本",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateString(tt.input, tt.maxLen)
			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
			if len([]rune(result)) > tt.maxLen {
				t.Errorf("Result length %d exceeds max length %d", len([]rune(result)), tt.maxLen)
			}
		})
	}
}

// TestCalculateAlertType tests the alert type calculation logic
// NOTE: As of the CV fix, callers pass (isAwaitingPickup || hasHoldOrRecall) as the
// second argument. The "Awaiting pickup" item status path reaches the (false, true) → "01"
// case here — test coverage for that path is in TestCheckinHandle_LocalHold_*.
func TestCalculateAlertType(t *testing.T) {
	tests := []struct {
		name            string
		inTransit       bool
		hasHoldOrRecall bool
		expected        string
	}{
		{
			name:            "No alert - not in transit, no holds",
			inTransit:       false,
			hasHoldOrRecall: false,
			expected:        "",
		},
		{
			name:            "Alert 01 - hold exists, not in transit",
			inTransit:       false,
			hasHoldOrRecall: true,
			expected:        "01",
		},
		{
			name:            "Alert 02 - in transit with hold",
			inTransit:       true,
			hasHoldOrRecall: true,
			expected:        "02",
		},
		{
			name:            "Alert 04 - in transit only",
			inTransit:       true,
			hasHoldOrRecall: false,
			expected:        "04",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculateAlertType(tt.inTransit, tt.hasHoldOrRecall)
			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

// TestCheckinHandler_Handle_MissingFields tests validation of required fields
func TestCheckinHandler_Handle_MissingFields(t *testing.T) {
	tenantConfig := testutil.NewTenantConfig()
	tenantConfig.Timezone = "America/New_York"

	logger := zap.NewNop()
	handler := NewCheckinHandler(logger, tenantConfig)
	unauthSession := types.NewSession("test-session", tenantConfig)

	tests := []struct {
		name           string
		fields         map[string]string
		session        *types.Session
		expectedPrefix string
	}{
		{
			name: "Missing item identifier",
			fields: map[string]string{
				string(parser.InstitutionID): "INST01",
			},
			session:        unauthSession,
			expectedPrefix: "100", // Should fail
		},
		{
			name:           "Missing both required fields",
			fields:         map[string]string{},
			session:        unauthSession,
			expectedPrefix: "100", // Should fail
		},
		{
			name: "Missing CP in session",
			fields: map[string]string{
				string(parser.InstitutionID):   "INST01",
				string(parser.ItemIdentifier):  "123456789",
				string(parser.CurrentLocation): "AP-LOC",
			},
			// Authenticated session without LocationCode — CP was not sent at login
			session:        testutil.NewAuthSession(tenantConfig),
			expectedPrefix: "100", // Should fail — CP required
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := &parser.Message{
				Code:           parser.CheckinRequest,
				SequenceNumber: "0",
				Fields:         tt.fields,
			}

			ctx := context.Background()
			response, err := handler.Handle(ctx, msg, tt.session)

			if err != nil {
				t.Errorf("Expected no error, got: %v", err)
			}

			if !strings.HasPrefix(response, tt.expectedPrefix) {
				t.Errorf("Expected response to start with '%s', got: %s", tt.expectedPrefix, response[:3])
			}
		})
	}
}

// TestCheckinResponseData_AlertType verifies that the CV field in the built response
// reflects the alertType set in checkinResponseData.
func TestCheckinResponseData_AlertType(t *testing.T) {
	tests := []struct {
		name         string
		alertType    string
		wantCV       string
		wantContains bool
	}{
		{
			name:         "Awaiting pickup - CV=01",
			alertType:    "01",
			wantCV:       "CV01",
			wantContains: true,
		},
		{
			name:         "In transit with hold - CV=02",
			alertType:    "02",
			wantCV:       "CV02",
			wantContains: true,
		},
		{
			name:         "In transit no hold - CV=04",
			alertType:    "04",
			wantCV:       "CV04",
			wantContains: true,
		},
		{
			name:         "Available no alert - CV absent",
			alertType:    "",
			wantCV:       "CV0", // CV| always appears as empty token; CV01/02/04 all start with CV0
			wantContains: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc := testutil.NewTenantConfig()
			sess := testutil.NewAuthSession(tc)
			h := NewCheckinHandler(zap.NewNop(), tc)
			data := &checkinResponseData{
				ok:            true,
				institutionID: "TEST-INST",
				itemBarcode:   "ITEM-001",
				alertType:     tt.alertType,
			}
			response := h.buildCheckinResponseWithData(data, sess)
			if tt.wantContains {
				if !strings.Contains(response, tt.wantCV) {
					t.Errorf("expected response to contain %q, got: %s", tt.wantCV, response)
				}
			} else {
				if strings.Contains(response, tt.wantCV) {
					t.Errorf("expected response NOT to contain %q, got: %s", tt.wantCV, response)
				}
			}
		})
	}
}

// TestCheckinResponseDataStructure tests the checkinResponseData structure
func TestCheckinResponseDataStructure(t *testing.T) {
	data := &checkinResponseData{
		ok:                  true,
		institutionID:       "INST01",
		itemBarcode:         "123456789",
		currentLocation:     "CIRC",
		permanentLocation:   "Main Library",
		title:               "Test Title",
		materialType:        "book",
		mediaTypeCode:       "001",
		callNumber:          "QA76.73 .G38",
		alertType:           "01",
		destinationLocation: "Hold Shelf",
		sequenceNumber:      "0",
		item:                &models.Item{ID: "item-uuid"},
	}

	// Verify all fields are accessible
	if data.ok != true {
		t.Error("ok field not set correctly")
	}
	if data.institutionID != "INST01" {
		t.Error("institutionID field not set correctly")
	}
	if data.itemBarcode != "123456789" {
		t.Error("itemBarcode field not set correctly")
	}
	if data.item == nil || data.item.ID != "item-uuid" {
		t.Error("item field not set correctly")
	}
}

// TestCheckinHandler_ClaimedReturnedResolutionConfig tests that config is properly used
func TestCheckinHandler_ClaimedReturnedResolutionConfig(t *testing.T) {
	tests := []struct {
		name                      string
		claimedReturnedResolution string
		expectedFOLIOValue        string
	}{
		{
			name:                      "Patron resolution configured",
			claimedReturnedResolution: "patron",
			expectedFOLIOValue:        "Returned by patron",
		},
		{
			name:                      "Library resolution configured",
			claimedReturnedResolution: "library",
			expectedFOLIOValue:        "Found by library",
		},
		{
			name:                      "None resolution configured",
			claimedReturnedResolution: "none",
			expectedFOLIOValue:        "",
		},
		{
			name:                      "Empty resolution defaults to none",
			claimedReturnedResolution: "",
			expectedFOLIOValue:        "",
		},
		{
			name:                      "Invalid resolution defaults to none",
			claimedReturnedResolution: "invalid",
			expectedFOLIOValue:        "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tenantConfig := testutil.NewTenantConfig()
			tenantConfig.Timezone = "America/New_York"
			tenantConfig.ClaimedReturnedResolution = tt.claimedReturnedResolution

			// Test that the config methods work correctly
			result := tenantConfig.MapClaimedReturnedResolutionToFOLIO()
			if result != tt.expectedFOLIOValue {
				t.Errorf("MapClaimedReturnedResolutionToFOLIO() = %v, want %v", result, tt.expectedFOLIOValue)
			}
		})
	}
}

// TestCheckinHandler_BuildCheckinResponseWithMessage tests custom error message building
func TestCheckinHandler_BuildCheckinResponseWithMessage(t *testing.T) {
	tenantConfig := testutil.NewTenantConfig()
	tenantConfig.Timezone = "America/New_York"

	logger := zap.NewNop()
	handler := NewCheckinHandler(logger, tenantConfig)
	session := types.NewSession("test-session", tenantConfig)

	msg := &parser.Message{
		Code:           parser.CheckinRequest,
		SequenceNumber: "0",
		Fields:         make(map[string]string),
	}

	tests := []struct {
		name            string
		ok              bool
		institutionID   string
		itemIdentifier  string
		currentLocation string
		screenMessage   string
		expectedOk      string
		expectedMessage string
	}{
		{
			name:            "Claimed returned - blocked by config",
			ok:              false,
			institutionID:   "INST01",
			itemIdentifier:  "123456789",
			currentLocation: "CIRC",
			screenMessage:   "Checkin failed - Item is claimed returned",
			expectedOk:      "100",
			expectedMessage: "AFCheckin failed - Item is claimed returned",
		},
		{
			name:            "Custom success message",
			ok:              true,
			institutionID:   "INST01",
			itemIdentifier:  "987654321",
			currentLocation: "REF",
			screenMessage:   "Custom success message",
			expectedOk:      "101",
			expectedMessage: "AFCustom success message",
		},
		{
			name:            "Custom error message",
			ok:              false,
			institutionID:   "INST01",
			itemIdentifier:  "111111111",
			currentLocation: "MAIN",
			screenMessage:   "Item not found",
			expectedOk:      "100",
			expectedMessage: "AFItem not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := handler.buildCheckinResponseWithMessage(
				tt.ok,
				tt.institutionID,
				tt.itemIdentifier,
				tt.currentLocation,
				tt.screenMessage,
				msg,
				session,
			)

			// Check response starts with correct ok value
			if !strings.HasPrefix(response, tt.expectedOk) {
				t.Errorf("Expected response to start with '%s', got: %s", tt.expectedOk, response[:3])
			}

			// Check screen message is included
			if !strings.Contains(response, tt.expectedMessage) {
				t.Errorf("Expected response to contain '%s', got: %s", tt.expectedMessage, response)
			}

			// Check institution ID
			if !strings.Contains(response, "AO"+tt.institutionID) {
				t.Errorf("Expected response to contain 'AO%s', got: %s", tt.institutionID, response)
			}

			// Check item identifier
			if !strings.Contains(response, "AB"+tt.itemIdentifier) {
				t.Errorf("Expected response to contain 'AB%s', got: %s", tt.itemIdentifier, response)
			}
		})
	}
}

// TestCheckinHandler_ClaimedReturnedIntegration documents integration test requirements
func TestCheckinHandler_ClaimedReturnedIntegration(t *testing.T) {
	// This test documents what would be tested with proper FOLIO mocking:
	//
	// Test Case 1: Claimed returned item with "patron" resolution
	// - Config: claimedReturnedResolution: "patron"
	// - Item status: "Claimed returned"
	// - Expected: Checkin succeeds, FOLIO receives claimedReturnedResolution: "Returned by patron"
	//
	// Test Case 2: Claimed returned item with "library" resolution
	// - Config: claimedReturnedResolution: "library"
	// - Item status: "Claimed returned"
	// - Expected: Checkin succeeds, FOLIO receives claimedReturnedResolution: "Found by library"
	//
	// Test Case 3: Claimed returned item with "none" resolution
	// - Config: claimedReturnedResolution: "none"
	// - Item status: "Claimed returned"
	// - Expected: Checkin is blocked, returns error "Checkin failed - Item is claimed returned"
	//
	// Test Case 4: Normal item with claimed returned config set
	// - Config: claimedReturnedResolution: "patron"
	// - Item status: "Available" or "Checked out"
	// - Expected: Checkin proceeds normally, no claimedReturnedResolution sent if not needed
	//
	// Implementation requires:
	// - Mock FOLIO Inventory client (GetItemByBarcode)
	// - Mock FOLIO Circulation client (Checkin)
	// - Ability to inspect the CheckinRequest sent to FOLIO

	t.Skip("Integration test - requires FOLIO mocking framework")
}
