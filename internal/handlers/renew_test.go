package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/spokanepubliclibrary/fsip2/internal/config"
	"github.com/spokanepubliclibrary/fsip2/internal/folio/models"
	"github.com/spokanepubliclibrary/fsip2/internal/sip2/parser"
	"github.com/spokanepubliclibrary/fsip2/internal/types"
	"go.uber.org/zap"
)

// TestRenewHandler_SuccessfulRenewal tests a successful item renewal
func TestRenewHandler_SuccessfulRenewal(t *testing.T) {
	// Create mock FOLIO server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/circulation/renew-by-barcode"):
			dueDate := time.Now().Add(14 * 24 * time.Hour)
			loan := models.Loan{
				ID:      "loan-123",
				UserID:  "user-123",
				ItemID:  "item-123",
				DueDate: &dueDate,
				Status:  models.LoanStatus{Name: "Open"},
				Item: &models.Item{
					InstanceID: "instance-123",
				},
			}
			json.NewEncoder(w).Encode(loan)
		default:
			http.NotFound(w, r)
		}
	}))
	defer mockServer.Close()

	// Create tenant config
	tenantConfig := &config.TenantConfig{
		OkapiURL:         mockServer.URL,
		Tenant:           "test",
		FieldDelimiter:   "|",
		MessageDelimiter: "\r",
	}

	// Create session with authenticated user
	session := types.NewSession("test-conn", tenantConfig)
	session.SetAuthenticated("testuser", "user-123", "12345", "token-123", time.Now().Add(1*time.Hour))

	// Create handler
	logger := zap.NewNop()
	handler := NewRenewHandler(logger, tenantConfig)

	// Create SIP2 renew request message
	msg := &parser.Message{
		Code: parser.RenewRequest,
		Fields: map[string]string{
			string(parser.InstitutionID):    "TEST",
			string(parser.PatronIdentifier): "12345",
			string(parser.ItemIdentifier):   "ITEM001",
		},
	}

	// Handle request
	response, err := handler.Handle(context.Background(), msg, session)
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	// Verify response
	if !strings.HasPrefix(response, "30") {
		t.Errorf("Expected response to start with '30', got: %s", response)
	}

	// Check for renewal OK indicator
	if !strings.Contains(response, "Y") {
		t.Errorf("Expected renewal OK indicator 'Y' in response: %s", response)
	}

	// Check required fields
	if !strings.Contains(response, "AOTEST") {
		t.Errorf("Expected institution ID in response: %s", response)
	}
	if !strings.Contains(response, "AA12345") {
		t.Errorf("Expected patron identifier in response: %s", response)
	}
	if !strings.Contains(response, "ABITEM001") {
		t.Errorf("Expected item identifier in response: %s", response)
	}
}

