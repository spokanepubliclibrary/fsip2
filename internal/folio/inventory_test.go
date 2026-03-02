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

func TestGetItemByBarcode_Success(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// First request - search by barcode
		if strings.Contains(r.URL.Path, "/inventory/items") && strings.Contains(r.URL.Query().Get("query"), "barcode==") {
			collection := models.ItemCollection{
				Items:        []models.Item{{ID: "item-123", Barcode: "123456"}},
				TotalRecords: 1,
			}
			json.NewEncoder(w).Encode(collection)
			return
		}

		// Second request - get by ID
		if r.URL.Path == "/inventory/items/item-123" {
			item := models.Item{
				ID:      "item-123",
				Barcode: "123456",
				Status:  models.ItemStatus{Name: "Available"},
			}
			json.NewEncoder(w).Encode(item)
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := NewInventoryClient(server.URL, "test-tenant")
	ctx := context.Background()

	item, err := client.GetItemByBarcode(ctx, "test-token", "123456")
	if err != nil {
		t.Fatalf("GetItemByBarcode failed: %v", err)
	}

	if item.ID != "item-123" {
		t.Errorf("Expected item ID item-123, got %s", item.ID)
	}
}

func TestGetItemByBarcode_NotFound(t *testing.T) {
	// Create mock server that returns empty collection
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		collection := models.ItemCollection{
			Items:        []models.Item{},
			TotalRecords: 0,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(collection)
	}))
	defer server.Close()

	client := NewInventoryClient(server.URL, "test-tenant")
	ctx := context.Background()

	_, err := client.GetItemByBarcode(ctx, "test-token", "nonexistent")
	if err == nil {
		t.Error("Expected error for item not found")
	}

	if !strings.Contains(err.Error(), "item not found") {
		t.Errorf("Expected 'item not found' error, got: %v", err)
	}
}

func TestGetItemByID_Success(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/inventory/items/item-123" {
			t.Errorf("Unexpected path: %s", r.URL.Path)
		}

		item := models.Item{
			ID:      "item-123",
			Barcode: "123456",
			Status:  models.ItemStatus{Name: "Available"},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(item)
	}))
	defer server.Close()

	client := NewInventoryClient(server.URL, "test-tenant")
	ctx := context.Background()

	item, err := client.GetItemByID(ctx, "test-token", "item-123")
	if err != nil {
		t.Fatalf("GetItemByID failed: %v", err)
	}

	if item.ID != "item-123" {
		t.Errorf("Expected item ID item-123, got %s", item.ID)
	}
}

func TestGetInstanceByID_Success(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/inventory/instances/instance-123" {
			t.Errorf("Unexpected path: %s", r.URL.Path)
		}

		instance := models.Instance{
			ID:    "instance-123",
			Title: "Test Book",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(instance)
	}))
	defer server.Close()

	client := NewInventoryClient(server.URL, "test-tenant")
	ctx := context.Background()

	instance, err := client.GetInstanceByID(ctx, "test-token", "instance-123")
	if err != nil {
		t.Fatalf("GetInstanceByID failed: %v", err)
	}

	if instance.Title != "Test Book" {
		t.Errorf("Expected title 'Test Book', got %s", instance.Title)
	}
}

func TestGetHoldingsByID_Success(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/holdings-storage/holdings/holdings-123" {
			t.Errorf("Unexpected path: %s", r.URL.Path)
		}

		holdings := models.Holdings{
			ID:         "holdings-123",
			InstanceID: "instance-123",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(holdings)
	}))
	defer server.Close()

	client := NewInventoryClient(server.URL, "test-tenant")
	ctx := context.Background()

	holdings, err := client.GetHoldingsByID(ctx, "test-token", "holdings-123")
	if err != nil {
		t.Fatalf("GetHoldingsByID failed: %v", err)
	}

	if holdings.ID != "holdings-123" {
		t.Errorf("Expected holdings ID holdings-123, got %s", holdings.ID)
	}
}

func TestGetLocationByID_Success(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		location := models.Location{
			ID:   "location-123",
			Name: "Main Library",
			Code: "MAIN",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(location)
	}))
	defer server.Close()

	client := NewInventoryClient(server.URL, "test-tenant")
	ctx := context.Background()

	location, err := client.GetLocationByID(ctx, "test-token", "location-123")
	if err != nil {
		t.Fatalf("GetLocationByID failed: %v", err)
	}

	if location.Code != "MAIN" {
		t.Errorf("Expected location code MAIN, got %s", location.Code)
	}
}

