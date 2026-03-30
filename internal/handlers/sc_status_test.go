package handlers

import (
	"context"
	"strings"
	"testing"

	"github.com/spokanepubliclibrary/fsip2/internal/config"
	"github.com/spokanepubliclibrary/fsip2/internal/sip2/parser"
	"github.com/spokanepubliclibrary/fsip2/internal/types"
	"go.uber.org/zap"
)

func TestSCStatusHandler_Handle_Success(t *testing.T) {
	// Create test configuration
	tenantConfig := &config.TenantConfig{
		Tenant:           "test-library",
		MessageDelimiter: "\r",
		FieldDelimiter:   "|",
		OkapiURL:         "http://localhost:9130",
		Charset:          "UTF-8",
	}

	// Create logger
	logger := zap.NewNop()

	// Create handler
	handler := NewSCStatusHandler(logger, tenantConfig)

	// Create session
	session := types.NewSession("test-session-123", tenantConfig)

	// Create SC Status request message (99)
	msg := &parser.Message{
		Code:   parser.SCStatus,
		Fields: make(map[string]string),
	}

	ctx := context.Background()
	response, err := handler.Handle(ctx, msg, session)

	// Verify no error
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Verify response
	if response == "" {
		t.Error("Expected non-empty response")
	}

	// Response should start with "98" (ACS Status Response)
	if !strings.HasPrefix(response, "98") {
		t.Errorf("Expected response to start with '98', got: %s", response[:10])
	}

	// Verify response contains online status (should be Y)
	if len(response) < 4 || response[2:3] != "Y" {
		t.Errorf("Expected online status 'Y', got response: %s", response[:20])
	}
}

func TestSCStatusHandler_Handle_WithInstitutionID(t *testing.T) {
	tenantConfig := &config.TenantConfig{
		Tenant:           "test-library",
		MessageDelimiter: "\r",
		FieldDelimiter:   "|",
	}

	logger := zap.NewNop()
	handler := NewSCStatusHandler(logger, tenantConfig)

	session := types.NewSession("test-session", tenantConfig)

	msg := &parser.Message{
		Code:   parser.SCStatus,
		Fields: make(map[string]string),
	}

	ctx := context.Background()
	response, err := handler.Handle(ctx, msg, session)

	// Verify no error
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Verify response contains the tenant name as institution ID (AO is a pure echo for SC Status)
	if !strings.Contains(response, "AOtest-library") {
		t.Errorf("Response should contain tenant name as institution ID, got: %s", response)
	}
}

func TestSCStatusHandler_Handle_WithSequenceNumber(t *testing.T) {
	tenantConfig := &config.TenantConfig{
		Tenant:                "test-library",
		MessageDelimiter:      "\r",
		FieldDelimiter:        "|",
		ErrorDetectionEnabled: true,
		Charset:               "UTF-8",
	}

	logger := zap.NewNop()
	handler := NewSCStatusHandler(logger, tenantConfig)

	session := types.NewSession("test-session", tenantConfig)

	msg := &parser.Message{
		Code:           parser.SCStatus,
		Fields:         make(map[string]string),
		SequenceNumber: "7",
	}

	ctx := context.Background()
	response, err := handler.Handle(ctx, msg, session)

	// Verify no error
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// When error detection is enabled, response should contain AY field
	if tenantConfig.ErrorDetectionEnabled && !strings.Contains(response, "AY7") {
		t.Errorf("Response should contain sequence number AY7, got: %s", response)
	}
}

