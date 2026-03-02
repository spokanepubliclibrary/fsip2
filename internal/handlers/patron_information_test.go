package handlers

import (
	"strings"
	"testing"
	"time"

	"github.com/spokanepubliclibrary/fsip2/internal/config"
	"github.com/spokanepubliclibrary/fsip2/internal/folio/models"
	"github.com/spokanepubliclibrary/fsip2/internal/types"
	"go.uber.org/zap"
)

// TestBuildPatronInformationResponse_ValidPatron tests the patron information response
// with various patron data scenarios
func TestBuildPatronInformationResponse_ValidPatron(t *testing.T) {
	tests := []struct {
		name             string
		user             *models.User
		manualBlocks     *models.ManualBlockCollection
		automatedBlocks  *models.AutomatedPatronBlock
		holds            []*models.Request
		loans            []*models.Loan
		overdueLoans     []*models.Loan
		accounts         []*models.Account
		unavailableHolds []*models.Request
		patronGroup      *models.PatronGroup
		institutionID    string
		patronIdentifier string
		language         string
		summary          string
		valid            bool
		pinVerified      bool
		sequenceNumber   string
		currency         string
		wantContains     []string
		wantNotContains  []string
		wantPatronStatus string
		wantHoldCount    string
		wantOverdueCount string
		wantChargedCount string
		wantFineCount    string
	}{
		{
			name: "Valid patron with no items or fees",
			user: &models.User{
				ID:       "user-123",
				Username: "testuser",
				Barcode:  "123456789",
				Personal: models.PersonalInfo{
					LastName:  "Doe",
					FirstName: "John",
					Email:     "john.doe@example.com",
				},
			},
			manualBlocks:     nil,
			automatedBlocks:  nil,
			holds:            []*models.Request{},
			loans:            []*models.Loan{},
			overdueLoans:     []*models.Loan{},
			accounts:         []*models.Account{},
			unavailableHolds: []*models.Request{},
			patronGroup:      nil,
			institutionID:    "TEST-INST",
			patronIdentifier: "123456789",
			language:         "000",
			summary:          "          ",
			valid:            true,
			pinVerified:      true,
			sequenceNumber:   "1",
			currency:         "USD",
			wantContains: []string{
				"64",                           // Message code
				"|AO" + "TEST-INST",            // Institution ID
				"|AA" + "123456789",            // Patron identifier
				"|AE" + "Doe, John",            // Patron name
				"|BE" + "john.doe@example.com", // Email
				"|BLY",                         // Valid patron
				"|CQY",                         // Valid PIN
				"|AY1",                         // Sequence number
			},
			wantNotContains: []string{
				"|BV", // No fees
				"|BH", // No currency when no fees
				"|AS", // No available holds
				"|AT", // No overdue items
				"|AU", // No charged items
				"|AV", // No fee details
			},
			wantPatronStatus: "              ", // No blocks
			wantHoldCount:    "0000",
			wantOverdueCount: "0000",
			wantChargedCount: "0000",
			wantFineCount:    "0000",
		},
		{
			name: "Valid patron with holds and loans",
			user: &models.User{
				ID:       "user-456",
				Username: "activeuser",
				Barcode:  "987654321",
				Personal: models.PersonalInfo{
					LastName:  "Smith",
					FirstName: "Jane",
					Email:     "jane.smith@example.com",
				},
			},
			manualBlocks:    nil,
			automatedBlocks: nil,
			holds: []*models.Request{
				{
					ID: "hold-1",
					Item: &models.RequestItem{
						Barcode: "ITEM001",
					},
				},
				{
					ID: "hold-2",
					Item: &models.RequestItem{
						Barcode: "ITEM002",
					},
				},
			},
			loans: []*models.Loan{
				{
					ID: "loan-1",
					Item: &models.Item{
						Barcode: "ITEM003",
					},
				},
			},
			overdueLoans:     []*models.Loan{},
			accounts:         []*models.Account{},
			unavailableHolds: []*models.Request{},
			patronGroup:      nil,
			institutionID:    "TEST-INST",
			patronIdentifier: "987654321",
			language:         "000",
			summary:          "Y Y        ",
			valid:            true,
			pinVerified:      false,
			sequenceNumber:   "2",
			currency:         "USD",
			wantContains: []string{
				"64",
				"|AE" + "Smith, Jane",
				"|BLY",
				"|CQN",            // PIN not verified
				"|AS" + "ITEM001", // Hold 1
				"|AS" + "ITEM002", // Hold 2
				"|AU" + "ITEM003", // Charged item
			},
			wantPatronStatus: "              ",
			wantHoldCount:    "0002",
			wantOverdueCount: "0000",
			wantChargedCount: "0001",
			wantFineCount:    "0000",
		},
		{
			name: "Valid patron with overdue items and fees",
			user: &models.User{
				ID:       "user-789",
				Username: "overdueuser",
				Barcode:  "111222333",
				Personal: models.PersonalInfo{
					LastName:  "Late",
					FirstName: "Always",
					Email:     "always.late@example.com",
					Phone:     "555-1234",
				},
			},
			manualBlocks:    nil,
			automatedBlocks: nil,
			holds:           []*models.Request{},
			loans:           []*models.Loan{},
			overdueLoans: []*models.Loan{
				{
					ID: "loan-overdue-1",
					Item: &models.Item{
						Barcode: "OVERDUE001",
					},
					DueDate: func() *time.Time { t := time.Now().Add(-7 * 24 * time.Hour); return &t }(),
				},
			},
			accounts: []*models.Account{
				{
					ID:          "acc-1",
					FeeFineType: "Overdue fine",
					Remaining:   models.FlexibleFloat(5.00),
					Title:       "The Great Gatsby",
				},
				{
					ID:          "acc-2",
					FeeFineType: "Lost item fee",
					Remaining:   models.FlexibleFloat(25.50),
					Title:       "To Kill a Mockingbird",
				},
			},
			unavailableHolds: []*models.Request{},
			patronGroup:      nil,
			institutionID:    "TEST-INST",
			patronIdentifier: "111222333",
			language:         "000",
			summary:          " YYY      ", // Position 1=overdue, 2=charged, 3=fines
			valid:            true,
			pinVerified:      true,
			sequenceNumber:   "3",
			currency:         "USD",
			wantContains: []string{
				"64",
				"|AE" + "Late, Always",
				"|BLY",
				"|CQY",
				"|AT" + "OVERDUE001", // Overdue item
				"|BV30.50",           // Total fees
				"|BH" + "USD",        // Currency
				"|AV",                // Fee details (should have 2)
				"Overdue fine",
				"Lost item fee",
			},
			wantPatronStatus: "              ",
			wantHoldCount:    "0000",
			wantOverdueCount: "0001",
			wantChargedCount: "0000",
			wantFineCount:    "0002",
		},
		{
			name: "Valid patron with manual borrowing block",
			user: &models.User{
				ID:       "user-blocked",
				Username: "blockeduser",
				Barcode:  "444555666",
				Personal: models.PersonalInfo{
					LastName:  "Blocked",
					FirstName: "User",
				},
			},
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
			holds:            []*models.Request{},
			loans:            []*models.Loan{},
			overdueLoans:     []*models.Loan{},
			accounts:         []*models.Account{},
			unavailableHolds: []*models.Request{},
			patronGroup:      nil,
			institutionID:    "TEST-INST",
			patronIdentifier: "444555666",
			language:         "000",
			summary:          "          ",
			valid:            true,
			pinVerified:      true,
			sequenceNumber:   "4",
			currency:         "USD",
			wantContains: []string{
				"64",
				"|BLY",
				"|CQY",
			},
			wantPatronStatus: "Y             ", // Position 0 = Y (charge privileges denied)
			wantHoldCount:    "0000",
			wantOverdueCount: "0000",
			wantChargedCount: "0000",
			wantFineCount:    "0000",
		},
		{
			name: "Valid patron with automated blocks",
			user: &models.User{
				ID:       "user-auto-blocked",
				Username: "autoblocked",
				Barcode:  "777888999",
				Personal: models.PersonalInfo{
					LastName:  "AutoBlocked",
					FirstName: "System",
				},
			},
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
			holds:            []*models.Request{},
			loans:            []*models.Loan{},
			overdueLoans:     []*models.Loan{},
			accounts:         []*models.Account{},
			unavailableHolds: []*models.Request{},
			patronGroup:      nil,
			institutionID:    "TEST-INST",
			patronIdentifier: "777888999",
			language:         "000",
			summary:          "          ",
			valid:            true,
			pinVerified:      true,
			sequenceNumber:   "5",
			currency:         "USD",
			wantContains: []string{
				"64",
				"|BLY",
				"|CQY",
			},
			wantPatronStatus: " Y Y          ", // Positions 1 and 3 = Y
			wantHoldCount:    "0000",
			wantOverdueCount: "0000",
			wantChargedCount: "0000",
			wantFineCount:    "0000",
		},
		{
			name: "Valid patron with combined manual and automated blocks",
			user: &models.User{
				ID:       "user-combo-blocked",
				Username: "comboblocked",
				Barcode:  "000111222",
				Personal: models.PersonalInfo{
					LastName:  "Combined",
					FirstName: "Blocks",
				},
			},
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
			holds:            []*models.Request{},
			loans:            []*models.Loan{},
			overdueLoans:     []*models.Loan{},
			accounts:         []*models.Account{},
			unavailableHolds: []*models.Request{},
			patronGroup:      nil,
			institutionID:    "TEST-INST",
			patronIdentifier: "000111222",
			language:         "000",
			summary:          "          ",
			valid:            true,
			pinVerified:      true,
			sequenceNumber:   "6",
			currency:         "USD",
			wantContains: []string{
				"64",
				"|BLY",
				"|CQY",
			},
			wantPatronStatus: "YY            ", // Positions 0 and 1 = Y
			wantHoldCount:    "0000",
			wantOverdueCount: "0000",
			wantChargedCount: "0000",
			wantFineCount:    "0000",
		},
		{
			name:             "Invalid patron (not found)",
			user:             nil,
			manualBlocks:     nil,
			automatedBlocks:  nil,
			holds:            nil,
			loans:            nil,
			overdueLoans:     nil,
			accounts:         nil,
			unavailableHolds: nil,
			patronGroup:      nil,
			institutionID:    "TEST-INST",
			patronIdentifier: "999999999",
			language:         "000",
			summary:          "          ",
			valid:            false,
			pinVerified:      false,
			sequenceNumber:   "7",
			currency:         "USD",
			wantContains: []string{
				"64",
				"|AO" + "TEST-INST",
				"|AA" + "999999999",
				"|BLN", // Invalid patron
				"|CQN", // Invalid PIN - Note: patron info doesn't add error message like patron status does
			},
			wantNotContains: []string{
				"|AE", // No patron name
				"|BE", // No email
				"|BV", // No fees
			},
			wantPatronStatus: "YYYYYYYYYYYYYY", // All blocks set
			wantHoldCount:    "0000",
			wantOverdueCount: "0000",
			wantChargedCount: "0000",
			wantFineCount:    "0000",
		},
		{
			name: "Valid patron with preferred first name",
			user: &models.User{
				ID:       "user-preferred",
				Username: "preferredname",
				Barcode:  "333444555",
				Personal: models.PersonalInfo{
					LastName:           "Johnson",
					FirstName:          "Robert",
					PreferredFirstName: "Bob",
					Email:              "bob.johnson@example.com",
				},
			},
			manualBlocks:     nil,
			automatedBlocks:  nil,
			holds:            []*models.Request{},
			loans:            []*models.Loan{},
			overdueLoans:     []*models.Loan{},
			accounts:         []*models.Account{},
			unavailableHolds: []*models.Request{},
			patronGroup:      nil,
			institutionID:    "TEST-INST",
			patronIdentifier: "333444555",
			language:         "000",
			summary:          "          ",
			valid:            true,
			pinVerified:      true,
			sequenceNumber:   "8",
			currency:         "USD",
			wantContains: []string{
				"64",
				"|AE" + "Johnson, Bob", // Should use preferred first name
				"|BLY",
				"|CQY",
			},
			wantPatronStatus: "              ",
			wantHoldCount:    "0000",
			wantOverdueCount: "0000",
			wantChargedCount: "0000",
			wantFineCount:    "0000",
		},
		{
			name: "Valid patron with address",
			user: &models.User{
				ID:       "user-address",
				Username: "useraddress",
				Barcode:  "666777888",
				Personal: models.PersonalInfo{
					LastName:  "AddressTest",
					FirstName: "User",
					Email:     "user@example.com",
					Addresses: []models.Address{
						{
							AddressLine1:   "123 Main St",
							AddressLine2:   "Apt 4B",
							City:           "Springfield",
							Region:         "IL",
							PostalCode:     "62701",
							PrimaryAddress: true,
						},
					},
				},
			},
			manualBlocks:     nil,
			automatedBlocks:  nil,
			holds:            []*models.Request{},
			loans:            []*models.Loan{},
			overdueLoans:     []*models.Loan{},
			accounts:         []*models.Account{},
			unavailableHolds: []*models.Request{},
			patronGroup:      nil,
			institutionID:    "TEST-INST",
			patronIdentifier: "666777888",
			language:         "000",
			summary:          "          ",
			valid:            true,
			pinVerified:      true,
			sequenceNumber:   "9",
			currency:         "USD",
			wantContains: []string{
				"64",
				"|AE" + "AddressTest, User",
				"|BD" + "123 Main St, Apt 4B, Springfield, IL, 62701", // Full address
				"|BLY",
				"|CQY",
			},
			wantPatronStatus: "              ",
			wantHoldCount:    "0000",
			wantOverdueCount: "0000",
			wantChargedCount: "0000",
			wantFineCount:    "0000",
		},
		{
			name: "Valid patron with patron group",
			user: &models.User{
				ID:          "user-group",
				Username:    "groupuser",
				Barcode:     "999000111",
				PatronGroup: "group-123",
				Personal: models.PersonalInfo{
					LastName:  "GroupTest",
					FirstName: "User",
				},
			},
			manualBlocks:     nil,
			automatedBlocks:  nil,
			holds:            []*models.Request{},
			loans:            []*models.Loan{},
			overdueLoans:     []*models.Loan{},
			accounts:         []*models.Account{},
			unavailableHolds: []*models.Request{},
			patronGroup: &models.PatronGroup{
				ID:    "group-123",
				Group: "Faculty",
				Desc:  "Faculty Members",
			},
			institutionID:    "TEST-INST",
			patronIdentifier: "999000111",
			language:         "000",
			summary:          "          ",
			valid:            true,
			pinVerified:      true,
			sequenceNumber:   "10",
			currency:         "USD",
			wantContains: []string{
				"64",
				"|AE" + "GroupTest, User",
				"|PC" + "group-123",       // Patron group UUID (if enabled)
				"|FU" + "Faculty",         // Patron group name (if enabled)
				"|FV" + "Faculty Members", // Patron group description (if enabled)
				"|BLY",
				"|CQY",
			},
			wantPatronStatus: "              ",
			wantHoldCount:    "0000",
			wantOverdueCount: "0000",
			wantChargedCount: "0000",
			wantFineCount:    "0000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create tenant config
			tenantConfig := &config.TenantConfig{
				Tenant:                "test-tenant",
				ErrorDetectionEnabled: true,
				MessageDelimiter:      "\r",
				FieldDelimiter:        "|",
				Currency:              tt.currency,
				Charset:               "UTF-8",
				SupportedMessages: []config.MessageSupport{
					{
						Code:    "63",
						Enabled: true,
						Fields: []config.FieldConfiguration{
							{Code: "BF", Enabled: true}, // Phone
							{Code: "BG", Enabled: true}, // Mobile phone
							{Code: "PC", Enabled: true}, // Patron group UUID
							{Code: "PB", Enabled: true}, // Birthdate
							{Code: "FU", Enabled: true}, // Patron group name
							{Code: "FV", Enabled: true}, // Patron group description
							{Code: "CD", Enabled: true}, // Unavailable holds
						},
					},
				},
			}

			// Create logger
			logger := zap.NewNop()

			// Create handler
			handler := NewPatronInformationHandler(logger, tenantConfig)

			// Create session
			session := types.NewSession("test-session", tenantConfig)

			// Build response
			response := handler.buildPatronInformationResponse(
				tt.user,
				tt.manualBlocks,
				tt.automatedBlocks,
				tt.holds,
				tt.loans,
				tt.overdueLoans,
				tt.accounts,
				tt.unavailableHolds,
				tt.patronGroup,
				tt.institutionID,
				tt.patronIdentifier,
				tt.language,
				tt.summary,
				tt.valid,
				tt.pinVerified,
				tt.sequenceNumber,
				session,
			)

			// Verify response starts with 64
			if !strings.HasPrefix(response, "64") {
				t.Errorf("Response should start with '64', got: %s", response[:10])
			}

			// Extract patron status from response (characters 2-16)
			if len(response) >= 16 {
				patronStatus := response[2:16]
				if patronStatus != tt.wantPatronStatus {
					t.Errorf("Patron status mismatch:\nwant: %q\ngot:  %q", tt.wantPatronStatus, patronStatus)
				}
			}

			// Verify fixed-length counts
			// Format: 64<patron_status><language><timestamp><hold_count><overdue_count><charged_count><fine_count><recall_count><unavailable_count>
			// Patron status: 14 chars (2-16)
			// Language: 3 chars (16-19)
			// Timestamp: 18 chars (19-37)
			// Hold count: 4 chars (37-41)
			// Overdue count: 4 chars (41-45)
			// Charged count: 4 chars (45-49)
			// Fine count: 4 chars (49-53)
			if len(response) >= 53 {
				holdCount := response[37:41]
				overdueCount := response[41:45]
				chargedCount := response[45:49]
				fineCount := response[49:53]

				if tt.wantHoldCount != "" && holdCount != tt.wantHoldCount {
					t.Errorf("Hold count mismatch: want %q, got %q", tt.wantHoldCount, holdCount)
				}
				if tt.wantOverdueCount != "" && overdueCount != tt.wantOverdueCount {
					t.Errorf("Overdue count mismatch: want %q, got %q", tt.wantOverdueCount, overdueCount)
				}
				if tt.wantChargedCount != "" && chargedCount != tt.wantChargedCount {
					t.Errorf("Charged count mismatch: want %q, got %q", tt.wantChargedCount, chargedCount)
				}
				if tt.wantFineCount != "" && fineCount != tt.wantFineCount {
					t.Errorf("Fine count mismatch: want %q, got %q", tt.wantFineCount, fineCount)
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

			// Verify response contains checksum (AZ) when error detection is enabled
			if tenantConfig.ErrorDetectionEnabled && !strings.Contains(response, "AZ") {
				t.Errorf("Response should contain checksum field (AZ), got: %s", response)
			}
		})
	}
}

// TestBuildPatronInformationResponse_ItemCounts tests that item counts are correct
func TestBuildPatronInformationResponse_ItemCounts(t *testing.T) {
	tests := []struct {
		name         string
		holds        int
		loans        int
		overdueLoans int
		accounts     int
		unavailable  int
		wantHolds    string
		wantLoans    string
		wantOverdue  string
		wantAccounts string
		wantUnavail  string
	}{
		{
			name:         "No items",
			holds:        0,
			loans:        0,
			overdueLoans: 0,
			accounts:     0,
			unavailable:  0,
			wantHolds:    "0000",
			wantLoans:    "0000",
			wantOverdue:  "0000",
			wantAccounts: "0000",
			wantUnavail:  "0000",
		},
		{
			name:         "Single items",
			holds:        1,
			loans:        1,
			overdueLoans: 1,
			accounts:     1,
			unavailable:  1,
			wantHolds:    "0001",
			wantLoans:    "0001",
			wantOverdue:  "0001",
			wantAccounts: "0001",
			wantUnavail:  "0001",
		},
		{
			name:         "Multiple items",
			holds:        5,
			loans:        10,
			overdueLoans: 3,
			accounts:     7,
			unavailable:  2,
			wantHolds:    "0005",
			wantLoans:    "0010",
			wantOverdue:  "0003",
			wantAccounts: "0007",
			wantUnavail:  "0002",
		},
		{
			name:         "Large counts",
			holds:        99,
			loans:        150,
			overdueLoans: 25,
			accounts:     50,
			unavailable:  30,
			wantHolds:    "0099",
			wantLoans:    "0150",
			wantOverdue:  "0025",
			wantAccounts: "0050",
			wantUnavail:  "0030",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tenantConfig := &config.TenantConfig{
				Tenant:                "test-tenant",
				ErrorDetectionEnabled: false,
				Charset:               "UTF-8",
			}

			logger := zap.NewNop()
			handler := NewPatronInformationHandler(logger, tenantConfig)
			session := types.NewSession("test-session", tenantConfig)

			user := &models.User{
				ID:       "user-123",
				Username: "testuser",
				Barcode:  "123456789",
				Personal: models.PersonalInfo{
					LastName:  "Test",
					FirstName: "User",
				},
			}

			// Create test data
			holds := make([]*models.Request, tt.holds)
			for i := 0; i < tt.holds; i++ {
				holds[i] = &models.Request{ID: "hold-" + string(rune(i))}
			}

			loans := make([]*models.Loan, tt.loans)
			for i := 0; i < tt.loans; i++ {
				loans[i] = &models.Loan{ID: "loan-" + string(rune(i))}
			}

			overdueLoans := make([]*models.Loan, tt.overdueLoans)
			for i := 0; i < tt.overdueLoans; i++ {
				overdueLoans[i] = &models.Loan{ID: "overdue-" + string(rune(i))}
			}

			accounts := make([]*models.Account, tt.accounts)
			for i := 0; i < tt.accounts; i++ {
				accounts[i] = &models.Account{ID: "acc-" + string(rune(i))}
			}

			unavailableHolds := make([]*models.Request, tt.unavailable)
			for i := 0; i < tt.unavailable; i++ {
				unavailableHolds[i] = &models.Request{ID: "unavail-" + string(rune(i))}
			}

			response := handler.buildPatronInformationResponse(
				user,
				nil,
				nil,
				holds,
				loans,
				overdueLoans,
				accounts,
				unavailableHolds,
				nil,
				"TEST-INST",
				"123456789",
				"000",
				"          ",
				true,
				true,
				"0",
				session,
			)

			// Extract counts from fixed-length fields
			if len(response) >= 61 {
				holdCount := response[37:41]
				overdueCount := response[41:45]
				chargedCount := response[45:49]
				fineCount := response[49:53]
				recallCount := response[53:57]
				unavailCount := response[57:61]

				if holdCount != tt.wantHolds {
					t.Errorf("Hold count: want %q, got %q", tt.wantHolds, holdCount)
				}
				if overdueCount != tt.wantOverdue {
					t.Errorf("Overdue count: want %q, got %q", tt.wantOverdue, overdueCount)
				}
				if chargedCount != tt.wantLoans {
					t.Errorf("Charged count: want %q, got %q", tt.wantLoans, chargedCount)
				}
				if fineCount != tt.wantAccounts {
					t.Errorf("Fine count: want %q, got %q", tt.wantAccounts, fineCount)
				}
				if recallCount != "0000" {
					t.Errorf("Recall count should always be 0000, got %q", recallCount)
				}
				if unavailCount != tt.wantUnavail {
					t.Errorf("Unavailable count: want %q, got %q", tt.wantUnavail, unavailCount)
				}
			} else {
				t.Errorf("Response too short to extract counts: %s", response)
			}
		})
	}
}

