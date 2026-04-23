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

func TestGetAccountsByUser_Success(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("query")
		if !strings.Contains(query, "userId==user-123") {
			t.Errorf("Expected userId query, got: %s", query)
		}

		collection := models.AccountCollection{
			Accounts: []models.Account{
				{ID: "account-1", UserID: "user-123", Amount: 10.00, Remaining: 10.00},
				{ID: "account-2", UserID: "user-123", Amount: 5.00, Remaining: 5.00},
			},
			TotalRecords: 2,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(collection)
	}))
	defer server.Close()

	client := NewFeesClient(server.URL, "test-tenant")
	ctx := context.Background()

	accounts, err := client.GetAccountsByUser(ctx, "test-token", "user-123")
	if err != nil {
		t.Fatalf("GetAccountsByUser failed: %v", err)
	}

	if accounts.TotalRecords != 2 {
		t.Errorf("Expected 2 accounts, got %d", accounts.TotalRecords)
	}
}

func TestGetOpenAccountsByUser_Success(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("query")
		if !strings.Contains(query, "status.name==Open") {
			t.Errorf("Expected Open status filter, got: %s", query)
		}

		collection := models.AccountCollection{
			Accounts: []models.Account{
				{
					ID:        "account-1",
					UserID:    "user-123",
					Amount:    10.00,
					Remaining: 10.00,
					Status:    models.AccountStatus{Name: "Open"},
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

	accounts, err := client.GetOpenAccountsByUser(ctx, "test-token", "user-123")
	if err != nil {
		t.Fatalf("GetOpenAccountsByUser failed: %v", err)
	}

	if accounts.TotalRecords != 1 {
		t.Errorf("Expected 1 open account, got %d", accounts.TotalRecords)
	}

	if accounts.Accounts[0].Status.Name != "Open" {
		t.Errorf("Expected Open status, got %s", accounts.Accounts[0].Status.Name)
	}
}

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

func TestGetAccountByID_Success(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/accounts/account-123" {
			t.Errorf("Unexpected path: %s", r.URL.Path)
		}

		account := models.Account{
			ID:        "account-123",
			UserID:    "user-123",
			Amount:    10.00,
			Remaining: 5.00,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(account)
	}))
	defer server.Close()

	client := NewFeesClient(server.URL, "test-tenant")
	ctx := context.Background()

	account, err := client.GetAccountByID(ctx, "test-token", "account-123")
	if err != nil {
		t.Fatalf("GetAccountByID failed: %v", err)
	}

	if account.ID != "account-123" {
		t.Errorf("Expected account ID account-123, got %s", account.ID)
	}

	if account.Remaining != 5.00 {
		t.Errorf("Expected remaining 5.00, got %f", account.Remaining)
	}
}

func TestGetFeeFineActions_Success(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("query")
		if !strings.Contains(query, "accountId==account-123") {
			t.Errorf("Expected accountId query, got: %s", query)
		}

		collection := models.FeeFineActionCollection{
			FeeFineActions: []models.FeeFineAction{
				{ID: "action-1", AccountID: "account-123", TypeAction: "Payment"},
				{ID: "action-2", AccountID: "account-123", TypeAction: "Charge"},
			},
			TotalRecords: 2,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(collection)
	}))
	defer server.Close()

	client := NewFeesClient(server.URL, "test-tenant")
	ctx := context.Background()

	actions, err := client.GetFeeFineActions(ctx, "test-token", "account-123")
	if err != nil {
		t.Fatalf("GetFeeFineActions failed: %v", err)
	}

	if actions.TotalRecords != 2 {
		t.Errorf("Expected 2 actions, got %d", actions.TotalRecords)
	}
}

