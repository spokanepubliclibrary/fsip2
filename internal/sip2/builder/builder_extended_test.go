package builder

import (
	"strings"
	"testing"
	"time"

	"github.com/spokanepubliclibrary/fsip2/internal/config"
	"github.com/spokanepubliclibrary/fsip2/internal/sip2/parser"
)

// TestBuildLoginResponse tests login response building (message 94)
func TestBuildLoginResponse(t *testing.T) {
	cfg := &config.TenantConfig{
		ErrorDetectionEnabled: false,
		MessageDelimiter:      "\r",
		FieldDelimiter:        "|",
		Timezone:              "America/New_York",
	}

	builder := NewResponseBuilder(cfg)

	t.Run("Login successful", func(t *testing.T) {
		response, err := builder.BuildLoginResponse(true, "0")
		if err != nil {
			t.Fatalf("BuildLoginResponse() error = %v", err)
		}
		if !strings.HasPrefix(response, "941") {
			t.Errorf("Expected 941, got: %s", response[:3])
		}
		if !strings.HasSuffix(response, cfg.MessageDelimiter) {
			t.Error("Expected message delimiter at end")
		}
	})

	t.Run("Login failed", func(t *testing.T) {
		response, err := builder.BuildLoginResponse(false, "0")
		if err != nil {
			t.Fatalf("BuildLoginResponse() error = %v", err)
		}
		if !strings.HasPrefix(response, "940") {
			t.Errorf("Expected 940, got: %s", response[:3])
		}
	})
}

// TestBuildPatronStatusResponse tests patron status response building (message 24)
func TestBuildPatronStatusResponse(t *testing.T) {
	cfg := &config.TenantConfig{
		ErrorDetectionEnabled: false,
		MessageDelimiter:      "\r",
		FieldDelimiter:        "|",
		Timezone:              "America/New_York",
	}

	builder := NewResponseBuilder(cfg)
	transactionDate := time.Date(2025, 1, 10, 14, 30, 0, 0, time.UTC)

	response, err := builder.BuildPatronStatusResponse(
		"              ", // 14 spaces
		"000",
		transactionDate,
		"test-inst",
		"patron123",
		"Doe, John",
		true,
		true,
		"USD",
		"5.50",
		"0",
	)

	if err != nil {
		t.Fatalf("BuildPatronStatusResponse() error = %v", err)
	}

	if !strings.HasPrefix(response, "24") {
		t.Errorf("Expected 24 prefix")
	}
	if !strings.Contains(response, "AOtest-inst") {
		t.Error("Missing institution ID")
	}
	if !strings.Contains(response, "AApatron123") {
		t.Error("Missing patron ID")
	}
	if !strings.Contains(response, "AEDoe, John") {
		t.Error("Missing personal name")
	}
	if !strings.Contains(response, "BLY") {
		t.Error("Missing valid patron field")
	}
}

// TestBuildCheckoutResponse tests checkout response (message 12)
func TestBuildCheckoutResponse(t *testing.T) {
	cfg := &config.TenantConfig{
		ErrorDetectionEnabled: false,
		MessageDelimiter:      "\r",
		FieldDelimiter:        "|",
		Timezone:              "America/New_York",
	}

	builder := NewResponseBuilder(cfg)
	transactionDate := time.Date(2025, 1, 10, 14, 30, 0, 0, time.UTC)
	dueDate := time.Date(2025, 1, 24, 23, 59, 59, 0, time.UTC)

	t.Run("Successful checkout", func(t *testing.T) {
		response, err := builder.BuildCheckoutResponse(
			true,  // ok
			true,  // renewalOK
			false, // magneticMedia
			false, // desensitize
			transactionDate,
			"test-inst",
			"patron123",
			"ITEM001",
			"The Great Gatsby",
			dueDate,
			"",    // feeType
			false, // securityInhibit
			"USD",
			"",
			"001", // mediaType
			"",    // itemProperties
			"",    // transactionID
			[]string{},
			[]string{},
			"0",
		)

		if err != nil {
			t.Fatalf("BuildCheckoutResponse() error = %v", err)
		}

		if !strings.HasPrefix(response, "121") {
			t.Errorf("Expected 121 prefix, got %s", response[:3])
		}
		if !strings.Contains(response, "ABITEM001") {
			t.Error("Missing item ID")
		}
		if !strings.Contains(response, "AJThe Great Gatsby") {
			t.Error("Missing title")
		}
	})

	t.Run("Failed checkout", func(t *testing.T) {
		response, err := builder.BuildCheckoutResponse(
			false, // ok
			false, // renewalOK
			false, false,
			transactionDate,
			"test-inst",
			"patron123",
			"ITEM002",
			"Unavailable Book",
			dueDate,
			"", false, "USD", "", "001", "", "",
			[]string{"Item not available"},
			[]string{},
			"0",
		)

		if err != nil {
			t.Fatalf("BuildCheckoutResponse() error = %v", err)
		}

		if !strings.HasPrefix(response, "120") {
			t.Errorf("Expected 120 prefix (failed)")
		}
		if !strings.Contains(response, "AFItem not available") {
			t.Error("Missing screen message")
		}
	})
}