// TestBuildPatronInformationResponse_FeeDetails tests fee detail formatting
func TestBuildPatronInformationResponse_FeeDetails(t *testing.T) {
	tenantConfig := &config.TenantConfig{
		Tenant:                "test-tenant",
		ErrorDetectionEnabled: false,
		Currency:              "USD",
		Charset:               "UTF-8",
	}

	logger := zap.NewNop()
	handler := NewPatronInformationHandler(logger, tenantConfig)
	session := types.NewSession("test-session", tenantConfig)

	user := &models.User{
		ID:       "user-123",
		Username: "testuser",
		Barcode:  "123456789",
		Personal: models.PersonalInfo{
			LastName:  "Test",
			FirstName: "User",
		},
	}

	accounts := []*models.Account{
		{
			ID:          "acc-1",
			FeeFineType: "Overdue fine",
			Remaining:   models.FlexibleFloat(10.50),
			Title:       "The Great Gatsby",
		},
		{
			ID:          "acc-2",
			FeeFineType: "Lost item fee",
			Remaining:   models.FlexibleFloat(25.00),
			Title:       "To Kill a Mockingbird",
		},
	}

	response := handler.buildPatronInformationResponse(
		user,
		nil,
		nil,
		nil,
		nil,
		nil,
		accounts,
		nil,
		nil,
		"TEST-INST",
		"123456789",
		"000",
		"   Y      ",
		true,
		true,
		"0",
		session,
	)

	// Check that fee details are included
	if !strings.Contains(response, "|AV") {
		t.Error("Response should contain AV (fee details) field")
	}

	// Check that both accounts are included
	if !strings.Contains(response, "acc-1") {
		t.Error("Response should contain account ID acc-1")
	}
	if !strings.Contains(response, "acc-2") {
		t.Error("Response should contain account ID acc-2")
	}

	// Check that fee types are quoted
	if !strings.Contains(response, `"Overdue fine"`) {
		t.Error("Response should contain quoted fee type 'Overdue fine'")
	}
	if !strings.Contains(response, `"Lost item fee"`) {
		t.Error("Response should contain quoted fee type 'Lost item fee'")
	}

	// Check total fees
	if !strings.Contains(response, "|BV35.50") {
		t.Error("Response should contain total fees BV35.50")
	}
}

