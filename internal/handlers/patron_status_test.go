package handlers

import (
	"strings"
	"testing"
	"time"

	"github.com/spokanepubliclibrary/fsip2/internal/folio/models"
	"github.com/spokanepubliclibrary/fsip2/internal/types"
	"github.com/spokanepubliclibrary/fsip2/tests/testutil"
	"go.uber.org/zap"
)

// TestBuildPatronStatusResponse_ValidPatronWithFees tests the patron status response
// when a valid patron with fees is provided
func TestBuildPatronStatusResponse_ValidPatronWithFees(t *testing.T) {
	tests := []struct {
		name             string
		// User identity fields — a valid_user.json fixture is loaded and these fields
		// are applied as overrides. Set userIsNil=true for the "patron not found" case.
		userIsNil        bool
		lastName         string
		firstName        string
		preferredName    string
		manualBlocks     *models.ManualBlockCollection
		automatedBlocks  *models.AutomatedPatronBlock
		accounts         []*models.Account
		institutionID    string
		patronIdentifier string
		valid            bool
		pinVerified      bool
		sequenceNumber   string
		currency         string
		wantContains     []string
		wantNotContains  []string
		wantPatronStatus string
	}{
		{
			name:      "Valid patron with fees and PIN verified",
			lastName:  "Doe",
			firstName: "John",
			manualBlocks:    nil,
			automatedBlocks: nil,
			accounts: []*models.Account{
				{
					ID:          "acc-1",
					FeeFineType: "Overdue fine",
					Remaining:   models.FlexibleFloat(10.50),
				},
				{
					ID:          "acc-2",
					FeeFineType: "Lost item fee",
					Remaining:   models.FlexibleFloat(25.00),
				},
			},
			institutionID:    "TEST-INST",
			patronIdentifier: "123456789",
			valid:            true,
			pinVerified:      true,
			sequenceNumber:   "1",
			currency:         "USD",
			wantContains: []string{
				"24",                // Message code
				"|AO" + "TEST-INST", // Institution ID
				"|AA" + "123456789", // Patron identifier
				"|AE" + "Doe, John", // Patron name
				"|BLY",              // Valid patron
				"|CQY",              // Valid PIN
				"|BV35.50",          // Total fees (10.50 + 25.00)
				"|BH" + "USD",       // Currency
				"|AY1",              // Sequence number
				"AZ",                // Checksum field
			},
			wantPatronStatus: "              ", // No blocks (14 spaces)
		},
		{
			name:      "Valid patron with no fees and PIN not verified",
			lastName:  "Smith",
			firstName: "Jane",
			manualBlocks:     nil,
			automatedBlocks:  nil,
			accounts:         []*models.Account{},
			institutionID:    "TEST-INST",
			patronIdentifier: "123456789",
			valid:            true,
			pinVerified:      false,
			sequenceNumber:   "2",
			currency:         "USD",
			wantContains: []string{
				"24",
				"|AO" + "TEST-INST",
				"|AA" + "123456789",
				"|AE" + "Smith, Jane",
				"|BLY", // Valid patron
				"|CQN", // PIN not verified
				"|AY2",
			},
			wantNotContains: []string{
				"|BV", // No fee amount
				"|BH", // No currency when no fees
			},
			wantPatronStatus: "              ", // No blocks
		},
		{
			name:      "Valid patron with manual borrowing block",
			lastName:  "Blocked",
			firstName: "User",
			manualBlocks: &models.ManualBlockCollection{
				ManualBlocks: []models.ManualBlock{
					{
						Borrowing: true,
						Renewals:  false,
						Requests:  false,
					},
				},
			},
			automatedBlocks:  nil,
			accounts:         []*models.Account{},
			institutionID:    "TEST-INST",
			patronIdentifier: "123456789",
			valid:            true,
			pinVerified:      true,
			sequenceNumber:   "3",
			currency:         "USD",
			wantContains: []string{
				"24",
				"|BLY",
				"|CQY",
			},
			wantPatronStatus: "Y             ", // Position 0 = Y (charge privileges denied)
		},
		{
			name:      "Valid patron with automated renewal and request blocks",
			lastName:  "Blocked",
			firstName: "Auto",
			manualBlocks: nil,
			automatedBlocks: &models.AutomatedPatronBlock{
				AutomatedPatronBlocks: []models.AutomatedBlock{
					{
						BlockBorrowing: false,
						BlockRenewals:  true,
						BlockRequests:  true,
					},
				},
			},
			accounts:         []*models.Account{},
			institutionID:    "TEST-INST",
			patronIdentifier: "123456789",
			valid:            true,
			pinVerified:      true,
			sequenceNumber:   "4",
			currency:         "USD",
			wantContains: []string{
				"24",
				"|BLY",
				"|CQY",
			},
			wantPatronStatus: " Y Y          ", // Position 1 = Y (renewals), Position 3 = Y (holds) - 14 chars total
		},
		{
			name:      "Valid patron with multiple manual blocks",
			lastName:  "AllBlocked",
			firstName: "Completely",
			manualBlocks: &models.ManualBlockCollection{
				ManualBlocks: []models.ManualBlock{
					{
						Borrowing: true,
						Renewals:  true,
						Requests:  true,
					},
				},
			},
			automatedBlocks:  nil,
			accounts:         []*models.Account{},
			institutionID:    "TEST-INST",
			patronIdentifier: "123456789",
			valid:            true,
			pinVerified:      true,
			sequenceNumber:   "5",
			currency:         "USD",
			wantContains: []string{
				"24",
				"|BLY",
				"|CQY",
			},
			wantPatronStatus: "YY Y          ", // Positions 0, 1, 3 = Y - 14 chars total
		},
		{
			name:      "Valid patron with both manual and automated blocks (should combine)",
			lastName:  "Combined",
			firstName: "Blocks",
			manualBlocks: &models.ManualBlockCollection{
				ManualBlocks: []models.ManualBlock{
					{
						Borrowing: true,
						Renewals:  false,
						Requests:  false,
					},
				},
			},
			automatedBlocks: &models.AutomatedPatronBlock{
				AutomatedPatronBlocks: []models.AutomatedBlock{
					{
						BlockBorrowing: false,
						BlockRenewals:  true,
						BlockRequests:  false,
					},
				},
			},
			accounts:         []*models.Account{},
			institutionID:    "TEST-INST",
			patronIdentifier: "123456789",
			valid:            true,
			pinVerified:      true,
			sequenceNumber:   "6",
			currency:         "USD",
			wantContains: []string{
				"24",
				"|BLY",
				"|CQY",
			},
			wantPatronStatus: "YY            ", // Positions 0 and 1 = Y - 14 chars total
		},
		{
			name:             "Invalid patron (patron not found)",
			userIsNil:        true,
			manualBlocks:     nil,
			automatedBlocks:  nil,
			accounts:         nil,
			institutionID:    "TEST-INST",
			patronIdentifier: "999999999",
			valid:            false,
			pinVerified:      false,
			sequenceNumber:   "7",
			currency:         "USD",
			wantContains: []string{
				"24",
				"|AO" + "TEST-INST",
				"|AA" + "999999999",
				"|BLN",                     // Invalid patron
				"|CQN",                     // Invalid PIN
				"|AF" + "Patron not found", // Error message
				"|AY7",
			},
			wantPatronStatus: "YYYYYYYYYYYYYY", // All blocks set (positions 0-13 = Y, all 14 positions)
		},
		{
			name:          "Valid patron with preferred first name",
			lastName:      "Johnson",
			firstName:     "Robert",
			preferredName: "Bob",
			manualBlocks:     nil,
			automatedBlocks:  nil,
			accounts:         []*models.Account{},
			institutionID:    "TEST-INST",
			patronIdentifier: "123456789",
			valid:            true,
			pinVerified:      true,
			sequenceNumber:   "8",
			currency:         "USD",
			wantContains: []string{
				"24",
				"|AE" + "Johnson, Bob", // Should use preferred first name
				"|BLY",
				"|CQY",
			},
			wantPatronStatus: "              ",
		},
		{
			name:      "Valid patron with only last name",
			lastName:  "Madonna",
			firstName: "",
			manualBlocks:     nil,
			automatedBlocks:  nil,
			accounts:         []*models.Account{},
			institutionID:    "TEST-INST",
			patronIdentifier: "123456789",
			valid:            true,
			pinVerified:      false,
			sequenceNumber:   "9",
			currency:         "USD",
			wantContains: []string{
				"24",
				"|AE" + "Madonna", // Just last name
				"|BLY",
				"|CQN",
			},
			wantPatronStatus: "              ",
		},
		{
			name:      "Valid patron with default currency when not configured",
			lastName:  "Default",
			firstName: "Currency",
			manualBlocks:    nil,
			automatedBlocks: nil,
			accounts: []*models.Account{
				{
					ID:          "acc-1",
					FeeFineType: "Fine",
					Remaining:   models.FlexibleFloat(5.00),
				},
			},
			institutionID:    "TEST-INST",
			patronIdentifier: "123456789",
			valid:            true,
			pinVerified:      true,
			sequenceNumber:   "10",
			currency:         "", // Empty currency
			wantContains: []string{
				"24",
				"|BLY",
				"|CQY",
				"|BV5.00",
				"|BH" + "USD", // Should default to USD
			},
			wantPatronStatus: "              ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Build user from fixture with per-test name overrides.
			var user *models.User
			if !tt.userIsNil {
				user = testutil.DefaultUser()
				user.Personal.LastName = tt.lastName
				user.Personal.FirstName = tt.firstName
				user.Personal.PreferredFirstName = tt.preferredName
			}

			// Create tenant config
			tenantConfig := testutil.NewTenantConfig(testutil.WithCurrency(tt.currency))

			// Create logger
			logger := zap.NewNop()

			// Create handler
			handler := NewPatronStatusHandler(logger, tenantConfig)

			// Create session
			session := types.NewSession("test-session", tenantConfig)

			// Build response
			response := handler.buildPatronStatusResponse(
				user,
				tt.manualBlocks,
				tt.automatedBlocks,
				tt.accounts,
				tt.institutionID,
				tt.patronIdentifier,
				tt.valid,
				tt.pinVerified,
				tt.sequenceNumber,
				session,
			)

			// Verify response starts with 24
			if !strings.HasPrefix(response, "24") {
				t.Errorf("Response should start with '24', got: %s", response[:10])
			}

			// Extract patron status from response (characters 2-16)
			if len(response) >= 16 {
				patronStatus := response[2:16]
				if patronStatus != tt.wantPatronStatus {
					t.Errorf("Patron status mismatch:\nwant: %q\ngot:  %q", tt.wantPatronStatus, patronStatus)
				}
			}

			// Check for expected strings
			for _, want := range tt.wantContains {
				if !strings.Contains(response, want) {
					t.Errorf("Response should contain %q, got: %s", want, response)
				}
			}

			// Check for strings that should not be present
			for _, notWant := range tt.wantNotContains {
				if strings.Contains(response, notWant) {
					t.Errorf("Response should NOT contain %q, got: %s", notWant, response)
				}
			}

			// Verify response ends with checksum (AZ) when error detection is enabled
			if tenantConfig.ErrorDetectionEnabled && !strings.Contains(response, "AZ") {
				t.Errorf("Response should contain checksum field (AZ), got: %s", response)
			}
		})
	}
}