// TestBuildRenewResponse tests renew response (message 30)
func TestBuildRenewResponse(t *testing.T) {
	cfg := &config.TenantConfig{
		ErrorDetectionEnabled: false,
		MessageDelimiter:      "\r",
		FieldDelimiter:        "|",
		Timezone:              "America/New_York",
	}

	builder := NewResponseBuilder(cfg)
	transactionDate := time.Date(2025, 1, 10, 14, 30, 0, 0, time.UTC)
	dueDate := time.Date(2025, 1, 24, 23, 59, 59, 0, time.UTC)

	response, err := builder.BuildRenewResponse(
		true,  // ok
		true,  // renewalOK
		false, // magneticMedia
		false, // desensitize
		transactionDate,
		"test-inst",
		"patron123",
		"ITEM001",
		"Renewed Book",
		dueDate,
		[]string{},
		[]string{},
		"0",
	)

	if err != nil {
		t.Fatalf("BuildRenewResponse() error = %v", err)
	}

	if !strings.HasPrefix(response, "301") {
		t.Errorf("Expected 301 prefix")
	}
	if !strings.Contains(response, "ABITEM001") {
		t.Error("Missing item ID")
	}
	if !strings.Contains(response, "AJRenewed Book") {
		t.Error("Missing title")
	}
}

// TestBuildCheckinResponse tests checkin response (message 10)
func TestBuildCheckinResponse(t *testing.T) {
	cfg := &config.TenantConfig{
		ErrorDetectionEnabled: false,
		MessageDelimiter:      "\r",
		FieldDelimiter:        "|",
		Timezone:              "America/New_York",
	}

	builder := NewResponseBuilder(cfg)
	transactionDate := time.Date(2025, 1, 10, 14, 30, 0, 0, time.UTC)

	response, err := builder.BuildCheckinResponse(
		true,  // ok
		true,  // resensitize
		false, // magneticMedia
		false, // alert
		transactionDate,
		"test-inst",
		"ITEM001",
		"MAIN-STACKS",  // permanentLocation
		"MAIN-STACKS",  // currentLocation
		"Returned Book",
		"Book",         // materialType
		"001",          // mediaType
		"PS123.A1",     // callNumber
		"",             // alertType
		"",             // destinationLocation
		"",             // sortBin
		"",             // patronID
		"",             // itemProperties
		[]string{},     // checkinNotes
		"",             // holdShelfExpiration
		"",             // requestorName
		[]string{},     // screenMessage
		[]string{},     // printLine
		"0",
	)

	if err != nil {
		t.Fatalf("BuildCheckinResponse() error = %v", err)
	}

	if !strings.HasPrefix(response, "101") {
		t.Errorf("Expected 101 prefix")
	}
	if !strings.Contains(response, "ABITEM001") {
		t.Error("Missing item ID")
	}
	if !strings.Contains(response, "AJReturned Book") {
		t.Error("Missing title")
	}
}

