package folio

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/spokanepubliclibrary/fsip2/internal/folio/models"
)

func TestCheckout_Success(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/circulation/check-out-by-barcode" {
			t.Errorf("Unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("Unexpected method: %s", r.Method)
		}

		// Verify request body
		var req CheckoutRequest
		json.NewDecoder(r.Body).Decode(&req)
		if req.ItemBarcode != "item123" || req.UserBarcode != "user123" {
			t.Error("Invalid checkout request")
		}

		// Return successful loan
		loan := models.Loan{
			ID:     "loan-123",
			ItemID: "item-id-123",
			UserID: "user-id-123",
			Status: models.LoanStatus{Name: "Open"},
			DueDate: func() *time.Time { t := time.Now().Add(14 * 24 * time.Hour); return &t }(),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(loan)
	}))
	defer server.Close()

	client := NewCirculationClient(server.URL, "test-tenant")
	ctx := context.Background()

	req := CheckoutRequest{
		ItemBarcode:    "item123",
		UserBarcode:    "user123",
		ServicePointID: "sp123",
	}

	loan, err := client.Checkout(ctx, "test-token", req)
	if err != nil {
		t.Fatalf("Checkout failed: %v", err)
	}

	if loan.ID != "loan-123" {
		t.Errorf("Expected loan ID loan-123, got %s", loan.ID)
	}

	if loan.Status.Name != "Open" {
		t.Errorf("Expected status Open, got %s", loan.Status.Name)
	}
}

func TestCheckout_ItemNotAvailable(t *testing.T) {
	// Create mock server that returns error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		json.NewEncoder(w).Encode(map[string]string{
			"message": "Item is not available",
		})
	}))
	defer server.Close()

	client := NewCirculationClient(server.URL, "test-tenant")
	ctx := context.Background()

	req := CheckoutRequest{
		ItemBarcode:    "unavailable-item",
		UserBarcode:    "user123",
		ServicePointID: "sp123",
	}

	_, err := client.Checkout(ctx, "test-token", req)
	if err == nil {
		t.Error("Expected error for unavailable item")
	}
}

func TestCheckin_Success(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/circulation/check-in-by-barcode" {
			t.Errorf("Unexpected path: %s", r.URL.Path)
		}

		var req CheckinRequest
		json.NewDecoder(r.Body).Decode(&req)
		if req.ItemBarcode != "item123" {
			t.Error("Invalid checkin request")
		}

		// Return loan with Closed status
		loan := models.Loan{
			ID:     "loan-123",
			ItemID: "item-id-123",
			Status: models.LoanStatus{Name: "Closed"},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(loan)
	}))
	defer server.Close()

	client := NewCirculationClient(server.URL, "test-tenant")
	ctx := context.Background()

	req := CheckinRequest{
		ItemBarcode:    "item123",
		ServicePointID: "sp123",
	}

	loan, err := client.Checkin(ctx, "test-token", req)
	if err != nil {
		t.Fatalf("Checkin failed: %v", err)
	}

	if loan.Status.Name != "Closed" {
		t.Errorf("Expected status Closed, got %s", loan.Status.Name)
	}
}

func TestRenew_Success(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/circulation/renew-by-barcode" {
			t.Errorf("Unexpected path: %s", r.URL.Path)
		}

		var req RenewRequest
		json.NewDecoder(r.Body).Decode(&req)

		// Return renewed loan with new due date
		loan := models.Loan{
			ID:            "loan-123",
			ItemID:        "item-id-123",
			UserID:        "user-id-123",
			Status:        models.LoanStatus{Name: "Open"},
			DueDate:       func() *time.Time { t := time.Now().Add(14 * 24 * time.Hour); return &t }(),
			RenewalCount:  1,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(loan)
	}))
	defer server.Close()

	client := NewCirculationClient(server.URL, "test-tenant")
	ctx := context.Background()

	req := RenewRequest{
		ItemBarcode: "item123",
		UserBarcode: "user123",
	}

	loan, err := client.Renew(ctx, "test-token", req)
	if err != nil {
		t.Fatalf("Renew failed: %v", err)
	}

	if loan.RenewalCount != 1 {
		t.Errorf("Expected renewal count 1, got %d", loan.RenewalCount)
	}
}

