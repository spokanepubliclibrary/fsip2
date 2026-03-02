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

func TestGetOpenRequestsByUser_Success(t *testing.T) {
	now := time.Now()
	requests := models.RequestCollection{
		Requests: []models.Request{
			{ID: "req-001", RequesterID: "user-001", Status: "Open - Not yet filled", RequestDate: &now},
		},
		TotalRecords: 1,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasPrefix(r.URL.Path, "/circulation/requests") {
			query := r.URL.Query().Get("query")
			if strings.Contains(query, "requesterId==") && strings.Contains(query, "Open") {
				json.NewEncoder(w).Encode(requests)
				return
			}
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := NewCirculationClient(server.URL, "test-tenant")
	result, err := client.GetOpenRequestsByUser(context.Background(), "token", "user-001")
	if err != nil {
		t.Fatalf("GetOpenRequestsByUser failed: %v", err)
	}
	if result.TotalRecords != 1 {
		t.Errorf("Expected 1 request, got %d", result.TotalRecords)
	}
	if result.Requests[0].ID != "req-001" {
		t.Errorf("Expected req-001, got %s", result.Requests[0].ID)
	}
}

func TestGetOpenRequestsByUser_Empty(t *testing.T) {
	emptyRequests := models.RequestCollection{Requests: []models.Request{}, TotalRecords: 0}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(emptyRequests)
	}))
	defer server.Close()

	client := NewCirculationClient(server.URL, "test-tenant")
	result, err := client.GetOpenRequestsByUser(context.Background(), "token", "user-no-requests")
	if err != nil {
		t.Fatalf("GetOpenRequestsByUser failed: %v", err)
	}
	if result.TotalRecords != 0 {
		t.Errorf("Expected 0 requests, got %d", result.TotalRecords)
	}
}

func TestGetOpenRequestsByUser_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "server error"})
	}))
	defer server.Close()

	client := NewCirculationClient(server.URL, "test-tenant")
	_, err := client.GetOpenRequestsByUser(context.Background(), "token", "user-001")
	if err == nil {
		t.Error("Expected error for server error response")
	}
}

func TestGetLoansByItem_Success(t *testing.T) {
	now := time.Now()
	dueDate := now.Add(14 * 24 * time.Hour)
	loans := models.LoanCollection{
		Loans: []models.Loan{
			{
				ID:      "loan-item-001",
				ItemID:  "item-uuid-001",
				DueDate: &dueDate,
				Status:  models.LoanStatus{Name: "Open"},
			},
		},
		TotalRecords: 1,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasPrefix(r.URL.Path, "/circulation/loans") {
			query := r.URL.Query().Get("query")
			if strings.Contains(query, "itemId==") {
				json.NewEncoder(w).Encode(loans)
				return
			}
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := NewCirculationClient(server.URL, "test-tenant")
	result, err := client.GetLoansByItem(context.Background(), "token", "item-uuid-001")
	if err != nil {
		t.Fatalf("GetLoansByItem failed: %v", err)
	}
	if result.TotalRecords != 1 {
		t.Errorf("Expected 1 loan, got %d", result.TotalRecords)
	}
	if result.Loans[0].ID != "loan-item-001" {
		t.Errorf("Expected loan-item-001, got %s", result.Loans[0].ID)
	}
}

func TestGetLoansByItem_Empty(t *testing.T) {
	emptyLoans := models.LoanCollection{Loans: []models.Loan{}, TotalRecords: 0}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(emptyLoans)
	}))
	defer server.Close()

	client := NewCirculationClient(server.URL, "test-tenant")
	result, err := client.GetLoansByItem(context.Background(), "token", "item-not-checked-out")
	if err != nil {
		t.Fatalf("GetLoansByItem failed: %v", err)
	}
	if result.TotalRecords != 0 {
		t.Errorf("Expected 0 loans, got %d", result.TotalRecords)
	}
}

func TestGetLoansByItem_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "server error"})
	}))
	defer server.Close()

	client := NewCirculationClient(server.URL, "test-tenant")
	_, err := client.GetLoansByItem(context.Background(), "token", "item-001")
	if err == nil {
		t.Error("Expected error for server error response")
	}
}

func TestGetRequestsByItem_Success(t *testing.T) {
	now := time.Now()
	requests := models.RequestCollection{
		Requests: []models.Request{
			{ID: "req-item-001", ItemID: "item-uuid-002", Status: "Open - Not yet filled", RequestDate: &now},
		},
		TotalRecords: 1,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasPrefix(r.URL.Path, "/circulation/requests") {
			query := r.URL.Query().Get("query")
			if strings.Contains(query, "itemId==") {
				json.NewEncoder(w).Encode(requests)
				return
			}
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := NewCirculationClient(server.URL, "test-tenant")
	result, err := client.GetRequestsByItem(context.Background(), "token", "item-uuid-002")
	if err != nil {
		t.Fatalf("GetRequestsByItem failed: %v", err)
	}
	if result.TotalRecords != 1 {
		t.Errorf("Expected 1 request, got %d", result.TotalRecords)
	}
	if result.Requests[0].ID != "req-item-001" {
		t.Errorf("Expected req-item-001, got %s", result.Requests[0].ID)
	}
}

func TestGetRequestsByItem_Empty(t *testing.T) {
	emptyRequests := models.RequestCollection{Requests: []models.Request{}, TotalRecords: 0}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(emptyRequests)
	}))
	defer server.Close()

	client := NewCirculationClient(server.URL, "test-tenant")
	result, err := client.GetRequestsByItem(context.Background(), "token", "item-no-requests")
	if err != nil {
		t.Fatalf("GetRequestsByItem failed: %v", err)
	}
	if result.TotalRecords != 0 {
		t.Errorf("Expected 0 requests, got %d", result.TotalRecords)
	}
}

func TestGetRequestsByItem_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "server error"})
	}))
	defer server.Close()

	client := NewCirculationClient(server.URL, "test-tenant")
	_, err := client.GetRequestsByItem(context.Background(), "token", "item-001")
	if err == nil {
		t.Error("Expected error for server error response")
	}
}