// TestBuildPatronInformationResponse_Checksum tests checksum generation
func TestBuildPatronInformationResponse_Checksum(t *testing.T) {
	tests := []struct {
		name                  string
		errorDetectionEnabled bool
		wantChecksum          bool
	}{
		{
			name:                  "With error detection enabled",
			errorDetectionEnabled: true,
			wantChecksum:          true,
		},
		{
			name:                  "With error detection disabled",
			errorDetectionEnabled: false,
			wantChecksum:          false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tenantConfig := &config.TenantConfig{
				Tenant:                "test-tenant",
				ErrorDetectionEnabled: tt.errorDetectionEnabled,
				MessageDelimiter:      "\r",
				FieldDelimiter:        "|",
				Charset:               "UTF-8",
			}

			logger := zap.NewNop()
			handler := NewPatronInformationHandler(logger, tenantConfig)
			session := types.NewSession("test-session", tenantConfig)

			user := &models.User{
				ID:       "user-123",
				Username: "testuser",
				Barcode:  "123456789",
				Personal: models.PersonalInfo{
					LastName:  "Test",
					FirstName: "User",
				},
			}

			response := handler.buildPatronInformationResponse(
				user,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				"TEST-INST",
				"123456789",
				"000",
				"          ",
				true,
				true,
				"1",
				session,
			)

			hasChecksum := strings.Contains(response, "AZ")
			if hasChecksum != tt.wantChecksum {
				if tt.wantChecksum {
					t.Errorf("Response should contain checksum (AZ) when error detection enabled")
				} else {
					t.Errorf("Response should NOT contain checksum (AZ) when error detection disabled")
				}
			}
		})
	}
}