func TestRenewByID_Success(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/circulation/renew-by-id" {
			t.Errorf("Unexpected path: %s", r.URL.Path)
		}

		var req RenewByIDRequest
		json.NewDecoder(r.Body).Decode(&req)

		if req.ItemID != "item-id-123" || req.UserID != "user-id-123" {
			t.Error("Invalid renew by ID request")
		}

		loan := models.Loan{
			ID:           "loan-123",
			ItemID:       req.ItemID,
			UserID:       req.UserID,
			Status:       models.LoanStatus{Name: "Open"},
			RenewalCount: 1,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(loan)
	}))
	defer server.Close()

	client := NewCirculationClient(server.URL, "test-tenant")
	ctx := context.Background()

	req := RenewByIDRequest{
		ItemID: "item-id-123",
		UserID: "user-id-123",
	}

	loan, err := client.RenewByID(ctx, "test-token", req)
	if err != nil {
		t.Fatalf("RenewByID failed: %v", err)
	}

	if loan.ItemID != "item-id-123" {
		t.Errorf("Expected item ID item-id-123, got %s", loan.ItemID)
	}
}

func TestGetLoansByUser_Success(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/circulation/loans") {
			t.Errorf("Unexpected path: %s", r.URL.Path)
		}

		// Verify query parameter
		query := r.URL.Query().Get("query")
		if !strings.Contains(query, "userId==user-123") {
			t.Errorf("Expected userId query, got: %s", query)
		}

		// Return loan collection
		collection := models.LoanCollection{
			Loans: []models.Loan{
				{ID: "loan-1", UserID: "user-123", Status: models.LoanStatus{Name: "Open"}},
				{ID: "loan-2", UserID: "user-123", Status: models.LoanStatus{Name: "Open"}},
			},
			TotalRecords: 2,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(collection)
	}))
	defer server.Close()

	client := NewCirculationClient(server.URL, "test-tenant")
	ctx := context.Background()

	loans, err := client.GetLoansByUser(ctx, "test-token", "user-123")
	if err != nil {
		t.Fatalf("GetLoansByUser failed: %v", err)
	}

	if loans.TotalRecords != 2 {
		t.Errorf("Expected 2 loans, got %d", loans.TotalRecords)
	}
}

func TestGetOpenLoansByUser_Success(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("query")
		if !strings.Contains(query, "status.name==Open") {
			t.Errorf("Expected Open status filter, got: %s", query)
		}

		collection := models.LoanCollection{
			Loans: []models.Loan{
				{ID: "loan-1", UserID: "user-123", Status: models.LoanStatus{Name: "Open"}},
			},
			TotalRecords: 1,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(collection)
	}))
	defer server.Close()

	client := NewCirculationClient(server.URL, "test-tenant")
	ctx := context.Background()

	loans, err := client.GetOpenLoansByUser(ctx, "test-token", "user-123")
	if err != nil {
		t.Fatalf("GetOpenLoansByUser failed: %v", err)
	}

	if loans.TotalRecords != 1 {
		t.Errorf("Expected 1 open loan, got %d", loans.TotalRecords)
	}

	if loans.Loans[0].Status.Name != "Open" {
		t.Errorf("Expected Open status, got %s", loans.Loans[0].Status.Name)
	}
}

func TestGetRequestsByUser_Success(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("query")
		if !strings.Contains(query, "requesterId==user-123") {
			t.Errorf("Expected requesterId query, got: %s", query)
		}

		collection := models.RequestCollection{
			Requests: []models.Request{
				{ID: "request-1", RequesterID: "user-123", Status: "Open - Not yet filled"},
				{ID: "request-2", RequesterID: "user-123", Status: "Open - Awaiting pickup"},
			},
			TotalRecords: 2,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(collection)
	}))
	defer server.Close()

	client := NewCirculationClient(server.URL, "test-tenant")
	ctx := context.Background()

	requests, err := client.GetRequestsByUser(ctx, "test-token", "user-123")
	if err != nil {
		t.Fatalf("GetRequestsByUser failed: %v", err)
	}

	if requests.TotalRecords != 2 {
		t.Errorf("Expected 2 requests, got %d", requests.TotalRecords)
	}
}

func TestGetAvailableHolds_Success(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("query")
		if !strings.Contains(query, "Open - Awaiting pickup") {
			t.Errorf("Expected Awaiting pickup status, got: %s", query)
		}

		collection := models.RequestCollection{
			Requests: []models.Request{
				{ID: "request-1", RequesterID: "user-123", Status: "Open - Awaiting pickup"},
			},
			TotalRecords: 1,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(collection)
	}))
	defer server.Close()

	client := NewCirculationClient(server.URL, "test-tenant")
	ctx := context.Background()

	holds, err := client.GetAvailableHolds(ctx, "test-token", "user-123")
	if err != nil {
		t.Fatalf("GetAvailableHolds failed: %v", err)
	}

	if holds.TotalRecords != 1 {
		t.Errorf("Expected 1 available hold, got %d", holds.TotalRecords)
	}

	if holds.Requests[0].Status != "Open - Awaiting pickup" {
		t.Errorf("Expected Awaiting pickup status, got %s", holds.Requests[0].Status)
	}
}