func TestPayFee_Success(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/feefineactions" {
			t.Errorf("Unexpected path: %s", r.URL.Path)
		}

		if r.Method != http.MethodPost {
			t.Errorf("Expected POST method, got %s", r.Method)
		}

		var payment models.Payment
		json.NewDecoder(r.Body).Decode(&payment)

		if payment.Amount != "10.00" {
			t.Errorf("Expected amount 10.00, got %s", payment.Amount)
		}

		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	client := NewFeesClient(server.URL, "test-tenant")
	ctx := context.Background()

	payment := &models.Payment{
		Amount:         "10.00",
		PaymentMethod:  "Cash",
		ServicePointID: "sp-123",
		UserName:       "admin",
		AccountIds:     []string{"account-123"},
	}

	err := client.PayFee(ctx, "test-token", payment)
	if err != nil {
		t.Fatalf("PayFee failed: %v", err)
	}
}

func TestWaiveFee_Success(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/waives" {
			t.Errorf("Unexpected path: %s", r.URL.Path)
		}

		var waiveReq map[string]interface{}
		json.NewDecoder(r.Body).Decode(&waiveReq)

		if waiveReq["amount"].(float64) != 10.00 {
			t.Errorf("Expected waive amount 10.00, got %v", waiveReq["amount"])
		}

		accountIds, ok := waiveReq["accountIds"].([]interface{})
		if !ok || len(accountIds) == 0 {
			t.Error("Expected accountIds array")
		}

		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	client := NewFeesClient(server.URL, "test-tenant")
	ctx := context.Background()

	err := client.WaiveFee(ctx, "test-token", "account-123", 10.00, "sp-123", "admin", "Waived by admin")
	if err != nil {
		t.Fatalf("WaiveFee failed: %v", err)
	}
}

func TestRefundFee_Success(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/refunds" {
			t.Errorf("Unexpected path: %s", r.URL.Path)
		}

		var refundReq map[string]interface{}
		json.NewDecoder(r.Body).Decode(&refundReq)

		if refundReq["amount"].(float64) != 5.00 {
			t.Errorf("Expected refund amount 5.00, got %v", refundReq["amount"])
		}

		if refundReq["paymentMethod"].(string) != "Cash" {
			t.Errorf("Expected payment method Cash, got %v", refundReq["paymentMethod"])
		}

		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	client := NewFeesClient(server.URL, "test-tenant")
	ctx := context.Background()

	err := client.RefundFee(ctx, "test-token", "account-123", 5.00, "Cash", "sp-123", "admin", "Refund approved")
	if err != nil {
		t.Fatalf("RefundFee failed: %v", err)
	}
}

func TestGetTotalOutstanding_Success(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		collection := models.AccountCollection{
			Accounts: []models.Account{
				{ID: "account-1", Amount: 10.00, Remaining: 10.00},
				{ID: "account-2", Amount: 5.00, Remaining: 5.00},
			},
			TotalRecords: 2,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(collection)
	}))
	defer server.Close()

	client := NewFeesClient(server.URL, "test-tenant")
	ctx := context.Background()

	total, err := client.GetTotalOutstanding(ctx, "test-token", "user-123")
	if err != nil {
		t.Fatalf("GetTotalOutstanding failed: %v", err)
	}

	// This depends on the GetTotalOutstanding method in AccountCollection
	// Assuming it sums the Remaining amounts
	if total <= 0 {
		t.Errorf("Expected positive total outstanding, got %f", total)
	}
}

func TestGetOutstandingAccounts_Success(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		collection := models.AccountCollection{
			Accounts: []models.Account{
				{ID: "account-1", Amount: 10.00, Remaining: 10.00},
				{ID: "account-2", Amount: 5.00, Remaining: 0.00}, // Paid off
			},
			TotalRecords: 2,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(collection)
	}))
	defer server.Close()

	client := NewFeesClient(server.URL, "test-tenant")
	ctx := context.Background()

	outstanding, err := client.GetOutstandingAccounts(ctx, "test-token", "user-123")
	if err != nil {
		t.Fatalf("GetOutstandingAccounts failed: %v", err)
	}

	// Should only return accounts with Remaining > 0
	// This depends on the IsOutstanding method
	if len(outstanding) == 0 {
		t.Error("Expected at least one outstanding account")
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