// TestBuildPatronInformationResponse_Language tests language field handling
func TestBuildPatronInformationResponse_Language(t *testing.T) {
	tests := []struct {
		name         string
		language     string
		wantLanguage string
	}{
		{
			name:         "English",
			language:     "000",
			wantLanguage: "000",
		},
		{
			name:         "French",
			language:     "001",
			wantLanguage: "001",
		},
		{
			name:         "Empty defaults to English",
			language:     "",
			wantLanguage: "000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tenantConfig := &config.TenantConfig{
				Tenant:                "test-tenant",
				ErrorDetectionEnabled: false,
				Charset:               "UTF-8",
			}

			logger := zap.NewNop()
			handler := NewPatronInformationHandler(logger, tenantConfig)
			session := types.NewSession("test-session", tenantConfig)

			user := &models.User{
				ID:       "user-123",
				Username: "testuser",
				Barcode:  "123456789",
				Personal: models.PersonalInfo{
					LastName:  "Test",
					FirstName: "User",
				},
			}

			response := handler.buildPatronInformationResponse(
				user,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				"TEST-INST",
				"123456789",
				tt.language,
				"          ",
				true,
				true,
				"0",
				session,
			)

			// Language field is at position 16-19
			if len(response) >= 19 {
				language := response[16:19]
				if language != tt.wantLanguage {
					t.Errorf("Language mismatch: want %q, got %q", tt.wantLanguage, language)
				}
			}
		})
	}
}