func TestGetUnavailableHolds_Success(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("query")
		if !strings.Contains(query, "Open - Not yet filled") {
			t.Errorf("Expected Not yet filled status in query, got: %s", query)
		}

		collection := models.RequestCollection{
			Requests: []models.Request{
				{ID: "request-1", Status: "Open - Not yet filled"},
				{ID: "request-2", Status: "Open - In transit"},
			},
			TotalRecords: 2,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(collection)
	}))
	defer server.Close()

	client := NewCirculationClient(server.URL, "test-tenant")
	ctx := context.Background()

	holds, err := client.GetUnavailableHolds(ctx, "test-token", "user-123")
	if err != nil {
		t.Fatalf("GetUnavailableHolds failed: %v", err)
	}

	if holds.TotalRecords != 2 {
		t.Errorf("Expected 2 unavailable holds, got %d", holds.TotalRecords)
	}
}

func TestRenewAll_Success(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/circulation/renew-by-barcode-all") {
			t.Errorf("Unexpected path: %s", r.URL.Path)
		}

		// Verify userBarcode query parameter
		userBarcode := r.URL.Query().Get("userBarcode")
		if userBarcode != "user123" {
			t.Errorf("Expected userBarcode user123, got %s", userBarcode)
		}

		// Return renewed loans
		collection := models.LoanCollection{
			Loans: []models.Loan{
				{ID: "loan-1", Status: models.LoanStatus{Name: "Open"}, RenewalCount: 1},
				{ID: "loan-2", Status: models.LoanStatus{Name: "Open"}, RenewalCount: 1},
			},
			TotalRecords: 2,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(collection)
	}))
	defer server.Close()

	client := NewCirculationClient(server.URL, "test-tenant")
	ctx := context.Background()

	result, err := client.RenewAll(ctx, "test-token", "user123")
	if err != nil {
		t.Fatalf("RenewAll failed: %v", err)
	}

	if result.TotalRecords != 2 {
		t.Errorf("Expected 2 renewed loans, got %d", result.TotalRecords)
	}
}

func TestGetLoanByID_Success(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/circulation/loans/loan-123" {
			t.Errorf("Unexpected path: %s", r.URL.Path)
		}

		loan := models.Loan{
			ID:     "loan-123",
			ItemID: "item-id-123",
			Status: models.LoanStatus{Name: "Open"},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(loan)
	}))
	defer server.Close()

	client := NewCirculationClient(server.URL, "test-tenant")
	ctx := context.Background()

	loan, err := client.GetLoanByID(ctx, "test-token", "loan-123")
	if err != nil {
		t.Fatalf("GetLoanByID failed: %v", err)
	}

	if loan.ID != "loan-123" {
		t.Errorf("Expected loan ID loan-123, got %s", loan.ID)
	}
}

func TestCreateRequest_Success(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/circulation/requests" {
			t.Errorf("Unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("Unexpected method: %s", r.Method)
		}

		var req models.Request
		json.NewDecoder(r.Body).Decode(&req)

		// Return created request with ID
		req.ID = "request-123"
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(req)
	}))
	defer server.Close()

	client := NewCirculationClient(server.URL, "test-tenant")
	ctx := context.Background()

	request := &models.Request{
		RequesterID: "user-123",
		ItemID:      "item-123",
		RequestType: "Hold",
	}

	result, err := client.CreateRequest(ctx, "test-token", request)
	if err != nil {
		t.Fatalf("CreateRequest failed: %v", err)
	}

	if result.ID != "request-123" {
		t.Errorf("Expected request ID request-123, got %s", result.ID)
	}
}

func TestCancelRequest_Success(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expectedPath := "/circulation/requests/request-123/cancel"
		if r.URL.Path != expectedPath {
			t.Errorf("Expected path %s, got %s", expectedPath, r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("Unexpected method: %s", r.Method)
		}

		var cancelData map[string]string
		json.NewDecoder(r.Body).Decode(&cancelData)

		if cancelData["cancellationReasonId"] != "reason-123" {
			t.Error("Invalid cancellation reason")
		}

		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewCirculationClient(server.URL, "test-tenant")
	ctx := context.Background()

	err := client.CancelRequest(ctx, "test-token", "request-123", "reason-123", "User requested cancellation")
	if err != nil {
		t.Fatalf("CancelRequest failed: %v", err)
	}
}
