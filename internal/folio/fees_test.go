package folio

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spokanepubliclibrary/fsip2/internal/folio/models"
)

func TestGetOpenAccountsExcludingSuspended_Success(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("query")
		if !strings.Contains(query, "Suspended claim returned") {
			t.Errorf("Expected suspended claim returned exclusion, got: %s", query)
		}

		collection := models.AccountCollection{
			Accounts: []models.Account{
				{
					ID:            "account-1",
					Status:        models.AccountStatus{Name: "Open"},
					PaymentStatus: models.PaymentStatus{Name: "Outstanding"},
					Amount:        10.00,
					Remaining:     10.00,
				},
			},
			TotalRecords: 1,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(collection)
	}))
	defer server.Close()

	client := NewFeesClient(server.URL, "test-tenant")
	ctx := context.Background()

	accounts, err := client.GetOpenAccountsExcludingSuspended(ctx, "test-token", "user-123")
	if err != nil {
		t.Fatalf("GetOpenAccountsExcludingSuspended failed: %v", err)
	}

	if accounts.TotalRecords != 1 {
		t.Errorf("Expected 1 account, got %d", accounts.TotalRecords)
	}
}

func TestGetEligibleAccountByID_Found(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("query")
		if !strings.Contains(query, `id=="account-123"`) {
			t.Errorf("Expected account ID in query, got: %s", query)
		}

		collection := models.AccountCollection{
			Accounts: []models.Account{
				{
					ID:            "account-123",
					Status:        models.AccountStatus{Name: "open"},
					PaymentStatus: models.PaymentStatus{Name: "Outstanding"},
					Amount:        10.00,
					Remaining:     10.00,
				},
			},
			TotalRecords: 1,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(collection)
	}))
	defer server.Close()

	client := NewFeesClient(server.URL, "test-tenant")
	ctx := context.Background()

	account, err := client.GetEligibleAccountByID(ctx, "test-token", "account-123")
	if err != nil {
		t.Fatalf("GetEligibleAccountByID failed: %v", err)
	}

	if account == nil {
		t.Fatal("Expected to find eligible account")
	}

	if account.ID != "account-123" {
		t.Errorf("Expected account ID account-123, got %s", account.ID)
	}
}

func TestGetEligibleAccountByID_NotFound(t *testing.T) {
	// Create mock server that returns empty collection
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		collection := models.AccountCollection{
			Accounts:     []models.Account{},
			TotalRecords: 0,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(collection)
	}))
	defer server.Close()

	client := NewFeesClient(server.URL, "test-tenant")
	ctx := context.Background()

	account, err := client.GetEligibleAccountByID(ctx, "test-token", "nonexistent")
	if err != nil {
		t.Fatalf("GetEligibleAccountByID should not error: %v", err)
	}

	if account != nil {
		t.Error("Expected nil account for ineligible/nonexistent account")
	}
}

func TestPayAccount_Success(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expectedPath := "/accounts/account-123/pay"
		if r.URL.Path != expectedPath {
			t.Errorf("Expected path %s, got %s", expectedPath, r.URL.Path)
		}

		if r.Method != http.MethodPost {
			t.Errorf("Expected POST method, got %s", r.Method)
		}

		var payment models.PaymentRequest
		json.NewDecoder(r.Body).Decode(&payment)

		if payment.Amount != "10.00" {
			t.Errorf("Expected payment amount 10.00, got %s", payment.Amount)
		}

		// Return payment response
		response := models.PaymentResponse{
			AccountID:       "account-123",
			Amount:          "10.00",
			RemainingAmount: "0.00",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewFeesClient(server.URL, "test-tenant")
	ctx := context.Background()

	payment := &models.PaymentRequest{
		Amount:        "10.00",
		PaymentMethod: "Cash",
	}

	response, err := client.PayAccount(ctx, "test-token", "account-123", payment)
	if err != nil {
		t.Fatalf("PayAccount failed: %v", err)
	}

	if response.RemainingAmount != "0.00" {
		t.Errorf("Expected remaining amount 0.00, got %s", response.RemainingAmount)
	}
}

func TestPayBulkAccounts_PostsToCorrectEndpoint(t *testing.T) {
	var capturedPath string
	var capturedMethod string
	var capturedBody models.Payment

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		capturedMethod = r.Method
		if err := json.NewDecoder(r.Body).Decode(&capturedBody); err != nil {
			t.Errorf("Failed to decode request body: %v", err)
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	client := NewFeesClient(server.URL, "test-tenant")
	ctx := context.Background()

	payment := &models.Payment{
		Amount:         "3.00",
		AccountIds:     []string{"acc-1", "acc-2", "acc-3"},
		ServicePointID: "sp-abc",
		UserName:       "staff",
		PaymentMethod:  "Cash",
		NotifyPatron:   false,
	}

	err := client.PayBulkAccounts(ctx, "test-token", payment)
	if err != nil {
		t.Fatalf("PayBulkAccounts failed: %v", err)
	}

	if capturedPath != "/accounts-bulk/pay" {
		t.Errorf("Expected path /accounts-bulk/pay, got %s", capturedPath)
	}
	if capturedMethod != http.MethodPost {
		t.Errorf("Expected POST method, got %s", capturedMethod)
	}
	if capturedBody.Amount != "3.00" {
		t.Errorf("Expected amount '3.00' as string in JSON body, got %q", capturedBody.Amount)
	}
	if len(capturedBody.AccountIds) != 3 {
		t.Errorf("Expected 3 account IDs, got %d", len(capturedBody.AccountIds))
	}
	if capturedBody.ServicePointID != "sp-abc" {
		t.Errorf("Expected servicePointId sp-abc, got %s", capturedBody.ServicePointID)
	}
}

func TestPayBulkAccounts_PropagatesAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		w.Write([]byte(`{"errors":[{"message":"amount exceeds outstanding balance"}]}`))
	}))
	defer server.Close()

	client := NewFeesClient(server.URL, "test-tenant")
	ctx := context.Background()

	payment := &models.Payment{
		Amount:     "999.00",
		AccountIds: []string{"acc-1"},
	}

	err := client.PayBulkAccounts(ctx, "test-token", payment)
	if err == nil {
		t.Fatal("Expected error from PayBulkAccounts on HTTP 422, got nil")
	}
}