// TestBuildPatronInformationResponse_SummaryControlsDetails tests that the summary field
// controls which variable field details are included, while counts are always accurate.
// This is the key behavior for the fixed fields refactor (Phase 5).
func TestBuildPatronInformationResponse_SummaryControlsDetails(t *testing.T) {
	tests := []struct {
		name            string
		summary         string
		wantContains    []string
		wantNotContains []string
		wantHoldCount   string
		wantOverdue     string
		wantCharged     string
		wantFines       string
		wantUnavail     string
	}{
		{
			name:    "Empty summary - counts present but no details",
			summary: "          ", // 10 spaces
			wantContains: []string{
				"64",   // Message code
				"|BLY", // Valid patron
				"|CQY", // Valid PIN
				"|BV",  // Total balance (always shown when accounts exist)
				"|BH",  // Currency (always shown when accounts exist)
			},
			wantNotContains: []string{
				"|AS", // No hold details
				"|AT", // No overdue details
				"|AU", // No charged details
				"|AV", // No fee details
				"|CD", // No unavailable hold details
			},
			wantHoldCount: "0003", // 3 holds
			wantOverdue:   "0002", // 2 overdue
			wantCharged:   "0005", // 5 charged
			wantFines:     "0002", // 2 accounts
			wantUnavail:   "0001", // 1 unavailable hold
		},
		{
			name:    "Full summary - all details included",
			summary: "YYYY Y    ", // Positions 0-3 (holds, overdue, charged, fines) and 5 (unavailable holds)
			wantContains: []string{
				"64",
				"|BLY",
				"|CQY",
				"|AS", // Hold details included
				"|AT", // Overdue details included
				"|AU", // Charged details included
				"|AV", // Fee details included
				"|CD", // Unavailable hold details included
				"|BV", // Total balance
				"|BH", // Currency
			},
			wantNotContains: []string{},
			wantHoldCount:   "0003",
			wantOverdue:     "0002",
			wantCharged:     "0005",
			wantFines:       "0002",
			wantUnavail:     "0001",
		},
		{
			name:    "Partial summary - only holds and fines",
			summary: "Y  Y      ", // Only positions 0 and 3
			wantContains: []string{
				"64",
				"|BLY",
				"|AS", // Hold details included
				"|AV", // Fee details included
				"|BV", // Total balance
			},
			wantNotContains: []string{
				"|AT", // No overdue details
				"|AU", // No charged details
				"|CD", // No unavailable hold details
			},
			wantHoldCount: "0003",
			wantOverdue:   "0002",
			wantCharged:   "0005",
			wantFines:     "0002",
			wantUnavail:   "0001",
		},
		{
			name:    "Partial summary - only overdue and charged",
			summary: " YY       ", // Only positions 1 and 2
			wantContains: []string{
				"64",
				"|BLY",
				"|AT", // Overdue details included
				"|AU", // Charged details included
				"|BV", // Total balance
			},
			wantNotContains: []string{
				"|AS", // No hold details
				"|AV", // No fee details
				"|CD", // No unavailable hold details
			},
			wantHoldCount: "0003",
			wantOverdue:   "0002",
			wantCharged:   "0005",
			wantFines:     "0002",
			wantUnavail:   "0001",
		},
		{
			name:    "Partial summary - only unavailable holds",
			summary: "     Y    ", // Only position 5
			wantContains: []string{
				"64",
				"|BLY",
				"|CD", // Unavailable hold details included
				"|BV", // Total balance
			},
			wantNotContains: []string{
				"|AS", // No hold details
				"|AT", // No overdue details
				"|AU", // No charged details
				"|AV", // No fee details
			},
			wantHoldCount: "0003",
			wantOverdue:   "0002",
			wantCharged:   "0005",
			wantFines:     "0002",
			wantUnavail:   "0001",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tenantConfig := &config.TenantConfig{
				Tenant:                "test-tenant",
				ErrorDetectionEnabled: false,
				Currency:              "USD",
				Charset:               "UTF-8",
				SupportedMessages: []config.MessageSupport{
					{
						Code:    "63",
						Enabled: true,
						Fields: []config.FieldConfiguration{
							{Code: "CD", Enabled: true}, // Unavailable holds
						},
					},
				},
			}

			logger := zap.NewNop()
			handler := NewPatronInformationHandler(logger, tenantConfig)
			session := types.NewSession("test-session", tenantConfig)

			user := &models.User{
				ID:       "user-123",
				Username: "testuser",
				Barcode:  "123456789",
				Personal: models.PersonalInfo{
					LastName:  "Test",
					FirstName: "User",
					Email:     "test@example.com",
				},
			}

			// Create test data with known counts
			holds := []*models.Request{
				{ID: "hold-1", Item: &models.RequestItem{Barcode: "HOLD001"}},
				{ID: "hold-2", Item: &models.RequestItem{Barcode: "HOLD002"}},
				{ID: "hold-3", Item: &models.RequestItem{Barcode: "HOLD003"}},
			}

			// 5 loans total
			loans := []*models.Loan{
				{ID: "loan-1", Item: &models.Item{Barcode: "LOAN001"}},
				{ID: "loan-2", Item: &models.Item{Barcode: "LOAN002"}},
				{ID: "loan-3", Item: &models.Item{Barcode: "LOAN003"}},
				{ID: "loan-4", Item: &models.Item{Barcode: "LOAN004"}},
				{ID: "loan-5", Item: &models.Item{Barcode: "LOAN005"}},
			}

			// 2 overdue loans (subset of loans)
			overdueLoans := []*models.Loan{
				{ID: "loan-1", Item: &models.Item{Barcode: "OVERDUE001"}},
				{ID: "loan-2", Item: &models.Item{Barcode: "OVERDUE002"}},
			}

			accounts := []*models.Account{
				{
					ID:          "acc-1",
					FeeFineType: "Overdue fine",
					Remaining:   models.FlexibleFloat(5.00),
					Title:       "Book 1",
				},
				{
					ID:          "acc-2",
					FeeFineType: "Lost item",
					Remaining:   models.FlexibleFloat(15.00),
					Title:       "Book 2",
				},
			}

			unavailableHolds := []*models.Request{
				{ID: "unavail-1", Item: &models.RequestItem{Barcode: "UNAVAIL001"}},
			}

			response := handler.buildPatronInformationResponse(
				user,
				nil,
				nil,
				holds,
				loans,
				overdueLoans,
				accounts,
				unavailableHolds,
				nil,
				"TEST-INST",
				"123456789",
				"000",
				tt.summary,
				true,
				true,
				"0",
				session,
			)

			// Verify fixed field counts are correct regardless of summary
			if len(response) >= 61 {
				holdCount := response[37:41]
				overdueCount := response[41:45]
				chargedCount := response[45:49]
				fineCount := response[49:53]
				unavailCount := response[57:61]

				if holdCount != tt.wantHoldCount {
					t.Errorf("Hold count: want %q, got %q", tt.wantHoldCount, holdCount)
				}
				if overdueCount != tt.wantOverdue {
					t.Errorf("Overdue count: want %q, got %q", tt.wantOverdue, overdueCount)
				}
				if chargedCount != tt.wantCharged {
					t.Errorf("Charged count: want %q, got %q", tt.wantCharged, chargedCount)
				}
				if fineCount != tt.wantFines {
					t.Errorf("Fine count: want %q, got %q", tt.wantFines, fineCount)
				}
				if unavailCount != tt.wantUnavail {
					t.Errorf("Unavailable count: want %q, got %q", tt.wantUnavail, unavailCount)
				}
			} else {
				t.Errorf("Response too short: %s", response)
			}

			// Check for expected strings
			for _, want := range tt.wantContains {
				if !strings.Contains(response, want) {
					t.Errorf("Response should contain %q, got: %s", want, response)
				}
			}

			// Check for strings that should NOT be present
			for _, notWant := range tt.wantNotContains {
				if strings.Contains(response, notWant) {
					t.Errorf("Response should NOT contain %q, got: %s", notWant, response)
				}
			}
		})
	}
}