// TestBuildACSStatusResponse tests ACS status response (message 98)
func TestBuildACSStatusResponse(t *testing.T) {
	cfg := &config.TenantConfig{
		ErrorDetectionEnabled: false,
		MessageDelimiter:      "\r",
		FieldDelimiter:        "|",
		Timezone:              "America/New_York",
	}

	builder := NewResponseBuilder(cfg)

	t.Run("All services online", func(t *testing.T) {
		response, err := builder.BuildACSStatusResponse(
			true,  // onlineStatus
			true,  // checkinOK
			true,  // checkoutOK
			true,  // renewalPolicy
			true,  // statusUpdateOK
			true,  // offlineOK
			30,    // timeoutPeriod
			3,     // retriesAllowed
			time.Now(),
			"2.00",
			"test-inst",
			"Test Library",
			"YNYNYNYNYNYNYNN",
			"CIRC-DESK",
			[]string{},
			[]string{},
			"0",
		)

		if err != nil {
			t.Fatalf("BuildACSStatusResponse() error = %v", err)
		}

		if !strings.HasPrefix(response, "98Y") {
			t.Errorf("Expected 98Y prefix (online)")
		}
		if !strings.Contains(response, "BXYNYNYNYNYNYNYNN") {
			t.Error("Missing supported messages")
		}
	})

	t.Run("Services offline", func(t *testing.T) {
		response, err := builder.BuildACSStatusResponse(
			false, false, false, false, false, false,
			0, 0,
			time.Now(),
			"2.00",
			"test-inst",
			"Test Library",
			"NNNNNNNNNNNNNNN",
			"",
			[]string{},
			[]string{},
			"0",
		)

		if err != nil {
			t.Fatalf("BuildACSStatusResponse() error = %v", err)
		}

		if !strings.HasPrefix(response, "98N") {
			t.Errorf("Expected 98N prefix (offline)")
		}
	})
}

// TestBuildPatronInformationResponse tests patron info response (message 64)
func TestBuildPatronInformationResponse(t *testing.T) {
	cfg := &config.TenantConfig{
		ErrorDetectionEnabled: false,
		MessageDelimiter:      "\r",
		FieldDelimiter:        "|",
		Timezone:              "America/New_York",
	}

	builder := NewResponseBuilder(cfg)
	transactionDate := time.Date(2025, 1, 10, 14, 30, 0, 0, time.UTC)

	response, err := builder.BuildPatronInformationResponse(
		"              ", // patronStatus (14 chars)
		"000",           // language
		transactionDate,
		2, // holdItemsCount
		1, // overdueItemsCount
		3, // chargedItemsCount
		1, // fineItemsCount
		0, // recallItemsCount
		0, // unavailableHoldsCount
		"test-inst",
		"patron123",
		"Doe, John",
		0,    // holdItemsLimit
		0,    // overdueItemsLimit
		0,    // chargedItemsLimit
		true, // validPatron
		true, // validPatronPassword
		"USD",
		"5.00",
		"",   // feeLimit
		[]string{"HOLD001", "HOLD002"},
		[]string{"OVERDUE001"},
		[]string{"LOAN001", "LOAN002", "LOAN003"},
		[]string{"FEE001"},
		[]string{},
		[]string{},
		"",          // homeAddress
		"",          // emailAddress
		"",          // homePhoneNumber
		[]string{},  // screenMessage
		[]string{},  // printLine
		"0",
	)

	if err != nil {
		t.Fatalf("BuildPatronInformationResponse() error = %v", err)
	}

	if !strings.HasPrefix(response, "64") {
		t.Error("Expected 64 prefix")
	}
	if !strings.Contains(response, "ASHOLD001") {
		t.Error("Missing hold item HOLD001")
	}
	if !strings.Contains(response, "ASHOLD002") {
		t.Error("Missing hold item HOLD002")
	}
	if !strings.Contains(response, "AULOAN001") {
		t.Error("Missing charged item LOAN001")
	}
	if !strings.Contains(response, "ATOVERDUE001") {
		t.Error("Missing overdue item")
	}
	if !strings.Contains(response, "AVFEE001") {
		t.Error("Missing fine item")
	}
}