// TestRenewHandler_RenewalFailure tests renewal failure scenarios
func TestRenewHandler_RenewalFailure(t *testing.T) {
	// Create mock FOLIO server that returns error
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/circulation/renew-by-barcode"):
			w.WriteHeader(http.StatusUnprocessableEntity)
			w.Write([]byte(`{"errors":[{"message":"Item is not renewable"}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer mockServer.Close()

	// Create tenant config
	tenantConfig := &config.TenantConfig{
		OkapiURL:         mockServer.URL,
		Tenant:           "test",
		FieldDelimiter:   "|",
		MessageDelimiter: "\r",
	}

	// Create session
	session := types.NewSession("test-conn", tenantConfig)
	session.SetAuthenticated("testuser", "user-123", "12345", "token-123", time.Now().Add(1*time.Hour))

	// Create handler
	logger := zap.NewNop()
	handler := NewRenewHandler(logger, tenantConfig)

	// Create renew request
	msg := &parser.Message{
		Code: parser.RenewRequest,
		Fields: map[string]string{
			string(parser.InstitutionID):    "TEST",
			string(parser.PatronIdentifier): "12345",
			string(parser.ItemIdentifier):   "ITEM001",
		},
	}

	// Handle request
	response, err := handler.Handle(context.Background(), msg, session)
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	// Verify response indicates failure
	if !strings.HasPrefix(response, "30") {
		t.Errorf("Expected response to start with '30', got: %s", response)
	}

	// Check for renewal failure indicator
	if !strings.Contains(response, "N") {
		t.Errorf("Expected renewal failure indicator 'N' in response: %s", response)
	}

	// Check for error message
	if !strings.Contains(response, "AF") {
		t.Errorf("Expected screen message field 'AF' in response: %s", response)
	}
}

// TestRenewHandler_MissingRequiredFields tests validation of required fields
func TestRenewHandler_MissingRequiredFields(t *testing.T) {
	tests := []struct {
		name   string
		fields map[string]string
	}{
		{
			name: "Missing institution ID",
			fields: map[string]string{
				string(parser.PatronIdentifier): "12345",
				string(parser.ItemIdentifier):   "ITEM001",
			},
		},
		{
			name: "Missing patron identifier",
			fields: map[string]string{
				string(parser.InstitutionID):  "TEST",
				string(parser.ItemIdentifier): "ITEM001",
			},
		},
		{
			name: "Missing item identifier",
			fields: map[string]string{
				string(parser.InstitutionID):    "TEST",
				string(parser.PatronIdentifier): "12345",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tenantConfig := &config.TenantConfig{
				OkapiURL:         "http://localhost",
				Tenant:           "test",
				FieldDelimiter:   "|",
				MessageDelimiter: "\r",
			}

			session := types.NewSession("test-conn", tenantConfig)
			session.SetAuthenticated("testuser", "user-123", "12345", "token-123", time.Now().Add(1*time.Hour))

			logger := zap.NewNop()
			handler := NewRenewHandler(logger, tenantConfig)

			msg := &parser.Message{
				Code:   parser.RenewRequest,
				Fields: tt.fields,
			}

			response, err := handler.Handle(context.Background(), msg, session)
			if err != nil {
				t.Fatalf("Handle() error = %v", err)
			}

			// Should return error response
			if !strings.Contains(response, "N") {
				t.Errorf("Expected failure indicator in response: %s", response)
			}
		})
	}
}

// TestRenewHandler_PatronNotFound tests renewal when patron cannot be found
func TestRenewHandler_PatronNotFound(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/users"):
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"errors":[{"message":"User not found"}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer mockServer.Close()

	tenantConfig := &config.TenantConfig{
		OkapiURL:         mockServer.URL,
		Tenant:           "test",
		FieldDelimiter:   "|",
		MessageDelimiter: "\r",
	}

	// Session without authenticated patron
	session := types.NewSession("test-conn", tenantConfig)
	session.SetAuthenticated("testuser", "", "", "token-123", time.Now().Add(1*time.Hour))

	logger := zap.NewNop()
	handler := NewRenewHandler(logger, tenantConfig)

	msg := &parser.Message{
		Code: parser.RenewRequest,
		Fields: map[string]string{
			string(parser.InstitutionID):    "TEST",
			string(parser.PatronIdentifier): "UNKNOWN",
			string(parser.ItemIdentifier):   "ITEM001",
		},
	}

	response, err := handler.Handle(context.Background(), msg, session)
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	// Should return error response
	if !strings.Contains(response, "N") {
		t.Errorf("Expected failure indicator in response: %s", response)
	}
}

// TestRenewHandler_ItemNotFound tests renewal when item cannot be found
func TestRenewHandler_ItemNotFound(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/circulation/renew-by-barcode"):
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"errors":[{"message":"No item with barcode UNKNOWN found"}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer mockServer.Close()

	tenantConfig := &config.TenantConfig{
		OkapiURL:         mockServer.URL,
		Tenant:           "test",
		FieldDelimiter:   "|",
		MessageDelimiter: "\r",
	}

	session := types.NewSession("test-conn", tenantConfig)
	session.SetAuthenticated("testuser", "user-123", "12345", "token-123", time.Now().Add(1*time.Hour))

	logger := zap.NewNop()
	handler := NewRenewHandler(logger, tenantConfig)

	msg := &parser.Message{
		Code: parser.RenewRequest,
		Fields: map[string]string{
			string(parser.InstitutionID):    "TEST",
			string(parser.PatronIdentifier): "12345",
			string(parser.ItemIdentifier):   "UNKNOWN",
		},
	}

	response, err := handler.Handle(context.Background(), msg, session)
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	// Should return error response
	if !strings.Contains(response, "N") {
		t.Errorf("Expected failure indicator in response: %s", response)
	}
}

// TestRenewHandler_MaxRenewalsReached tests renewal when max renewals reached
func TestRenewHandler_MaxRenewalsReached(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/circulation/renew-by-barcode"):
			w.WriteHeader(http.StatusUnprocessableEntity)
			w.Write([]byte(`{"errors":[{"message":"Renewal limit reached"}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer mockServer.Close()

	tenantConfig := &config.TenantConfig{
		OkapiURL:         mockServer.URL,
		Tenant:           "test",
		FieldDelimiter:   "|",
		MessageDelimiter: "\r",
	}

	session := types.NewSession("test-conn", tenantConfig)
	session.SetAuthenticated("testuser", "user-123", "12345", "token-123", time.Now().Add(1*time.Hour))

	logger := zap.NewNop()
	handler := NewRenewHandler(logger, tenantConfig)

	msg := &parser.Message{
		Code: parser.RenewRequest,
		Fields: map[string]string{
			string(parser.InstitutionID):    "TEST",
			string(parser.PatronIdentifier): "12345",
			string(parser.ItemIdentifier):   "ITEM001",
		},
	}

	response, err := handler.Handle(context.Background(), msg, session)
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	// Should return error response with renewal limit message
	if !strings.Contains(response, "N") {
		t.Errorf("Expected failure indicator in response: %s", response)
	}
	if !strings.Contains(response, "Renewal limit") {
		t.Errorf("Expected renewal limit message in response: %s", response)
	}
}

// TestRenewHandler_PatronVerificationRequired tests renewal with patron verification
func TestRenewHandler_PatronVerificationRequired(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/users"):
			// Return user with barcode
			user := models.User{
				ID:       "user-123",
				Username: "testuser",
				Barcode:  "12345",
			}
			json.NewEncoder(w).Encode(models.UserCollection{
				Users:        []models.User{user},
				TotalRecords: 1,
			})
		case strings.Contains(r.URL.Path, "/bl-users/by-id"):
			// Return user credentials without password
			composite := map[string]interface{}{
				"id":       "user-123",
				"username": "testuser",
				"password": nil,
			}
			json.NewEncoder(w).Encode(composite)
		default:
			http.NotFound(w, r)
		}
	}))
	defer mockServer.Close()

	tenantConfig := &config.TenantConfig{
		OkapiURL:                           mockServer.URL,
		Tenant:                             "test",
		FieldDelimiter:                     "|",
		MessageDelimiter:                   "\r",
		PatronPasswordVerificationRequired: true,
	}

	// Session without authenticated patron
	session := types.NewSession("test-conn", tenantConfig)
	session.SetAuthenticated("testuser", "", "", "token-123", time.Now().Add(1*time.Hour))

	logger := zap.NewNop()
	handler := NewRenewHandler(logger, tenantConfig)

	msg := &parser.Message{
		Code: parser.RenewRequest,
		Fields: map[string]string{
			string(parser.InstitutionID):    "TEST",
			string(parser.PatronIdentifier): "12345",
			string(parser.ItemIdentifier):   "ITEM001",
			// Missing patron password
		},
	}

	response, err := handler.Handle(context.Background(), msg, session)
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	// Should fail due to missing password
	if !strings.Contains(response, "N") {
		t.Errorf("Expected failure indicator in response: %s", response)
	}
}

