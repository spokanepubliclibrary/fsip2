package handlers

import (
	"context"
	"encoding/json"
	"fmt"
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

// TestRenewAllHandler_SuccessfulRenewalAll tests successful renewal of all items
func TestRenewAllHandler_SuccessfulRenewalAll(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/circulation/loans"):
			// Return open loans
			dueDate := time.Now().Add(7 * 24 * time.Hour)
			loans := models.LoanCollection{
				Loans: []models.Loan{
					{
						ID:      "loan-1",
						UserID:  "user-123",
						ItemID:  "item-1",
						DueDate: &dueDate,
						Status:  models.LoanStatus{Name: "Open"},
					},
					{
						ID:      "loan-2",
						UserID:  "user-123",
						ItemID:  "item-2",
						DueDate: &dueDate,
						Status:  models.LoanStatus{Name: "Open"},
					},
				},
				TotalRecords: 2,
			}
			json.NewEncoder(w).Encode(loans)
		case strings.Contains(r.URL.Path, "/circulation/renew-by-id"):
			// Successful renewal
			newDueDate := time.Now().Add(14 * 24 * time.Hour)
			loan := models.Loan{
				ID:      "loan-renewed",
				UserID:  "user-123",
				ItemID:  "item-123",
				DueDate: &newDueDate,
				Status:  models.LoanStatus{Name: "Open"},
			}
			json.NewEncoder(w).Encode(loan)
		case strings.Contains(r.URL.Path, "/inventory/items/item-1"):
			item := models.Item{ID: "item-1", Barcode: "ITEM001"}
			json.NewEncoder(w).Encode(item)
		case strings.Contains(r.URL.Path, "/inventory/items/item-2"):
			item := models.Item{ID: "item-2", Barcode: "ITEM002"}
			json.NewEncoder(w).Encode(item)
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
	handler := NewRenewAllHandler(logger, tenantConfig)

	msg := &parser.Message{
		Code: parser.RenewAllRequest,
		Fields: map[string]string{
			string(parser.InstitutionID):    "TEST",
			string(parser.PatronIdentifier): "12345",
		},
	}

	response, err := handler.Handle(context.Background(), msg, session)
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	// Verify response
	if !strings.HasPrefix(response, "66") {
		t.Errorf("Expected response to start with '66', got: %s", response)
	}

	// Check for success indicator
	if !strings.Contains(response, "Y") {
		t.Errorf("Expected success indicator 'Y' in response: %s", response)
	}

	// Check renewed items count
	if !strings.Contains(response, "BM0002") {
		t.Errorf("Expected renewed count of 2: %s", response)
	}

	// Check unrenewed items count
	if !strings.Contains(response, "BN0000") {
		t.Errorf("Expected unrenewed count of 0: %s", response)
	}
}

// TestRenewAllHandler_ZeroLoans tests renew all with no loans
func TestRenewAllHandler_ZeroLoans(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/circulation/loans"):
			// Return empty loan collection
			loans := models.LoanCollection{
				Loans:        []models.Loan{},
				TotalRecords: 0,
			}
			json.NewEncoder(w).Encode(loans)
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
	handler := NewRenewAllHandler(logger, tenantConfig)

	msg := &parser.Message{
		Code: parser.RenewAllRequest,
		Fields: map[string]string{
			string(parser.InstitutionID):    "TEST",
			string(parser.PatronIdentifier): "12345",
		},
	}

	response, err := handler.Handle(context.Background(), msg, session)
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	// Should succeed with 0 renewals
	if !strings.HasPrefix(response, "66") {
		t.Errorf("Expected response to start with '66', got: %s", response)
	}

	// Check counts are zero
	if !strings.Contains(response, "BM0000") {
		t.Errorf("Expected renewed count of 0: %s", response)
	}
	if !strings.Contains(response, "BN0000") {
		t.Errorf("Expected unrenewed count of 0: %s", response)
	}
}

// TestRenewAllHandler_PartialRenewal tests renew all with some items failing
func TestRenewAllHandler_PartialRenewal(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/circulation/loans"):
			dueDate := time.Now().Add(7 * 24 * time.Hour)
			loans := models.LoanCollection{
				Loans: []models.Loan{
					{ID: "loan-1", UserID: "user-123", ItemID: "item-1", DueDate: &dueDate, Status: models.LoanStatus{Name: "Open"}},
					{ID: "loan-2", UserID: "user-123", ItemID: "item-2", DueDate: &dueDate, Status: models.LoanStatus{Name: "Open"}},
					{ID: "loan-3", UserID: "user-123", ItemID: "item-3", DueDate: &dueDate, Status: models.LoanStatus{Name: "Open"}},
				},
				TotalRecords: 3,
			}
			json.NewEncoder(w).Encode(loans)
		case strings.Contains(r.URL.Path, "/circulation/renew-by-id"):
			// Check which item is being renewed based on request body
			var renewReq map[string]string
			json.NewDecoder(r.Body).Decode(&renewReq)

			if renewReq["itemId"] == "item-2" {
				// Fail renewal for item-2
				w.WriteHeader(http.StatusUnprocessableEntity)
				w.Write([]byte(`{"errors":[{"message":"Renewal limit reached"}]}`))
				return
			}

			// Success for other items
			newDueDate := time.Now().Add(14 * 24 * time.Hour)
			loan := models.Loan{
				ID:      renewReq["itemId"],
				UserID:  "user-123",
				ItemID:  renewReq["itemId"],
				DueDate: &newDueDate,
				Status:  models.LoanStatus{Name: "Open"},
			}
			json.NewEncoder(w).Encode(loan)
		case strings.Contains(r.URL.Path, "/inventory/items/item-1"):
			json.NewEncoder(w).Encode(models.Item{ID: "item-1", Barcode: "ITEM001"})
		case strings.Contains(r.URL.Path, "/inventory/items/item-2"):
			json.NewEncoder(w).Encode(models.Item{ID: "item-2", Barcode: "ITEM002"})
		case strings.Contains(r.URL.Path, "/inventory/items/item-3"):
			json.NewEncoder(w).Encode(models.Item{ID: "item-3", Barcode: "ITEM003"})
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
	handler := NewRenewAllHandler(logger, tenantConfig)

	msg := &parser.Message{
		Code: parser.RenewAllRequest,
		Fields: map[string]string{
			string(parser.InstitutionID):    "TEST",
			string(parser.PatronIdentifier): "12345",
		},
	}

	response, err := handler.Handle(context.Background(), msg, session)
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	// Should succeed with partial renewal
	if !strings.Contains(response, "Y") {
		t.Errorf("Expected success indicator (at least one renewed): %s", response)
	}

	// Check for both renewed and unrenewed items
	if !strings.Contains(response, "BM0002") {
		t.Errorf("Expected 2 renewed items: %s", response)
	}
	if !strings.Contains(response, "BN0001") {
		t.Errorf("Expected 1 unrenewed item: %s", response)
	}

	// Check for screen message about partial failure
	if !strings.Contains(response, "Some items could not be renewed") {
		t.Errorf("Expected partial failure message: %s", response)
	}
}

// TestRenewAllHandler_AllRenewalsFail tests when all renewals fail
func TestRenewAllHandler_AllRenewalsFail(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/circulation/loans"):
			dueDate := time.Now().Add(7 * 24 * time.Hour)
			loans := models.LoanCollection{
				Loans: []models.Loan{
					{ID: "loan-1", UserID: "user-123", ItemID: "item-1", DueDate: &dueDate, Status: models.LoanStatus{Name: "Open"}},
				},
				TotalRecords: 1,
			}
			json.NewEncoder(w).Encode(loans)
		case strings.Contains(r.URL.Path, "/circulation/renew-by-id"):
			w.WriteHeader(http.StatusUnprocessableEntity)
			w.Write([]byte(`{"errors":[{"message":"Item cannot be renewed"}]}`))
		case strings.Contains(r.URL.Path, "/inventory/items/item-1"):
			json.NewEncoder(w).Encode(models.Item{ID: "item-1", Barcode: "ITEM001"})
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
	handler := NewRenewAllHandler(logger, tenantConfig)

	msg := &parser.Message{
		Code: parser.RenewAllRequest,
		Fields: map[string]string{
			string(parser.InstitutionID):    "TEST",
			string(parser.PatronIdentifier): "12345",
		},
	}

	response, err := handler.Handle(context.Background(), msg, session)
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	// Should fail with all items unrenewed
	if !strings.Contains(response, "N") {
		t.Errorf("Expected failure indicator when no items renewed: %s", response)
	}

	// Check counts
	if !strings.Contains(response, "BM0000") {
		t.Errorf("Expected 0 renewed items: %s", response)
	}
	if !strings.Contains(response, "BN0001") {
		t.Errorf("Expected 1 unrenewed item: %s", response)
	}

	// Check for error message
	if !strings.Contains(response, "No items could be renewed") {
		t.Errorf("Expected failure message: %s", response)
	}
}

// TestRenewAllHandler_MaxItemsLimit tests the max items limit configuration
func TestRenewAllHandler_MaxItemsLimit(t *testing.T) {
	// Create 100 loans but limit should cap at 50
	var loans []models.Loan
	dueDate := time.Now().Add(7 * 24 * time.Hour)
	for i := 0; i < 100; i++ {
		loans = append(loans, models.Loan{
			ID:      fmt.Sprintf("loan-%d", i),
			UserID:  "user-123",
			ItemID:  fmt.Sprintf("item-%d", i),
			DueDate: &dueDate,
			Status:  models.LoanStatus{Name: "Open"},
		})
	}

	renewCount := 0
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/circulation/loans"):
			collection := models.LoanCollection{
				Loans:        loans,
				TotalRecords: 100,
			}
			json.NewEncoder(w).Encode(collection)
		case strings.Contains(r.URL.Path, "/circulation/renew-by-id"):
			renewCount++
			newDueDate := time.Now().Add(14 * 24 * time.Hour)
			loan := models.Loan{
				ID:      "loan-renewed",
				UserID:  "user-123",
				ItemID:  "item-123",
				DueDate: &newDueDate,
				Status:  models.LoanStatus{Name: "Open"},
			}
			json.NewEncoder(w).Encode(loan)
		case strings.Contains(r.URL.Path, "/inventory/items"):
			itemID := strings.TrimPrefix(r.URL.Path, "/inventory/items/")
			json.NewEncoder(w).Encode(models.Item{ID: itemID, Barcode: "BARCODE"})
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
		RenewAllMaxItems: 50, // Set max to 50
	}

	session := types.NewSession("test-conn", tenantConfig)
	session.SetAuthenticated("testuser", "user-123", "12345", "token-123", time.Now().Add(1*time.Hour))

	logger := zap.NewNop()
	handler := NewRenewAllHandler(logger, tenantConfig)

	msg := &parser.Message{
		Code: parser.RenewAllRequest,
		Fields: map[string]string{
			string(parser.InstitutionID):    "TEST",
			string(parser.PatronIdentifier): "12345",
		},
	}

	_, err := handler.Handle(context.Background(), msg, session)
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	// Should only attempt to renew 50 items (max limit)
	if renewCount != 50 {
		t.Errorf("Expected 50 renewal attempts (max limit), got %d", renewCount)
	}
}