// TestBuildItemInformationResponse tests item info response (message 18)
func TestBuildItemInformationResponse(t *testing.T) {
	cfg := &config.TenantConfig{
		ErrorDetectionEnabled: false,
		MessageDelimiter:      "\r",
		FieldDelimiter:        "|",
		Timezone:              "America/New_York",
	}

	builder := NewResponseBuilder(cfg)
	transactionDate := time.Date(2025, 1, 10, 14, 30, 0, 0, time.UTC)

	response, err := builder.BuildItemInformationResponse(
		"03", // circulationStatus (Available)
		"00", // securityMarker
		"00", // feeType
		transactionDate,
		"test-inst",
		"ITEM001",
		"The Catcher in the Rye",
		"MAIN-STACKS",                                // permanentLocation
		"MAIN-STACKS",                                // currentLocation
		"20250124    235959",                         // dueDate
		"001",                                        // mediaType
		"Book",                                       // materialType
		"PS123.A1",                                   // callNumber
		"",                                           // routingLocation
		"0",                                          // holdQueueLength
		"Salinger, J.D.",                             // primaryContributor
		"",                                           // workDescription
		[]string{},                                   // isbns
		[]string{},                                   // upcs
		"",                                           // holdShelfExpiration
		"",                                           // requestorBarcode
		"",                                           // requestorName
		[]string{},                                   // screenMessage
		[]string{},                                   // printLine
		"0",
	)

	if err != nil {
		t.Fatalf("BuildItemInformationResponse() error = %v", err)
	}

	if !strings.HasPrefix(response, "1803") {
		t.Errorf("Expected 1803 prefix (circulation status 03)")
	}
	if !strings.Contains(response, "ABITEM001") {
		t.Error("Missing item ID")
	}
	if !strings.Contains(response, "AJThe Catcher in the Rye") {
		t.Error("Missing title")
	}
}

// TestBuild_WithChecksum tests checksum calculation
func TestBuild_WithChecksum(t *testing.T) {
	cfg := &config.TenantConfig{
		ErrorDetectionEnabled: true,
		MessageDelimiter:      "\r",
		FieldDelimiter:        "|",
		Charset:               "UTF-8",
		Timezone:              "America/New_York",
	}

	builder := NewResponseBuilder(cfg)

	response, err := builder.BuildLoginResponse(true, "3")
	if err != nil {
		t.Fatalf("BuildLoginResponse() error = %v", err)
	}

	if !strings.Contains(response, "AY3") {
		t.Error("Expected sequence number AY3")
	}
	if !strings.Contains(response, "AZ") {
		t.Error("Expected checksum AZ field")
	}

	// Verify checksum is 4 characters
	parts := strings.Split(response, "AZ")
	if len(parts) == 2 {
		checksum := strings.TrimSuffix(parts[1], cfg.MessageDelimiter)
		if len(checksum) != 4 {
			t.Errorf("Expected checksum to be 4 characters, got %d: %s", len(checksum), checksum)
		}
	}
}

// TestBuild_DifferentDelimiters tests different field delimiters
func TestBuild_DifferentDelimiters(t *testing.T) {
	tests := []struct {
		name             string
		messageDelimiter string
		fieldDelimiter   string
	}{
		{"Standard delimiters", "\r", "|"},
		{"Alternative delimiters", "\r\n", "|"},
		{"Custom field delimiter", "\r", "^"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.TenantConfig{
				ErrorDetectionEnabled: false,
				MessageDelimiter:      tt.messageDelimiter,
				FieldDelimiter:        tt.fieldDelimiter,
				Timezone:              "America/New_York",
			}

			builder := NewResponseBuilder(cfg)
			response, err := builder.BuildLoginResponse(true, "0")
			if err != nil {
				t.Fatalf("BuildLoginResponse() error = %v", err)
			}

			if !strings.HasSuffix(response, tt.messageDelimiter) {
				t.Errorf("Expected message delimiter %q at end", tt.messageDelimiter)
			}
		})
	}
}

// TestBuildSupportedMessagesString tests the supported messages string generation
func TestBuildSupportedMessagesString(t *testing.T) {
	t.Run("All messages supported", func(t *testing.T) {
		supportedMessages := []parser.MessageCode{
			parser.PatronStatusRequest,
			parser.CheckoutRequest,
			parser.CheckinRequest,
			parser.PatronInformationRequest,
			parser.ItemInformationRequest,
			parser.EndPatronSessionRequest,
			parser.FeePaidRequest,
			parser.ItemStatusUpdateRequest,
			parser.RenewRequest,
			parser.RenewAllRequest,
		}
		result := BuildSupportedMessagesString(supportedMessages)

		if len(result) != 16 {
			t.Errorf("Expected length = 13, got %d: %s", len(result), result)
		}

		if !strings.Contains(result, "Y") {
			t.Errorf("Expected to contain Y, got: %s", result)
		}
	})

	t.Run("No messages supported", func(t *testing.T) {
		result := BuildSupportedMessagesString([]parser.MessageCode{})

		if len(result) != 16 {
			t.Errorf("Expected length = 13, got %d: %s", len(result), result)
		}

		// All N's expected
		if strings.Count(result, "N") != 16 {
			t.Errorf("Expected all N's, got: %s", result)
		}
	})

	t.Run("Mixed support", func(t *testing.T) {
		supportedMessages := []parser.MessageCode{
			parser.PatronStatusRequest,
			parser.CheckoutRequest,
			parser.CheckinRequest,
		}
		result := BuildSupportedMessagesString(supportedMessages)

		if len(result) != 16 {
			t.Errorf("Expected length = 13, got %d: %s", len(result), result)
		}
	})
}