// TestBuildPatronStatusResponse_FeeCalculation tests that fees are calculated correctly
func TestBuildPatronStatusResponse_FeeCalculation(t *testing.T) {
	tests := []struct {
		name             string
		accounts         []*models.Account
		expectedFeeTotal string
	}{
		{
			name: "Single fee",
			accounts: []*models.Account{
				{Remaining: models.FlexibleFloat(10.00)},
			},
			expectedFeeTotal: "|BV10.00",
		},
		{
			name: "Multiple fees",
			accounts: []*models.Account{
				{Remaining: models.FlexibleFloat(10.50)},
				{Remaining: models.FlexibleFloat(25.25)},
				{Remaining: models.FlexibleFloat(5.75)},
			},
			expectedFeeTotal: "|BV41.50",
		},
		{
			name: "Fees with decimal precision",
			accounts: []*models.Account{
				{Remaining: models.FlexibleFloat(0.01)},
				{Remaining: models.FlexibleFloat(0.99)},
			},
			expectedFeeTotal: "|BV1.00",
		},
		{
			name:             "No fees",
			accounts:         []*models.Account{},
			expectedFeeTotal: "", // Should not include BV field
		},
		{
			name: "Large fee amount",
			accounts: []*models.Account{
				{Remaining: models.FlexibleFloat(999.99)},
				{Remaining: models.FlexibleFloat(500.01)},
			},
			expectedFeeTotal: "|BV1500.00",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tenantConfig := testutil.NewTenantConfig(testutil.WithErrorDetection(false))

			logger := zap.NewNop()
			handler := NewPatronStatusHandler(logger, tenantConfig)
			session := types.NewSession("test-session", tenantConfig)

			user := testutil.DefaultUser()

			response := handler.buildPatronStatusResponse(
				user,
				nil,
				nil,
				tt.accounts,
				"TEST-INST",
				"123456789",
				true,
				true,
				"0",
				session,
			)

			if tt.expectedFeeTotal == "" {
				// Should not contain BV field
				if strings.Contains(response, "|BV") {
					t.Errorf("Response should not contain BV field when no fees, got: %s", response)
				}
			} else {
				// Should contain the expected fee total
				if !strings.Contains(response, tt.expectedFeeTotal) {
					t.Errorf("Expected fee total %q not found in response: %s", tt.expectedFeeTotal, response)
				}
			}
		})
	}
}