func TestUpdateItemStatus_Success(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// GET item
		if r.Method == http.MethodGet {
			item := models.Item{
				ID:      "item-123",
				Barcode: "123456",
				Status:  models.ItemStatus{Name: "Available"},
			}
			json.NewEncoder(w).Encode(item)
			return
		}

		// PUT item (update status)
		if r.Method == http.MethodPut {
			var item models.Item
			json.NewDecoder(r.Body).Decode(&item)

			if item.Status.Name != "Missing" {
				t.Errorf("Expected status Missing, got %s", item.Status.Name)
			}

			w.WriteHeader(http.StatusNoContent)
			return
		}

		w.WriteHeader(http.StatusMethodNotAllowed)
	}))
	defer server.Close()

	client := NewInventoryClient(server.URL, "test-tenant")
	ctx := context.Background()

	err := client.UpdateItemStatus(ctx, "test-token", "item-123", "Missing")
	if err != nil {
		t.Fatalf("UpdateItemStatus failed: %v", err)
	}
}

func TestSearchInstances_Success(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/search/instances") {
			t.Errorf("Unexpected path: %s", r.URL.Path)
		}

		result := struct {
			Instances    []models.Instance `json:"instances"`
			TotalRecords int               `json:"totalRecords"`
		}{
			Instances: []models.Instance{
				{ID: "instance-1", Title: "Book One"},
				{ID: "instance-2", Title: "Book Two"},
			},
			TotalRecords: 2,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}))
	defer server.Close()

	client := NewInventoryClient(server.URL, "test-tenant")
	ctx := context.Background()

	instances, err := client.SearchInstances(ctx, "test-token", "title=book")
	if err != nil {
		t.Fatalf("SearchInstances failed: %v", err)
	}

	if len(instances) != 2 {
		t.Errorf("Expected 2 instances, got %d", len(instances))
	}
}

func TestSearchInstancesByTitle_Success(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("query")
		if !strings.Contains(query, "title all") {
			t.Errorf("Expected title search query, got: %s", query)
		}

		result := struct {
			Instances    []models.Instance `json:"instances"`
			TotalRecords int               `json:"totalRecords"`
		}{
			Instances: []models.Instance{
				{ID: "instance-1", Title: "The Great Book"},
			},
			TotalRecords: 1,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}))
	defer server.Close()

	client := NewInventoryClient(server.URL, "test-tenant")
	ctx := context.Background()

	instances, err := client.SearchInstancesByTitle(ctx, "test-token", "The Great Book")
	if err != nil {
		t.Fatalf("SearchInstancesByTitle failed: %v", err)
	}

	if len(instances) != 1 {
		t.Errorf("Expected 1 instance, got %d", len(instances))
	}

	if instances[0].Title != "The Great Book" {
		t.Errorf("Expected title 'The Great Book', got %s", instances[0].Title)
	}
}

func TestSearchInstancesByISBN_Success(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("query")
		if !strings.Contains(query, "isbn") {
			t.Errorf("Expected ISBN search query, got: %s", query)
		}

		result := struct {
			Instances    []models.Instance `json:"instances"`
			TotalRecords int               `json:"totalRecords"`
		}{
			Instances: []models.Instance{
				{ID: "instance-1", Title: "Book with ISBN"},
			},
			TotalRecords: 1,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}))
	defer server.Close()

	client := NewInventoryClient(server.URL, "test-tenant")
	ctx := context.Background()

	instances, err := client.SearchInstancesByISBN(ctx, "test-token", "978-0-123456-78-9")
	if err != nil {
		t.Fatalf("SearchInstancesByISBN failed: %v", err)
	}

	if len(instances) != 1 {
		t.Errorf("Expected 1 instance, got %d", len(instances))
	}
}

func TestGetMaterialTypeByID_Success(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		materialType := models.MaterialType{
			ID:   "material-123",
			Name: "Book",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(materialType)
	}))
	defer server.Close()

	client := NewInventoryClient(server.URL, "test-tenant")
	ctx := context.Background()

	materialType, err := client.GetMaterialTypeByID(ctx, "test-token", "material-123")
	if err != nil {
		t.Fatalf("GetMaterialTypeByID failed: %v", err)
	}

	if materialType.Name != "Book" {
		t.Errorf("Expected material type Book, got %s", materialType.Name)
	}
}

func TestGetServicePointByID_Success(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		servicePoint := models.ServicePoint{
			ID:   "sp-123",
			Name: "Main Circulation Desk",
			Code: "MAIN",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(servicePoint)
	}))
	defer server.Close()

	client := NewInventoryClient(server.URL, "test-tenant")
	ctx := context.Background()

	sp, err := client.GetServicePointByID(ctx, "test-token", "sp-123")
	if err != nil {
		t.Fatalf("GetServicePointByID failed: %v", err)
	}

	if sp.Code != "MAIN" {
		t.Errorf("Expected service point code MAIN, got %s", sp.Code)
	}
}