// TestBuild_GeneralMethod tests the generic Build method
func TestBuild_GeneralMethod(t *testing.T) {
	cfg := &config.TenantConfig{
		ErrorDetectionEnabled: false,
		MessageDelimiter:      "\r",
		FieldDelimiter:        "|",
		Timezone:              "America/New_York",
	}

	builder := NewResponseBuilder(cfg)

	t.Run("Simple message", func(t *testing.T) {
		response, err := builder.Build(parser.LoginResponse, "1", "0")
		if err != nil {
			t.Fatalf("Build() error = %v", err)
		}

		if !strings.HasPrefix(response, "941") {
			t.Errorf("Expected 941 prefix, got: %s", response[:3])
		}

		if !strings.HasSuffix(response, cfg.MessageDelimiter) {
			t.Error("Expected message delimiter at end")
		}
	})

	t.Run("Message with fields", func(t *testing.T) {
		content := "              00020250110    143000AOtest|AApatron|"
		response, err := builder.Build(parser.PatronStatusResponse, content, "1")
		if err != nil {
			t.Fatalf("Build() error = %v", err)
		}

		if !strings.HasPrefix(response, "24") {
			t.Errorf("Expected 24 prefix")
		}
	})
}

// TestBuildPatronStatusString tests the standalone patron status string builder
func TestBuildPatronStatusString(t *testing.T) {
	t.Run("All false returns 14 spaces", func(t *testing.T) {
		result := BuildPatronStatusString(
			false, false, false, false, false, false, false,
			false, false, false, false, false, false, false,
		)
		if len(result) != 14 {
			t.Fatalf("Expected length 14, got %d", len(result))
		}
		for i, c := range result {
			if c != ' ' {
				t.Errorf("Position %d: expected ' ', got '%c'", i, c)
			}
		}
	})

	t.Run("All true returns 14 Y's", func(t *testing.T) {
		result := BuildPatronStatusString(
			true, true, true, true, true, true, true,
			true, true, true, true, true, true, true,
		)
		if len(result) != 14 {
			t.Fatalf("Expected length 14, got %d", len(result))
		}
		for i, c := range result {
			if c != 'Y' {
				t.Errorf("Position %d: expected 'Y', got '%c'", i, c)
			}
		}
	})

	t.Run("Individual flags set correct positions", func(t *testing.T) {
		flags := []bool{
			true, false, false, false, false, false, false,
			false, false, false, false, false, false, false,
		}
		result := BuildPatronStatusString(
			flags[0], flags[1], flags[2], flags[3], flags[4], flags[5], flags[6],
			flags[7], flags[8], flags[9], flags[10], flags[11], flags[12], flags[13],
		)
		if result[0] != 'Y' {
			t.Errorf("Position 0 (chargePrivilegesDenied): expected 'Y', got '%c'", result[0])
		}
		for i := 1; i < 14; i++ {
			if result[i] != ' ' {
				t.Errorf("Position %d: expected ' ', got '%c'", i, result[i])
			}
		}
	})

	t.Run("Last flag sets position 13", func(t *testing.T) {
		result := BuildPatronStatusString(
			false, false, false, false, false, false, false,
			false, false, false, false, false, false, true,
		)
		if result[13] != 'Y' {
			t.Errorf("Position 13 (tooManyItemsBilled): expected 'Y', got '%c'", result[13])
		}
		for i := 0; i < 13; i++ {
			if result[i] != ' ' {
				t.Errorf("Position %d: expected ' ', got '%c'", i, result[i])
			}
		}
	})
}