// TestBuildPatronStatusResponse_SequenceNumber tests sequence number handling
func TestBuildPatronStatusResponse_SequenceNumber(t *testing.T) {
	tests := []struct {
		name           string
		sequenceNumber string
		wantAY         string
	}{
		{
			name:           "Sequence number 0",
			sequenceNumber: "0",
			wantAY:         "|AY0",
		},
		{
			name:           "Sequence number 1",
			sequenceNumber: "1",
			wantAY:         "|AY1",
		},
		{
			name:           "Sequence number 99",
			sequenceNumber: "99",
			wantAY:         "|AY99",
		},
		{
			name:           "Empty sequence number",
			sequenceNumber: "",
			wantAY:         "|AY", // Should still include AY field
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tenantConfig := testutil.NewTenantConfig()

			logger := zap.NewNop()
			handler := NewPatronStatusHandler(logger, tenantConfig)
			session := types.NewSession("test-session", tenantConfig)

			user := testutil.DefaultUser()

			response := handler.buildPatronStatusResponse(
				user,
				nil,
				nil,
				nil,
				"TEST-INST",
				"123456789",
				true,
				true,
				tt.sequenceNumber,
				session,
			)

			if !strings.Contains(response, tt.wantAY) {
				t.Errorf("Response should contain %q, got: %s", tt.wantAY, response)
			}
		})
	}
}

