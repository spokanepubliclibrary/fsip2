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

	loans, err := client.GetOpenLoansByUser(ctx, "test-token", "user-123", 2147483647)
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

	holds, err := client.GetAvailableHolds(ctx, "test-token", "user-123", 2147483647)
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

	holds, err := client.GetUnavailableHolds(ctx, "test-token", "user-123", 1000)
	if err != nil {
		t.Fatalf("GetUnavailableHolds failed: %v", err)
	}

	if holds.TotalRecords != 2 {
		t.Errorf("Expected 2 unavailable holds, got %d", holds.TotalRecords)
	}
}

func TestGetAvailableHolds_CountOnly(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("limit"); got != "0" {
			t.Errorf("Expected limit=0, got: %s", got)
		}

		// FOLIO/RMB count-only convention: limit=0 returns an accurate
		// TotalRecords with no request records.
		collection := models.RequestCollection{
			Requests:     []models.Request{},
			TotalRecords: 5,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(collection)
	}))
	defer server.Close()

	client := NewCirculationClient(server.URL, "test-tenant")
	ctx := context.Background()

	holds, err := client.GetAvailableHolds(ctx, "test-token", "user-123", 0)
	if err != nil {
		t.Fatalf("GetAvailableHolds failed: %v", err)
	}

	if holds.TotalRecords != 5 {
		t.Errorf("Expected TotalRecords 5, got %d", holds.TotalRecords)
	}

	if len(holds.Requests) != 0 {
		t.Errorf("Expected no request records, got %d", len(holds.Requests))
	}
}

func TestGetUnavailableHolds_CountOnly(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("limit"); got != "0" {
			t.Errorf("Expected limit=0, got: %s", got)
		}

		// FOLIO/RMB count-only convention: limit=0 returns an accurate
		// TotalRecords with no request records.
		collection := models.RequestCollection{
			Requests:     []models.Request{},
			TotalRecords: 3,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(collection)
	}))
	defer server.Close()

	client := NewCirculationClient(server.URL, "test-tenant")
	ctx := context.Background()

	holds, err := client.GetUnavailableHolds(ctx, "test-token", "user-123", 0)
	if err != nil {
		t.Fatalf("GetUnavailableHolds failed: %v", err)
	}

	if holds.TotalRecords != 3 {
		t.Errorf("Expected TotalRecords 3, got %d", holds.TotalRecords)
	}

	if len(holds.Requests) != 0 {
		t.Errorf("Expected no request records, got %d", len(holds.Requests))
	}
}

func TestGetRequestsByItem_QueryFormat(t *testing.T) {
	// Verifies GetRequestsByItem sends the correct CQL query:
	// itemId filter, open-status filter (In transit + Awaiting pickup), sort descending, and limit.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("query")
		limit := r.URL.Query().Get("limit")

		if !strings.Contains(query, "itemId==item-uuid-123") {
			t.Errorf("must filter by itemId, got query: %s", query)
		}
		if !strings.Contains(query, "Open - In transit") {
			t.Errorf("must include in-transit status, got query: %s", query)
		}
		if !strings.Contains(query, "Open - Awaiting pickup") {
			t.Errorf("must include awaiting-pickup status, got query: %s", query)
		}
		if !strings.Contains(query, "sortby requestDate/sort.descending") {
			t.Errorf("must sort newest first, got query: %s", query)
		}
		if limit == "" {
			t.Error("must specify a limit")
		}

		collection := models.RequestCollection{Requests: []models.Request{}, TotalRecords: 0}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(collection)
	}))
	defer server.Close()

	client := NewCirculationClient(server.URL, "test-tenant")
	_, err := client.GetRequestsByItem(context.Background(), "test-token", "item-uuid-123")
	if err != nil {
		t.Fatalf("GetRequestsByItem failed: %v", err)
	}
}