func TestSCStatusHandler_BuildACSStatusResponse(t *testing.T) {
	tenantConfig := &config.TenantConfig{
		Tenant:           "test-library",
		MessageDelimiter: "\r",
		FieldDelimiter:   "|",
		SupportedMessages: []config.MessageSupport{
			{Code: "11", Enabled: true}, // Checkout
			{Code: "09", Enabled: true}, // Checkin
		},
	}

	logger := zap.NewNop()
	handler := NewSCStatusHandler(logger, tenantConfig)

	session := types.NewSession("test-session", tenantConfig)

	response := handler.buildACSStatusResponse(session, "0")

	// Verify response format
	if !strings.HasPrefix(response, "98") {
		t.Errorf("Expected response to start with '98', got: %s", response[:10])
	}

	// Verify status flags are set correctly
	// Format: 98<online><checkin><checkout><renewal><status_update><offline>...
	// online=Y, checkin=Y (enabled), checkout=Y (enabled), renewal=N (always), status_update=N (default), offline=N (default)
	expectedFlags := "YYYNNN"
	if len(response) < 8 || response[2:8] != expectedFlags {
		t.Errorf("Expected flags %s, got: %s", expectedFlags, response[2:min(8, len(response))])
	}

	// Verify contains institution ID field
	if !strings.Contains(response, "AOtest-library") {
		t.Errorf("Response should contain institution ID field, got: %s", response)
	}

	// Verify contains library name field
	if !strings.Contains(response, "AMtest-library") {
		t.Errorf("Response should contain library name field, got: %s", response)
	}

	// Verify contains supported messages field (BX) - should be NYYNNNNNNNNNNNNN based on config
	if !strings.Contains(response, "BXNYYNNNNNNNNNNNNN") {
		t.Errorf("Response should contain supported messages field based on config, got: %s", response)
	}

	// Verify protocol version is present (2.00)
	if !strings.Contains(response, "2.00") {
		t.Errorf("Response should contain protocol version 2.00, got: %s", response)
	}

	// Verify timeout and retries use defaults (030 and 003)
	if !strings.Contains(response, "030003") {
		t.Errorf("Response should contain default timeout (030) and retries (003), got: %s", response)
	}
}

func TestSCStatusHandler_BuildACSStatusResponse_WithCustomInstitution(t *testing.T) {
	tenantConfig := &config.TenantConfig{
		Tenant:           "main-library",
		MessageDelimiter: "\r",
		FieldDelimiter:   "|",
	}

	logger := zap.NewNop()
	handler := NewSCStatusHandler(logger, tenantConfig)

	session := types.NewSession("test-session", tenantConfig)

	response := handler.buildACSStatusResponse(session, "1")

	// Verify AO uses TenantConfig.Tenant (SC Status has no incoming AO to echo)
	if !strings.Contains(response, "AOmain-library") {
		t.Errorf("Response should use tenant name as institution ID, got: %s", response)
	}

	// Verify library name uses tenant config
	if !strings.Contains(response, "AMmain-library") {
		t.Errorf("Response should contain library name from tenant config, got: %s", response)
	}
}

func TestSCStatusHandler_BuildACSStatusResponse_DefaultInstitution(t *testing.T) {
	tenantConfig := &config.TenantConfig{
		Tenant:           "default-library",
		MessageDelimiter: "\r",
		FieldDelimiter:   "|",
	}

	logger := zap.NewNop()
	handler := NewSCStatusHandler(logger, tenantConfig)

	session := types.NewSession("test-session", tenantConfig)
	// Don't set institution ID - should fall back to tenant name

	response := handler.buildACSStatusResponse(session, "0")

	// Should use tenant name as institution ID
	if !strings.Contains(response, "AOdefault-library") {
		t.Errorf("Response should use tenant as default institution ID, got: %s", response)
	}
}

func TestSCStatusHandler_OfflineMode(t *testing.T) {
	tenantConfig := &config.TenantConfig{
		Tenant:           "test-library",
		MessageDelimiter: "\r",
		FieldDelimiter:   "|",
	}

	logger := zap.NewNop()
	handler := NewSCStatusHandler(logger, tenantConfig)

	session := types.NewSession("test-session", tenantConfig)

	response := handler.buildACSStatusResponse(session, "0")

	// Verify offline mode is disabled (position 7, should be 'N')
	// Format: 98<online><checkin><checkout><renewal><status_update><offline>
	if len(response) >= 8 && response[7:8] != "N" {
		t.Errorf("Expected offline mode to be 'N', got: %s", response[7:8])
	}
}