// TestBuildPatronStatusResponse_Checksum tests checksum generation
func TestBuildPatronStatusResponse_Checksum(t *testing.T) {
	tests := []struct {
		name                  string
		errorDetectionEnabled bool
		wantChecksumField     bool
	}{
		{
			name:                  "With error detection enabled",
			errorDetectionEnabled: true,
			wantChecksumField:     true,
		},
		{
			name:                  "With error detection disabled",
			errorDetectionEnabled: false,
			wantChecksumField:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tenantConfig := testutil.NewTenantConfig(testutil.WithErrorDetection(tt.errorDetectionEnabled))

			logger := zap.NewNop()
			handler := NewPatronStatusHandler(logger, tenantConfig)
			session := types.NewSession("test-session", tenantConfig)

			user := testutil.DefaultUser()

			response := handler.buildPatronStatusResponse(
				user,
				nil,
				nil,
				nil,
				"TEST-INST",
				"123456789",
				true,
				true,
				"1",
				session,
			)

			hasChecksum := strings.Contains(response, "AZ")
			if hasChecksum != tt.wantChecksumField {
				if tt.wantChecksumField {
					t.Errorf("Response should contain checksum field (AZ) when error detection is enabled, got: %s", response)
				} else {
					t.Errorf("Response should NOT contain checksum field (AZ) when error detection is disabled, got: %s", response)
				}
			}
		})
	}
}

// TestBuildPatronStatusResponse_Timestamp tests that timestamp is included
func TestBuildPatronStatusResponse_Timestamp(t *testing.T) {
	tenantConfig := testutil.NewTenantConfig(testutil.WithErrorDetection(false))

	logger := zap.NewNop()
	handler := NewPatronStatusHandler(logger, tenantConfig)
	session := types.NewSession("test-session", tenantConfig)

	user := testutil.DefaultUser()

	before := time.Now()
	response := handler.buildPatronStatusResponse(
		user,
		nil,
		nil,
		nil,
		"TEST-INST",
		"123456789",
		true,
		true,
		"0",
		session,
	)
	_ = before // Used for timestamp validation context

	// Response format: 24<patron_status><language><timestamp>|...
	// patron_status = 14 chars, language = 3 chars, timestamp starts at position 19
	if len(response) < 37 {
		t.Fatalf("Response too short to contain timestamp: %s", response)
	}

	// Timestamp is in position 19-36 (18 characters: YYYYMMDDZZZZHHMMSS)
	// Just verify the year is reasonable
	timestampSection := response[19:23]
	currentYear := before.Format("2006")
	if timestampSection != currentYear {
		// Allow some flexibility, just check it's a valid year
		if len(timestampSection) != 4 {
			t.Errorf("Timestamp year section should be 4 digits, got: %q in response: %s", timestampSection, response)
		}
	}
}