// TestRenewHandler_WithoutPatronVerification tests renewal without verification
func TestRenewHandler_WithoutPatronVerification(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/circulation/renew-by-barcode"):
			dueDate := time.Now().Add(14 * 24 * time.Hour)
			loan := models.Loan{
				ID:      "loan-123",
				UserID:  "user-123",
				ItemID:  "item-123",
				DueDate: &dueDate,
				Status:  models.LoanStatus{Name: "Open"},
				Item: &models.Item{
					InstanceID: "instance-123",
				},
			}
			json.NewEncoder(w).Encode(loan)
		default:
			http.NotFound(w, r)
		}
	}))
	defer mockServer.Close()

	tenantConfig := &config.TenantConfig{
		OkapiURL:                           mockServer.URL,
		Tenant:                             "test",
		FieldDelimiter:                     "|",
		MessageDelimiter:                   "\r",
		PatronPasswordVerificationRequired: false, // Verification disabled
	}

	session := types.NewSession("test-conn", tenantConfig)
	session.SetAuthenticated("testuser", "user-123", "12345", "token-123", time.Now().Add(1*time.Hour))

	logger := zap.NewNop()
	handler := NewRenewHandler(logger, tenantConfig)

	msg := &parser.Message{
		Code: parser.RenewRequest,
		Fields: map[string]string{
			string(parser.InstitutionID):    "TEST",
			string(parser.PatronIdentifier): "12345",
			string(parser.ItemIdentifier):   "ITEM001",
			// No password provided, but verification disabled
		},
	}

	response, err := handler.Handle(context.Background(), msg, session)
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	// Should succeed
	if !strings.Contains(response, "Y") {
		t.Errorf("Expected success indicator in response: %s", response)
	}
}