// TestBuildPatronInformationResponse_FixedFieldPositions verifies the exact byte positions
// of all fixed fields in the 64 response per SIP2 specification
func TestBuildPatronInformationResponse_FixedFieldPositions(t *testing.T) {
	tenantConfig := &config.TenantConfig{
		Tenant:                "test-tenant",
		ErrorDetectionEnabled: false,
		Currency:              "USD",
		Charset:               "UTF-8",
	}

	logger := zap.NewNop()
	handler := NewPatronInformationHandler(logger, tenantConfig)
	session := types.NewSession("test-session", tenantConfig)

	user := &models.User{
		ID:       "user-123",
		Username: "testuser",
		Barcode:  "123456789",
		Personal: models.PersonalInfo{
			LastName:  "Test",
			FirstName: "User",
		},
	}

	// Create test data with specific counts for verification
	holds := make([]*models.Request, 6)
	for i := range holds {
		holds[i] = &models.Request{ID: "hold"}
	}

	loans := make([]*models.Loan, 7)
	for i := range loans {
		loans[i] = &models.Loan{ID: "loan"}
	}

	overdueLoans := make([]*models.Loan, 5)
	for i := range overdueLoans {
		overdueLoans[i] = &models.Loan{ID: "overdue"}
	}

	accounts := make([]*models.Account, 4)
	for i := range accounts {
		accounts[i] = &models.Account{ID: "acc"}
	}

	unavailableHolds := make([]*models.Request, 1)
	for i := range unavailableHolds {
		unavailableHolds[i] = &models.Request{ID: "unavail"}
	}

	response := handler.buildPatronInformationResponse(
		user,
		nil,
		nil,
		holds,
		loans,
		overdueLoans,
		accounts,
		unavailableHolds,
		nil,
		"TEST-INST",
		"123456789",
		"001", // French language
		"          ",
		true,
		true,
		"0",
		session,
	)

	// Verify minimum response length
	if len(response) < 61 {
		t.Fatalf("Response too short: %d bytes, expected at least 61", len(response))
	}

	// Verify fixed field positions per SIP2 spec
	tests := []struct {
		name  string
		start int
		end   int
		want  string
	}{
		{"Message code", 0, 2, "64"},
		{"Patron status length", 2, 16, "              "}, // 14 spaces (no blocks)
		{"Language", 16, 19, "001"},                       // French
		// Timestamp at 19-37 is variable, skip
		{"Hold items count", 37, 41, "0006"},
		{"Overdue items count", 41, 45, "0005"},
		{"Charged items count", 45, 49, "0007"},
		{"Fine items count", 49, 53, "0004"},
		{"Recall items count", 53, 57, "0000"}, // Always 0000
		{"Unavailable holds count", 57, 61, "0001"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := response[tt.start:tt.end]
			if got != tt.want {
				t.Errorf("%s: want %q, got %q (positions %d-%d)", tt.name, tt.want, got, tt.start, tt.end)
			}
		})
	}
}