func TestSCStatusHandler_SupportedMessages(t *testing.T) {
	tests := []struct {
		name              string
		supportedMessages []config.MessageSupport
		expectedBX        string
	}{
		{
			name:              "no messages configured",
			supportedMessages: []config.MessageSupport{},
			expectedBX:        "BXNNNNNNNNNNNNNNNN",
		},
		{
			name: "only checkout enabled",
			supportedMessages: []config.MessageSupport{
				{Code: "11", Enabled: true}, // Checkout
			},
			expectedBX: "BXNYNNNNNNNNNNNNNN",
		},
		{
			name: "common configuration",
			supportedMessages: []config.MessageSupport{
				{Code: "23", Enabled: false}, // Patron Status Request - Position 1: N
				{Code: "11", Enabled: true},  // Checkout - Position 2: Y
				{Code: "09", Enabled: true},  // Checkin - Position 3: Y
				{Code: "99", Enabled: true},  // SC/ACS Status - Position 5: Y
				{Code: "97", Enabled: true},  // Request Resend - Position 6: Y
				{Code: "93", Enabled: true},  // Login - Position 7: Y
				{Code: "63", Enabled: true},  // Patron Information - Position 8: Y
				{Code: "35", Enabled: true},  // End Patron Session - Position 9: Y
				{Code: "37", Enabled: true},  // Fee Paid - Position 10: Y
			},
			expectedBX: "BXNYYNYYYYYYNNNNNN", // N Y Y N(block patron always N) Y Y Y Y Y Y N N N N N N
		},
		{
			name: "all messages enabled",
			supportedMessages: []config.MessageSupport{
				{Code: "23", Enabled: true},
				{Code: "11", Enabled: true},
				{Code: "09", Enabled: true},
				{Code: "01", Enabled: true}, // Block Patron - always N
				{Code: "99", Enabled: true},
				{Code: "97", Enabled: true},
				{Code: "93", Enabled: true},
				{Code: "63", Enabled: true},
				{Code: "35", Enabled: true},
				{Code: "37", Enabled: true},
				{Code: "17", Enabled: true},
				{Code: "19", Enabled: true},
				{Code: "25", Enabled: true}, // Patron Enable - always N
				{Code: "15", Enabled: true},
				{Code: "29", Enabled: true},
				{Code: "65", Enabled: true},
			},
			expectedBX: "BXYYYNYYYYYYYYNYYY", // Y Y Y N(block patron) Y Y Y Y Y Y Y Y N(patron enable) Y Y Y
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tenantConfig := &config.TenantConfig{
				Tenant:            "test-library",
				MessageDelimiter:  "\r",
				FieldDelimiter:    "|",
				SupportedMessages: tt.supportedMessages,
			}

			logger := zap.NewNop()
			handler := NewSCStatusHandler(logger, tenantConfig)
			session := types.NewSession("test-session", tenantConfig)

			response := handler.buildACSStatusResponse(session, "0")

			if !strings.Contains(response, tt.expectedBX) {
				t.Errorf("Expected BX field %s, got response: %s", tt.expectedBX, response)
			}
		})
	}
}