// TestRenewHandler_ResponseFormat tests the format of renewal response
func TestRenewHandler_ResponseFormat(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/circulation/renew-by-barcode"):
			dueDate := time.Date(2025, 12, 31, 23, 59, 0, 0, time.UTC)
			loan := models.Loan{
				ID:      "loan-123",
				UserID:  "user-123",
				ItemID:  "item-123",
				DueDate: &dueDate,
				Status:  models.LoanStatus{Name: "Open"},
				Item: &models.Item{
					InstanceID: "instance-123",
				},
			}
			json.NewEncoder(w).Encode(loan)
		default:
			http.NotFound(w, r)
		}
	}))
	defer mockServer.Close()

	tenantConfig := &config.TenantConfig{
		OkapiURL:         mockServer.URL,
		Tenant:           "test",
		FieldDelimiter:   "|",
		MessageDelimiter: "\r",
	}

	session := types.NewSession("test-conn", tenantConfig)
	session.SetAuthenticated("testuser", "user-123", "12345", "token-123", time.Now().Add(1*time.Hour))

	logger := zap.NewNop()
	handler := NewRenewHandler(logger, tenantConfig)

	msg := &parser.Message{
		Code: parser.RenewRequest,
		Fields: map[string]string{
			string(parser.InstitutionID):    "TEST",
			string(parser.PatronIdentifier): "12345",
			string(parser.ItemIdentifier):   "ITEM001",
		},
		SequenceNumber: "123",
	}

	response, err := handler.Handle(context.Background(), msg, session)
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	// Verify response format
	if !strings.HasPrefix(response, "30") {
		t.Errorf("Response should start with '30', got: %s", response)
	}

	// Check for required fields
	requiredFields := []string{"AO", "AA", "AB", "AJ"}
	for _, field := range requiredFields {
		if !strings.Contains(response, field) {
			t.Errorf("Response missing required field %s: %s", field, response)
		}
	}

	// Check for sequence number
	if !strings.Contains(response, "AY123") {
		t.Errorf("Response missing sequence number: %s", response)
	}
}

// TestRenewHandler_InstanceIDFallback tests fallback when instance ID not in renewal response
func TestRenewHandler_InstanceIDFallback(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/circulation/renew-by-barcode"):
			dueDate := time.Now().Add(14 * 24 * time.Hour)
			loan := models.Loan{
				ID:      "loan-123",
				UserID:  "user-123",
				ItemID:  "item-123",
				DueDate: &dueDate,
				Status:  models.LoanStatus{Name: "Open"},
				// No Item field - instance ID missing
			}
			json.NewEncoder(w).Encode(loan)
		case strings.Contains(r.URL.Path, "/inventory/items/item-123"):
			item := models.Item{
				ID:               "item-123",
				Barcode:          "ITEM001",
				HoldingsRecordID: "holdings-123",
				InstanceID:       "", // Not set on item
			}
			json.NewEncoder(w).Encode(item)
		case strings.Contains(r.URL.Path, "/holdings-storage/holdings/holdings-123"):
			holdings := models.Holdings{
				ID:         "holdings-123",
				InstanceID: "instance-123", // Retrieved from holdings
			}
			json.NewEncoder(w).Encode(holdings)
		default:
			http.NotFound(w, r)
		}
	}))
	defer mockServer.Close()

	tenantConfig := &config.TenantConfig{
		OkapiURL:         mockServer.URL,
		Tenant:           "test",
		FieldDelimiter:   "|",
		MessageDelimiter: "\r",
	}

	session := types.NewSession("test-conn", tenantConfig)
	session.SetAuthenticated("testuser", "user-123", "12345", "token-123", time.Now().Add(1*time.Hour))

	logger := zap.NewNop()
	handler := NewRenewHandler(logger, tenantConfig)

	msg := &parser.Message{
		Code: parser.RenewRequest,
		Fields: map[string]string{
			string(parser.InstitutionID):    "TEST",
			string(parser.PatronIdentifier): "12345",
			string(parser.ItemIdentifier):   "ITEM001",
		},
	}

	response, err := handler.Handle(context.Background(), msg, session)
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	// Should succeed with instance ID retrieved from holdings
	if !strings.Contains(response, "Y") {
		t.Errorf("Expected success indicator in response: %s", response)
	}

	// Should contain instance ID in title field (AJ)
	if !strings.Contains(response, "AJinstance-123") {
		t.Errorf("Expected instance ID in title field: %s", response)
	}
}