// TestBuildPatronInformationResponse_CountsWithNoData verifies counts are 0000 when
// patron has no items
func TestBuildPatronInformationResponse_CountsWithNoData(t *testing.T) {
	tenantConfig := &config.TenantConfig{
		Tenant:                "test-tenant",
		ErrorDetectionEnabled: false,
		Charset:               "UTF-8",
	}

	logger := zap.NewNop()
	handler := NewPatronInformationHandler(logger, tenantConfig)
	session := types.NewSession("test-session", tenantConfig)

	user := &models.User{
		ID:       "user-123",
		Username: "testuser",
		Barcode:  "123456789",
		Personal: models.PersonalInfo{
			LastName:  "Test",
			FirstName: "User",
		},
	}

	// All empty slices
	response := handler.buildPatronInformationResponse(
		user,
		nil,
		nil,
		[]*models.Request{}, // No holds
		[]*models.Loan{},    // No loans
		[]*models.Loan{},    // No overdue
		[]*models.Account{}, // No accounts
		[]*models.Request{}, // No unavailable holds
		nil,
		"TEST-INST",
		"123456789",
		"000",
		"YYYYYY    ", // Request all details
		true,
		true,
		"0",
		session,
	)

	// All counts should be 0000
	if len(response) >= 61 {
		holdCount := response[37:41]
		overdueCount := response[41:45]
		chargedCount := response[45:49]
		fineCount := response[49:53]
		recallCount := response[53:57]
		unavailCount := response[57:61]

		if holdCount != "0000" {
			t.Errorf("Hold count should be 0000, got %q", holdCount)
		}
		if overdueCount != "0000" {
			t.Errorf("Overdue count should be 0000, got %q", overdueCount)
		}
		if chargedCount != "0000" {
			t.Errorf("Charged count should be 0000, got %q", chargedCount)
		}
		if fineCount != "0000" {
			t.Errorf("Fine count should be 0000, got %q", fineCount)
		}
		if recallCount != "0000" {
			t.Errorf("Recall count should be 0000, got %q", recallCount)
		}
		if unavailCount != "0000" {
			t.Errorf("Unavailable count should be 0000, got %q", unavailCount)
		}
	}

	// Should not contain any detail fields
	detailFields := []string{"|AS", "|AT", "|AU", "|AV", "|CD"}
	for _, field := range detailFields {
		if strings.Contains(response, field) {
			t.Errorf("Response should NOT contain %q when no data exists", field)
		}
	}

	// Should not contain fee amount or currency when no accounts
	if strings.Contains(response, "|BV") {
		t.Error("Response should NOT contain |BV when no accounts exist")
	}
	if strings.Contains(response, "|BH") {
		t.Error("Response should NOT contain |BH when no accounts exist")
	}
}