// Helper function for min of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func TestSCStatusHandler_TimeoutAndRetries(t *testing.T) {
	tests := []struct {
		name            string
		timeoutPeriod   int
		retriesAllowed  int
		expectedTimeout string
		expectedRetries string
	}{
		{
			name:            "default values",
			timeoutPeriod:   0,
			retriesAllowed:  0,
			expectedTimeout: "030",
			expectedRetries: "003",
		},
		{
			name:            "custom timeout 5 seconds",
			timeoutPeriod:   5,
			retriesAllowed:  0,
			expectedTimeout: "005",
			expectedRetries: "003",
		},
		{
			name:            "custom retries 10",
			timeoutPeriod:   0,
			retriesAllowed:  10,
			expectedTimeout: "030",
			expectedRetries: "010",
		},
		{
			name:            "custom both timeout 120 and retries 5",
			timeoutPeriod:   120,
			retriesAllowed:  5,
			expectedTimeout: "120",
			expectedRetries: "005",
		},
		{
			name:            "maximum values 999",
			timeoutPeriod:   999,
			retriesAllowed:  999,
			expectedTimeout: "999",
			expectedRetries: "999",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tenantConfig := &config.TenantConfig{
				Tenant:         "test-library",
				TimeoutPeriod:  tt.timeoutPeriod,
				RetriesAllowed: tt.retriesAllowed,
			}

			logger := zap.NewNop()
			handler := NewSCStatusHandler(logger, tenantConfig)
			session := types.NewSession("test-session", tenantConfig)

			response := handler.buildACSStatusResponse(session, "0")

			// Check for timeout and retries in the response
			expectedCombined := tt.expectedTimeout + tt.expectedRetries
			if !strings.Contains(response, expectedCombined) {
				t.Errorf("Expected timeout %s and retries %s in response, got: %s", tt.expectedTimeout, tt.expectedRetries, response)
			}
		})
	}
}

func TestSCStatusHandler_StatusUpdateAndOfflineFlags(t *testing.T) {
	tests := []struct {
		name                string
		statusUpdateOk      bool
		offlineOk           bool
		expectedStatusFlag  string
		expectedOfflineFlag string
	}{
		{
			name:                "both disabled (default)",
			statusUpdateOk:      false,
			offlineOk:           false,
			expectedStatusFlag:  "N",
			expectedOfflineFlag: "N",
		},
		{
			name:                "status update enabled",
			statusUpdateOk:      true,
			offlineOk:           false,
			expectedStatusFlag:  "Y",
			expectedOfflineFlag: "N",
		},
		{
			name:                "offline enabled",
			statusUpdateOk:      false,
			offlineOk:           true,
			expectedStatusFlag:  "N",
			expectedOfflineFlag: "Y",
		},
		{
			name:                "both enabled",
			statusUpdateOk:      true,
			offlineOk:           true,
			expectedStatusFlag:  "Y",
			expectedOfflineFlag: "Y",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tenantConfig := &config.TenantConfig{
				Tenant:         "test-library",
				StatusUpdateOk: tt.statusUpdateOk,
				OfflineOk:      tt.offlineOk,
			}

			logger := zap.NewNop()
			handler := NewSCStatusHandler(logger, tenantConfig)
			session := types.NewSession("test-session", tenantConfig)

			response := handler.buildACSStatusResponse(session, "0")

			// Response format: 98<online><checkin><checkout><renewal><status_update><offline>...
			// Position 6 (index 6): status_update
			// Position 7 (index 7): offline
			if len(response) >= 8 {
				statusFlag := string(response[6])
				offlineFlag := string(response[7])

				if statusFlag != tt.expectedStatusFlag {
					t.Errorf("Expected status update flag '%s', got '%s' in response: %s", tt.expectedStatusFlag, statusFlag, response[:20])
				}
				if offlineFlag != tt.expectedOfflineFlag {
					t.Errorf("Expected offline flag '%s', got '%s' in response: %s", tt.expectedOfflineFlag, offlineFlag, response[:20])
				}
			} else {
				t.Errorf("Response too short to check flags: %s", response)
			}
		})
	}
}

func TestSCStatusHandler_TerminalLocation(t *testing.T) {
	tests := []struct {
		name            string
		setLocationCode bool
		locationCode    string
		expectANField   bool
	}{
		{
			name:            "no location code set",
			setLocationCode: false,
			expectANField:   false,
		},
		{
			name:            "location code set",
			setLocationCode: true,
			locationCode:    "DESK-01",
			expectANField:   true,
		},
		{
			name:            "location code UUID format",
			setLocationCode: true,
			locationCode:    "f1c4dae1-e71a-473d-b565-38203c017dd0",
			expectANField:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tenantConfig := &config.TenantConfig{
				Tenant: "test-library",
			}

			logger := zap.NewNop()
			handler := NewSCStatusHandler(logger, tenantConfig)
			session := types.NewSession("test-session", tenantConfig)

			if tt.setLocationCode {
				session.SetLocationCode(tt.locationCode)
			}

			response := handler.buildACSStatusResponse(session, "0")

			if tt.expectANField {
				expectedAN := "AN" + tt.locationCode
				if !strings.Contains(response, expectedAN) {
					t.Errorf("Expected AN field with location code '%s' in response, got: %s", tt.locationCode, response)
				}
			} else {
				// If no location code is set, AN field should not be present or be empty
				if strings.Contains(response, "|AN") && !strings.Contains(response, "|AN|") {
					// AN field exists and has a value
					t.Errorf("Did not expect AN field with value in response when no location code set, got: %s", response)
				}
			}
		})
	}
}