// TestRenewAllHandler_MissingRequiredFields tests validation
func TestRenewAllHandler_MissingRequiredFields(t *testing.T) {
	tests := []struct {
		name   string
		fields map[string]string
	}{
		{
			name: "Missing institution ID",
			fields: map[string]string{
				string(parser.PatronIdentifier): "12345",
			},
		},
		{
			name: "Missing patron identifier",
			fields: map[string]string{
				string(parser.InstitutionID): "TEST",
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
			handler := NewRenewAllHandler(logger, tenantConfig)

			msg := &parser.Message{
				Code:   parser.RenewAllRequest,
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

// TestRenewAllHandler_PatronNotFound tests when patron cannot be found
func TestRenewAllHandler_PatronNotFound(t *testing.T) {
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
	handler := NewRenewAllHandler(logger, tenantConfig)

	msg := &parser.Message{
		Code: parser.RenewAllRequest,
		Fields: map[string]string{
			string(parser.InstitutionID):    "TEST",
			string(parser.PatronIdentifier): "UNKNOWN",
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

// TestRenewAllHandler_PatronVerificationRequired tests with patron verification
func TestRenewAllHandler_PatronVerificationRequired(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/users"):
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

	session := types.NewSession("test-conn", tenantConfig)
	session.SetAuthenticated("testuser", "", "", "token-123", time.Now().Add(1*time.Hour))

	logger := zap.NewNop()
	handler := NewRenewAllHandler(logger, tenantConfig)

	msg := &parser.Message{
		Code: parser.RenewAllRequest,
		Fields: map[string]string{
			string(parser.InstitutionID):    "TEST",
			string(parser.PatronIdentifier): "12345",
			// Missing password
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

// TestRenewAllHandler_GetLoansFails tests when retrieving loans fails
func TestRenewAllHandler_GetLoansFails(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/circulation/loans"):
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"errors":[{"message":"Internal error"}]}`))
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
	handler := NewRenewAllHandler(logger, tenantConfig)

	msg := &parser.Message{
		Code: parser.RenewAllRequest,
		Fields: map[string]string{
			string(parser.InstitutionID):    "TEST",
			string(parser.PatronIdentifier): "12345",
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

// TestRenewAllHandler_ResponseFormat tests the response format
func TestRenewAllHandler_ResponseFormat(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/circulation/loans"):
			dueDate := time.Now().Add(7 * 24 * time.Hour)
			loans := models.LoanCollection{
				Loans: []models.Loan{
					{ID: "loan-1", UserID: "user-123", ItemID: "item-1", DueDate: &dueDate, Status: models.LoanStatus{Name: "Open"}},
				},
				TotalRecords: 1,
			}
			json.NewEncoder(w).Encode(loans)
		case strings.Contains(r.URL.Path, "/circulation/renew-by-id"):
			newDueDate := time.Now().Add(14 * 24 * time.Hour)
			loan := models.Loan{
				ID:      "loan-1",
				UserID:  "user-123",
				ItemID:  "item-1",
				DueDate: &newDueDate,
				Status:  models.LoanStatus{Name: "Open"},
			}
			json.NewEncoder(w).Encode(loan)
		case strings.Contains(r.URL.Path, "/inventory/items/item-1"):
			json.NewEncoder(w).Encode(models.Item{ID: "item-1", Barcode: "ITEM001"})
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
	handler := NewRenewAllHandler(logger, tenantConfig)

	msg := &parser.Message{
		Code: parser.RenewAllRequest,
		Fields: map[string]string{
			string(parser.InstitutionID):    "TEST",
			string(parser.PatronIdentifier): "12345",
		},
		SequenceNumber: "456",
	}

	response, err := handler.Handle(context.Background(), msg, session)
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	// Verify response format
	if !strings.HasPrefix(response, "66") {
		t.Errorf("Response should start with '66', got: %s", response)
	}

	// Check for required fields
	requiredFields := []string{"AO", "AA", "BM", "BN"}
	for _, field := range requiredFields {
		if !strings.Contains(response, field) {
			t.Errorf("Response missing required field %s: %s", field, response)
		}
	}

	// Check for sequence number
	if !strings.Contains(response, "AY456") {
		t.Errorf("Response missing sequence number: %s", response)
	}
}

// TestRenewAllHandler_ItemBarcodeRetrievalFailure tests fallback when item barcode retrieval fails
func TestRenewAllHandler_ItemBarcodeRetrievalFailure(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/circulation/loans"):
			dueDate := time.Now().Add(7 * 24 * time.Hour)
			loans := models.LoanCollection{
				Loans: []models.Loan{
					{ID: "loan-1", UserID: "user-123", ItemID: "item-1", DueDate: &dueDate, Status: models.LoanStatus{Name: "Open"}},
				},
				TotalRecords: 1,
			}
			json.NewEncoder(w).Encode(loans)
		case strings.Contains(r.URL.Path, "/circulation/renew-by-id"):
			newDueDate := time.Now().Add(14 * 24 * time.Hour)
			loan := models.Loan{
				ID:      "loan-1",
				UserID:  "user-123",
				ItemID:  "item-1",
				DueDate: &newDueDate,
				Status:  models.LoanStatus{Name: "Open"},
			}
			json.NewEncoder(w).Encode(loan)
		case strings.Contains(r.URL.Path, "/inventory/items/item-1"):
			// Fail to retrieve item barcode
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"errors":[{"message":"Item not found"}]}`))
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
	handler := NewRenewAllHandler(logger, tenantConfig)

	msg := &parser.Message{
		Code: parser.RenewAllRequest,
		Fields: map[string]string{
			string(parser.InstitutionID):    "TEST",
			string(parser.PatronIdentifier): "12345",
		},
	}

	response, err := handler.Handle(context.Background(), msg, session)
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	// Should succeed even when item barcode retrieval fails
	if !strings.Contains(response, "Y") {
		t.Errorf("Expected success indicator in response: %s", response)
	}

	// Should contain count of 1 renewed item
	if !strings.Contains(response, "BM0001") {
		t.Errorf("Expected 1 renewed item: %s", response)
	}

	// Should contain count of 0 unrenewed items
	if !strings.Contains(response, "BN0000") {
		t.Errorf("Expected 0 unrenewed items: %s", response)
	}
}


// TestRenewAllHandler_SingleLoan tests renew all with exactly one loan
func TestRenewAllHandler_SingleLoan(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/circulation/loans"):
			dueDate := time.Now().Add(7 * 24 * time.Hour)
			loans := models.LoanCollection{
				Loans: []models.Loan{
					{ID: "loan-1", UserID: "user-123", ItemID: "item-1", DueDate: &dueDate, Status: models.LoanStatus{Name: "Open"}},
				},
				TotalRecords: 1,
			}
			json.NewEncoder(w).Encode(loans)
		case strings.Contains(r.URL.Path, "/circulation/renew-by-id"):
			newDueDate := time.Now().Add(14 * 24 * time.Hour)
			loan := models.Loan{
				ID:      "loan-1",
				UserID:  "user-123",
				ItemID:  "item-1",
				DueDate: &newDueDate,
				Status:  models.LoanStatus{Name: "Open"},
			}
			json.NewEncoder(w).Encode(loan)
		case strings.Contains(r.URL.Path, "/inventory/items/item-1"):
			json.NewEncoder(w).Encode(models.Item{ID: "item-1", Barcode: "ITEM001"})
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
	handler := NewRenewAllHandler(logger, tenantConfig)

	msg := &parser.Message{
		Code: parser.RenewAllRequest,
		Fields: map[string]string{
			string(parser.InstitutionID):    "TEST",
			string(parser.PatronIdentifier): "12345",
		},
	}

	response, err := handler.Handle(context.Background(), msg, session)
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	// Should succeed with 1 renewal
	if !strings.Contains(response, "Y") {
		t.Errorf("Expected success indicator in response: %s", response)
	}

	if !strings.Contains(response, "BM0001") {
		t.Errorf("Expected 1 renewed item: %s", response)
	}

	if !strings.Contains(response, "BN0000") {
		t.Errorf("Expected 0 unrenewed items: %s", response)
	}
}

// TestRenewAllHandler_MultipleLoans tests renew all with multiple loans (5 items)
func TestRenewAllHandler_MultipleLoans(t *testing.T) {
	var loans []models.Loan
	dueDate := time.Now().Add(7 * 24 * time.Hour)
	for i := 1; i <= 5; i++ {
		loans = append(loans, models.Loan{
			ID:      fmt.Sprintf("loan-%d", i),
			UserID:  "user-123",
			ItemID:  fmt.Sprintf("item-%d", i),
			DueDate: &dueDate,
			Status:  models.LoanStatus{Name: "Open"},
		})
	}

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/circulation/loans"):
			collection := models.LoanCollection{
				Loans:        loans,
				TotalRecords: 5,
			}
			json.NewEncoder(w).Encode(collection)
		case strings.Contains(r.URL.Path, "/circulation/renew-by-id"):
			newDueDate := time.Now().Add(14 * 24 * time.Hour)
			loan := models.Loan{
				ID:      "loan-renewed",
				UserID:  "user-123",
				ItemID:  "item-123",
				DueDate: &newDueDate,
				Status:  models.LoanStatus{Name: "Open"},
			}
			json.NewEncoder(w).Encode(loan)
		case strings.Contains(r.URL.Path, "/inventory/items"):
			itemID := strings.TrimPrefix(r.URL.Path, "/inventory/items/")
			json.NewEncoder(w).Encode(models.Item{ID: itemID, Barcode: fmt.Sprintf("BARCODE-%s", itemID)})
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
	handler := NewRenewAllHandler(logger, tenantConfig)

	msg := &parser.Message{
		Code: parser.RenewAllRequest,
		Fields: map[string]string{
			string(parser.InstitutionID):    "TEST",
			string(parser.PatronIdentifier): "12345",
		},
	}

	response, err := handler.Handle(context.Background(), msg, session)
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	// Should succeed with 5 renewals
	if !strings.Contains(response, "Y") {
		t.Errorf("Expected success indicator in response: %s", response)
	}

	if !strings.Contains(response, "BM0005") {
		t.Errorf("Expected 5 renewed items: %s", response)
	}
}

// TestRenewAllHandler_AuthenticationError tests when authentication fails
func TestRenewAllHandler_AuthenticationError(t *testing.T) {
	tenantConfig := &config.TenantConfig{
		OkapiURL:         "http://localhost:9999", // Invalid URL
		Tenant:           "test",
		FieldDelimiter:   "|",
		MessageDelimiter: "\r",
	}

	// Session without valid token
	session := types.NewSession("test-conn", tenantConfig)
	session.SetAuthenticated("testuser", "user-123", "12345", "", time.Now().Add(-1*time.Hour))

	logger := zap.NewNop()
	handler := NewRenewAllHandler(logger, tenantConfig)

	msg := &parser.Message{
		Code: parser.RenewAllRequest,
		Fields: map[string]string{
			string(parser.InstitutionID):    "TEST",
			string(parser.PatronIdentifier): "12345",
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
