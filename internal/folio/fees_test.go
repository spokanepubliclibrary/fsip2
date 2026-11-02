package folio

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spokanepubliclibrary/fsip2/internal/folio/models"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
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

func TestGetOpenAccountsExcludingSuspended_RequestsUnlimited(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("limit"); got != "2147483647" {
			t.Errorf("Expected limit=2147483647, got %q (raw query: %s)", got, r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(models.AccountCollection{})
	}))
	defer server.Close()

	client := NewFeesClient(server.URL, "test-tenant")
	if _, err := client.GetOpenAccountsExcludingSuspended(context.Background(), "test-token", "user-123"); err != nil {
		t.Fatalf("GetOpenAccountsExcludingSuspended failed: %v", err)
	}
}

// truncatedAccountsServer returns one account but reports totalRecords=3
func truncatedAccountsServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		collection := models.AccountCollection{
			Accounts:     []models.Account{{ID: "account-1", Remaining: 5.00}},
			TotalRecords: 3,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(collection)
	}))
}

func TestGetOpenAccountsExcludingSuspended_WarnsOnTruncation(t *testing.T) {
	server := truncatedAccountsServer()
	defer server.Close()

	// NewClient copies clientLogger at creation, so the logger must be set first
	core, recorded := observer.New(zap.WarnLevel)
	SetClientLogger(zap.New(core))
	t.Cleanup(func() { SetClientLogger(nil) })

	client := NewFeesClient(server.URL, "test-tenant")
	accounts, err := client.GetOpenAccountsExcludingSuspended(context.Background(), "test-token", "user-123")
	if err != nil {
		t.Fatalf("Truncation should not fail the call, got: %v", err)
	}
	if len(accounts.Accounts) != 1 {
		t.Errorf("Expected the 1 returned account, got %d", len(accounts.Accounts))
	}

	entries := recorded.FilterMessage("fee list truncated").All()
	if len(entries) != 1 {
		t.Fatalf("Expected 1 truncation WARN, got %d", len(entries))
	}
	fields := entries[0].ContextMap()
	if fields["user_id"] != "user-123" {
		t.Errorf("Expected user_id=user-123, got %v", fields["user_id"])
	}
	if fields["total_records"] != int64(3) || fields["returned"] != int64(1) {
		t.Errorf("Expected total_records=3 returned=1, got %v / %v", fields["total_records"], fields["returned"])
	}
}

func TestGetOpenAccountsExcludingSuspended_TruncationWithoutLogger(t *testing.T) {
	server := truncatedAccountsServer()
	defer server.Close()

	// No logger set: the WARN must be skipped rather than panic on a nil *zap.Logger
	SetClientLogger(nil)
	client := NewFeesClient(server.URL, "test-tenant")
	if _, err := client.GetOpenAccountsExcludingSuspended(context.Background(), "test-token", "user-123"); err != nil {
		t.Fatalf("GetOpenAccountsExcludingSuspended failed: %v", err)
	}
}

const (
	testPatronUUID  = "5a1f0c3e-7b2d-4e8a-9c6f-1d3b5e7f9a02"
	testAccountUUID = "8e4b2a6c-1f3d-4a5b-b7c9-0d2e4f6a8b13" // was "account-123"
)

func TestGetEligibleAccountByID_Found(t *testing.T) {
	tests := []struct {
		name      string
		accountID string
	}{
		{name: "lower case", accountID: testAccountUUID},
		{name: "upper case", accountID: strings.ToUpper(testAccountUUID)},
		{name: "mixed case", accountID: "8E4b2A6c-1F3d-4A5b-B7c9-0D2e4F6a8B13"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				query := r.URL.Query().Get("query")
				// The ID is sent unchanged (FOLIO matches UUIDs in any case), scoped to the patron
				want := `id=="` + tt.accountID + `" and userId=="` + testPatronUUID + `" and ` + eligibleAccountCQL
				if query != want {
					t.Errorf("Unexpected query:\n got: %s\nwant: %s", query, want)
				}

				collection := models.AccountCollection{
					Accounts: []models.Account{
						{
							ID:            testAccountUUID,
							UserID:        testPatronUUID,
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
			account, err := client.GetEligibleAccountByID(context.Background(), "test-token", testPatronUUID, tt.accountID)
			if err != nil {
				t.Fatalf("GetEligibleAccountByID failed: %v", err)
			}
			if account == nil {
				t.Fatal("Expected to find eligible account")
			}
			if account.ID != testAccountUUID {
				t.Errorf("Expected account ID %s, got %s", testAccountUUID, account.ID)
			}
		})
	}
}

func TestGetEligibleAccountByID_NotFound(t *testing.T) {
	// Create mock server that returns empty collection (no such account, closed,
	// or owned by another patron)
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

	// was "nonexistent"; a well-formed UUID that FOLIO does not return
	account, err := client.GetEligibleAccountByID(ctx, "test-token", testPatronUUID, "3c5e7a9b-2d4f-4b6a-8c1e-5f7a9b0c2d46")
	if err != nil {
		t.Fatalf("GetEligibleAccountByID should not error: %v", err)
	}

	if account != nil {
		t.Error("Expected nil account for ineligible/nonexistent account")
	}
}

func TestGetEligibleAccountByID_NonUUIDMakesNoRequest(t *testing.T) {
	tests := []struct {
		name      string
		accountID string
	}{
		{name: "legacy test id", accountID: "account-123"},
		{name: "nonexistent", accountID: "nonexistent"},
		{name: "mask", accountID: "*"},
		{name: "quote injection", accountID: `x" or id=="y`},
		{name: "braces", accountID: "{" + testAccountUUID + "}"},
		{name: "empty", accountID: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var calls int
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(models.AccountCollection{})
			}))
			defer server.Close()

			client := NewFeesClient(server.URL, "test-tenant")
			account, err := client.GetEligibleAccountByID(context.Background(), "test-token", testPatronUUID, tt.accountID)
			if err != nil {
				t.Fatalf("Expected nil error for non-UUID account ID, got: %v", err)
			}
			if account != nil {
				t.Errorf("Expected nil account for non-UUID account ID, got %+v", account)
			}
			if calls != 0 {
				t.Errorf("Expected no FOLIO request for non-UUID account ID, got %d", calls)
			}
		})
	}
}

func TestGetOpenAccountsExcludingSuspended_SharesEligibilityPredicate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		want := `userId=="` + testPatronUUID + `" and ` + eligibleAccountCQL
		if got := r.URL.Query().Get("query"); got != want {
			t.Errorf("Unexpected query:\n got: %s\nwant: %s", got, want)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(models.AccountCollection{})
	}))
	defer server.Close()

	client := NewFeesClient(server.URL, "test-tenant")
	if _, err := client.GetOpenAccountsExcludingSuspended(context.Background(), "test-token", testPatronUUID); err != nil {
		t.Fatalf("GetOpenAccountsExcludingSuspended failed: %v", err)
	}
}

func TestPayAccount_PathEscapesAccountID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if want := "/accounts/a%2Fb%3Fc/pay"; r.URL.EscapedPath() != want {
			t.Errorf("Expected escaped path %s, got %s", want, r.URL.EscapedPath())
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(models.PaymentResponse{})
	}))
	defer server.Close()

	client := NewFeesClient(server.URL, "test-tenant")
	if _, err := client.PayAccount(context.Background(), "test-token", "a/b?c", &models.PaymentRequest{Amount: "1.00"}); err != nil {
		t.Fatalf("PayAccount failed: %v", err)
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
