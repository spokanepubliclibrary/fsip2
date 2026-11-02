package folio

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spokanepubliclibrary/fsip2/internal/folio/models"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
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
	if !errors.Is(err, ErrItemNotFound) {
		t.Errorf("Expected errors.Is(err, ErrItemNotFound), got: %v", err)
	}
	// The message text is unchanged from before ErrItemNotFound was introduced
	if got, want := err.Error(), "item not found with barcode: nonexistent"; got != want {
		t.Errorf("Expected message %q, got %q", want, got)
	}
}

func TestGetItemByBarcode_HTTPErrorIsNotItemNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewInventoryClient(server.URL, "test-tenant")
	_, err := client.GetItemByBarcode(context.Background(), "test-token", "123456")
	if err == nil {
		t.Fatal("Expected error for HTTP 500")
	}
	if errors.Is(err, ErrItemNotFound) {
		t.Errorf("An HTTP error must not be ErrItemNotFound, got: %v", err)
	}
	if errors.Is(err, ErrAmbiguousBarcode) {
		t.Errorf("An HTTP error must not be ErrAmbiguousBarcode, got: %v", err)
	}
}

// ambiguousItemsServer returns two items for a barcode search and counts by-ID lookups
func ambiguousItemsServer(byIDCalls *int) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/inventory/items" {
			collection := models.ItemCollection{
				Items:        []models.Item{{ID: "item-1", Barcode: "111"}, {ID: "item-2", Barcode: "222"}},
				TotalRecords: 2,
			}
			json.NewEncoder(w).Encode(collection)
			return
		}
		*byIDCalls++
		json.NewEncoder(w).Encode(models.Item{ID: "item-1"})
	}))
}

func TestGetItemByBarcode_Ambiguous(t *testing.T) {
	byIDCalls := 0
	server := ambiguousItemsServer(&byIDCalls)
	defer server.Close()

	SetClientLogger(nil)
	client := NewInventoryClient(server.URL, "test-tenant")
	item, err := client.GetItemByBarcode(context.Background(), "test-token", "123456")
	if !errors.Is(err, ErrAmbiguousBarcode) {
		t.Fatalf("Expected ErrAmbiguousBarcode, got: %v", err)
	}
	if errors.Is(err, ErrItemNotFound) {
		t.Errorf("Ambiguous match must not be ErrItemNotFound, got: %v", err)
	}
	if item != nil {
		t.Errorf("Expected no item on ambiguous match, got %s", item.ID)
	}
	if byIDCalls != 0 {
		t.Errorf("Expected no by-ID lookup on ambiguous match, got %d", byIDCalls)
	}
}

func TestGetItemByBarcode_AmbiguousWarns(t *testing.T) {
	byIDCalls := 0
	server := ambiguousItemsServer(&byIDCalls)
	defer server.Close()

	// NewClient copies clientLogger at creation, so the logger must be set first
	core, recorded := observer.New(zap.WarnLevel)
	SetClientLogger(zap.New(core))
	t.Cleanup(func() { SetClientLogger(nil) })

	client := NewInventoryClient(server.URL, "test-tenant")
	if _, err := client.GetItemByBarcode(context.Background(), "test-token", "123456"); !errors.Is(err, ErrAmbiguousBarcode) {
		t.Fatalf("Expected ErrAmbiguousBarcode, got: %v", err)
	}

	entries := recorded.FilterMessage("ambiguous item barcode").All()
	if len(entries) != 1 {
		t.Fatalf("Expected 1 ambiguity WARN, got %d", len(entries))
	}
	fields := entries[0].ContextMap()
	if fields["total_records"] != int64(2) {
		t.Errorf("Expected total_records=2, got %v", fields["total_records"])
	}
	if _, ok := fields["barcode"]; ok {
		t.Errorf("WARN must not log the barcode, got fields %v", fields)
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