func TestSCStatusHandler_CompleteResponse(t *testing.T) {
	// Test a complete response with all configuration options set
	tenantConfig := &config.TenantConfig{
		Tenant:                "spokane",
		ErrorDetectionEnabled: true, // Enable error detection for AY/AZ fields
		MessageDelimiter:      "\r",
		FieldDelimiter:        "|",
		TimeoutPeriod:         50,
		RetriesAllowed:        3,
		StatusUpdateOk:        false,
		OfflineOk:             false,
		SupportedMessages: []config.MessageSupport{
			{Code: "23", Enabled: false}, // Patron Status Request
			{Code: "11", Enabled: true},  // Checkout
			{Code: "09", Enabled: true},  // Checkin
			{Code: "99", Enabled: true},  // SC/ACS Status
			{Code: "97", Enabled: true},  // Request Resend
			{Code: "93", Enabled: true},  // Login
			{Code: "63", Enabled: true},  // Patron Information
			{Code: "35", Enabled: true},  // End Patron Session
			{Code: "37", Enabled: true},  // Fee Paid
		},
	}

	logger := zap.NewNop()
	handler := NewSCStatusHandler(logger, tenantConfig)
	session := types.NewSession("test-session", tenantConfig)
	session.SetLocationCode("f1c4dae1-e71a-473d-b565-38203c017dd0")

	response := handler.buildACSStatusResponse(session, "1")

	// Verify all components
	t.Run("verify message code", func(t *testing.T) {
		if !strings.HasPrefix(response, "98") {
			t.Errorf("Expected response to start with '98', got: %s", response[:2])
		}
	})

	t.Run("verify status flags", func(t *testing.T) {
		// Online=Y, Checkin=Y, Checkout=Y, Renewal=N, StatusUpdate=N, Offline=N
		expectedFlags := "YYYNNN"
		if !strings.Contains(response[2:8], expectedFlags) {
			t.Errorf("Expected flags %s, got: %s", expectedFlags, response[2:8])
		}
	})

	t.Run("verify timeout and retries", func(t *testing.T) {
		if !strings.Contains(response, "050003") {
			t.Errorf("Expected timeout 050 and retries 003, got response: %s", response)
		}
	})

	t.Run("verify protocol version", func(t *testing.T) {
		if !strings.Contains(response, "2.00") {
			t.Errorf("Expected protocol version 2.00 in response: %s", response)
		}
	})

	t.Run("verify institution ID", func(t *testing.T) {
		if !strings.Contains(response, "AOspokane") {
			t.Errorf("Expected institution ID 'spokane' in response: %s", response)
		}
	})

	t.Run("verify library name", func(t *testing.T) {
		if !strings.Contains(response, "AMspokane") {
			t.Errorf("Expected library name 'spokane' in response: %s", response)
		}
	})

	t.Run("verify supported messages", func(t *testing.T) {
		if !strings.Contains(response, "BXNYYNYYYYYYNNNNNN") {
			t.Errorf("Expected BX field NYYNYYYYYYNNNNNN in response: %s", response)
		}
	})

	t.Run("verify terminal location", func(t *testing.T) {
		if !strings.Contains(response, "ANf1c4dae1-e71a-473d-b565-38203c017dd0") {
			t.Errorf("Expected terminal location in AN field in response: %s", response)
		}
	})
}