// TestBuildPatronInformationResponse_InvalidPatronAllZeroCounts verifies that invalid
// patron responses have all zero counts
func TestBuildPatronInformationResponse_InvalidPatronAllZeroCounts(t *testing.T) {
	tenantConfig := &config.TenantConfig{
		Tenant:                "test-tenant",
		ErrorDetectionEnabled: false,
		Charset:               "UTF-8",
	}

	logger := zap.NewNop()
	handler := NewPatronInformationHandler(logger, tenantConfig)
	session := types.NewSession("test-session", tenantConfig)

	// nil user = invalid patron
	response := handler.buildPatronInformationResponse(
		nil, // Invalid patron
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		"TEST-INST",
		"999999999",
		"000",
		"YYYYYY    ", // Request all details
		false,        // Not valid
		false,        // Not verified
		"0",
		session,
	)

	// All counts should be 0000
	if len(response) >= 61 {
		holdCount := response[37:41]
		overdueCount := response[41:45]
		chargedCount := response[45:49]
		fineCount := response[49:53]
		recallCount := response[53:57]
		unavailCount := response[57:61]

		if holdCount != "0000" {
			t.Errorf("Hold count should be 0000 for invalid patron, got %q", holdCount)
		}
		if overdueCount != "0000" {
			t.Errorf("Overdue count should be 0000 for invalid patron, got %q", overdueCount)
		}
		if chargedCount != "0000" {
			t.Errorf("Charged count should be 0000 for invalid patron, got %q", chargedCount)
		}
		if fineCount != "0000" {
			t.Errorf("Fine count should be 0000 for invalid patron, got %q", fineCount)
		}
		if recallCount != "0000" {
			t.Errorf("Recall count should be 0000 for invalid patron, got %q", recallCount)
		}
		if unavailCount != "0000" {
			t.Errorf("Unavailable count should be 0000 for invalid patron, got %q", unavailCount)
		}
	}

	// Patron status should be all Y (all blocks set)
	patronStatus := response[2:16]
	if patronStatus != "YYYYYYYYYYYYYY" {
		t.Errorf("Patron status should be all Y for invalid patron, got %q", patronStatus)
	}

	// Should contain BLN (invalid patron)
	if !strings.Contains(response, "|BLN") {
		t.Error("Response should contain |BLN for invalid patron")
	}
}

// TestBuildPatronInformationResponse_DetailsMatchCounts verifies that when details are
// requested, the number of detail fields matches the counts
func TestBuildPatronInformationResponse_DetailsMatchCounts(t *testing.T) {
	tenantConfig := &config.TenantConfig{
		Tenant:                "test-tenant",
		ErrorDetectionEnabled: false,
		Currency:              "USD",
		Charset:               "UTF-8",
		SupportedMessages: []config.MessageSupport{
			{
				Code:    "63",
				Enabled: true,
				Fields: []config.FieldConfiguration{
					{Code: "CD", Enabled: true},
				},
			},
		},
	}

	logger := zap.NewNop()
	handler := NewPatronInformationHandler(logger, tenantConfig)
	session := types.NewSession("test-session", tenantConfig)

	user := &models.User{
		ID:       "user-123",
		Username: "testuser",
		Barcode:  "123456789",
		Personal: models.PersonalInfo{
			LastName:  "Test",
			FirstName: "User",
		},
	}

	// Create specific counts
	holds := []*models.Request{
		{ID: "h1", Item: &models.RequestItem{Barcode: "H001"}},
		{ID: "h2", Item: &models.RequestItem{Barcode: "H002"}},
		{ID: "h3", Item: &models.RequestItem{Barcode: "H003"}},
	}

	loans := []*models.Loan{
		{ID: "l1", Item: &models.Item{Barcode: "L001"}},
		{ID: "l2", Item: &models.Item{Barcode: "L002"}},
	}

	overdueLoans := []*models.Loan{
		{ID: "o1", Item: &models.Item{Barcode: "O001"}},
	}

	accounts := []*models.Account{
		{ID: "a1", FeeFineType: "Fine1", Remaining: models.FlexibleFloat(5.0), Title: "T1"},
		{ID: "a2", FeeFineType: "Fine2", Remaining: models.FlexibleFloat(10.0), Title: "T2"},
		{ID: "a3", FeeFineType: "Fine3", Remaining: models.FlexibleFloat(15.0), Title: "T3"},
		{ID: "a4", FeeFineType: "Fine4", Remaining: models.FlexibleFloat(20.0), Title: "T4"},
	}

	unavailableHolds := []*models.Request{
		{ID: "u1", Item: &models.RequestItem{Barcode: "U001"}},
		{ID: "u2", Item: &models.RequestItem{Barcode: "U002"}},
	}

	response := handler.buildPatronInformationResponse(
		user,
		nil,
		nil,
		holds,
		loans,
		overdueLoans,
		accounts,
		unavailableHolds,
		nil,
		"TEST-INST",
		"123456789",
		"000",
		"YYYY Y    ", // Request all details: positions 0-3 and 5
		true,
		true,
		"0",
		session,
	)

	// Verify counts
	holdCount := response[37:41]
	overdueCount := response[41:45]
	chargedCount := response[45:49]
	fineCount := response[49:53]
	unavailCount := response[57:61]

	if holdCount != "0003" {
		t.Errorf("Hold count: want 0003, got %q", holdCount)
	}
	if overdueCount != "0001" {
		t.Errorf("Overdue count: want 0001, got %q", overdueCount)
	}
	if chargedCount != "0002" {
		t.Errorf("Charged count: want 0002, got %q", chargedCount)
	}
	if fineCount != "0004" {
		t.Errorf("Fine count: want 0004, got %q", fineCount)
	}
	if unavailCount != "0002" {
		t.Errorf("Unavailable count: want 0002, got %q", unavailCount)
	}

	// Count actual detail fields in response
	asCount := strings.Count(response, "|AS")
	atCount := strings.Count(response, "|AT")
	auCount := strings.Count(response, "|AU")
	avCount := strings.Count(response, "|AV")
	cdCount := strings.Count(response, "|CD")

	if asCount != 3 {
		t.Errorf("AS field count: want 3, got %d", asCount)
	}
	if atCount != 1 {
		t.Errorf("AT field count: want 1, got %d", atCount)
	}
	if auCount != 2 {
		t.Errorf("AU field count: want 2, got %d", auCount)
	}
	if avCount != 4 {
		t.Errorf("AV field count: want 4, got %d", avCount)
	}
	if cdCount != 2 {
		t.Errorf("CD field count: want 2, got %d", cdCount)
	}
}