// TestRenewHandler_DueDateFormatting tests different due date formats
func TestRenewHandler_DueDateFormatting(t *testing.T) {
	testDueDate := time.Date(2025, 6, 15, 14, 30, 0, 0, time.UTC)

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/circulation/renew-by-barcode"):
			loan := models.Loan{
				ID:      "loan-123",
				UserID:  "user-123",
				ItemID:  "item-123",
				DueDate: &testDueDate,
				Status:  models.LoanStatus{Name: "Open"},
				Item: &models.Item{
					InstanceID: "instance-123",
				},
			}
			json.NewEncoder(w).Encode(loan)
		default:
			http.NotFound(w, r)
		}
	}))
	defer mockServer.Close()

	tenantConfig := &config.TenantConfig{
		OkapiURL:         mockServer.URL,
		Tenant:           "test",
		FieldDelimiter:   "|",
		MessageDelimiter: "\r",
	}

	session := types.NewSession("test-conn", tenantConfig)
	session.SetAuthenticated("testuser", "user-123", "12345", "token-123", time.Now().Add(1*time.Hour))

	logger := zap.NewNop()
	handler := NewRenewHandler(logger, tenantConfig)

	msg := &parser.Message{
		Code: parser.RenewRequest,
		Fields: map[string]string{
			string(parser.InstitutionID):    "TEST",
			string(parser.PatronIdentifier): "12345",
			string(parser.ItemIdentifier):   "ITEM001",
		},
	}

	response, err := handler.Handle(context.Background(), msg, session)
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	// Response should include formatted due date
	if !strings.Contains(response, "20250615") {
		t.Errorf("Expected formatted due date YYYYMMDD in response: %s", response)
	}
}

// TestRenewHandler_AuthenticationError tests renewal when authentication fails
func TestRenewHandler_AuthenticationError(t *testing.T) {
	tenantConfig := &config.TenantConfig{
		OkapiURL:         "http://localhost:9999", // Invalid URL
		Tenant:           "test",
		FieldDelimiter:   "|",
		MessageDelimiter: "\r",
	}

	// Session without authenticated token (expired)
	session := types.NewSession("test-conn", tenantConfig)
	session.SetAuthenticated("testuser", "user-123", "12345", "", time.Now().Add(-1*time.Hour))

	logger := zap.NewNop()
	handler := NewRenewHandler(logger, tenantConfig)

	msg := &parser.Message{
		Code: parser.RenewRequest,
		Fields: map[string]string{
			string(parser.InstitutionID):    "TEST",
			string(parser.PatronIdentifier): "12345",
			string(parser.ItemIdentifier):   "ITEM001",
		},
	}

	response, err := handler.Handle(context.Background(), msg, session)
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	// Should return error response
	if !strings.Contains(response, "N") {
		t.Errorf("Expected failure indicator in response: %s", response)
	}
	if !strings.Contains(response, "authentication error") {
		t.Errorf("Expected authentication error message in response: %s", response)
	}
}

// TestRenewHandler_SequenceNumber tests sequence number handling
func TestRenewHandler_SequenceNumber(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/circulation/renew-by-barcode"):
			dueDate := time.Now().Add(14 * 24 * time.Hour)
			loan := models.Loan{
				ID:      "loan-123",
				UserID:  "user-123",
				ItemID:  "item-123",
				DueDate: &dueDate,
				Status:  models.LoanStatus{Name: "Open"},
				Item: &models.Item{
					InstanceID: "instance-123",
				},
			}
			json.NewEncoder(w).Encode(loan)
		default:
			http.NotFound(w, r)
		}
	}))
	defer mockServer.Close()

	tenantConfig := &config.TenantConfig{
		OkapiURL:         mockServer.URL,
		Tenant:           "test",
		FieldDelimiter:   "|",
		MessageDelimiter: "\r",
	}

	session := types.NewSession("test-conn", tenantConfig)
	session.SetAuthenticated("testuser", "user-123", "12345", "token-123", time.Now().Add(1*time.Hour))

	logger := zap.NewNop()
	handler := NewRenewHandler(logger, tenantConfig)

	tests := []struct {
		name           string
		sequenceNumber string
		expectInResp   string
	}{
		{"With sequence number", "999", "AY999"},
		{"Default sequence number", "", "AY0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := &parser.Message{
				Code: parser.RenewRequest,
				Fields: map[string]string{
					string(parser.InstitutionID):    "TEST",
					string(parser.PatronIdentifier): "12345",
					string(parser.ItemIdentifier):   "ITEM001",
				},
				SequenceNumber: tt.sequenceNumber,
			}

			response, err := handler.Handle(context.Background(), msg, session)
			if err != nil {
				t.Fatalf("Handle() error = %v", err)
			}

			if !strings.Contains(response, tt.expectInResp) {
				t.Errorf("Expected %s in response: %s", tt.expectInResp, response)
			}
		})
	}
}
