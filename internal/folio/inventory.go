package folio

import (
	"context"
	"fmt"
	"net/url"

	"github.com/spokanepubliclibrary/fsip2/internal/folio/models"
)

// InventoryClient handles inventory operations
type InventoryClient struct {
	client *Client
}

// NewInventoryClient creates a new inventory client
func NewInventoryClient(baseURL, tenant string) *InventoryClient {
	return &InventoryClient{
		client: NewClient(baseURL, tenant),
	}
}

// GetItemByBarcode retrieves an item by barcode with expanded location details
func (ic *InventoryClient) GetItemByBarcode(ctx context.Context, token string, barcode string) (*models.Item, error) {
	// First, search for the item to get its ID
	query := fmt.Sprintf("barcode==%s", barcode)
	path := fmt.Sprintf("/inventory/items?query=%s", url.QueryEscape(query))

	var items models.ItemCollection
	err := ic.client.Get(ctx, path, token, &items)
	if err != nil {
		return nil, fmt.Errorf("failed to get item: %w", err)
	}

	if items.TotalRecords == 0 {
		return nil, fmt.Errorf("item not found with barcode: %s", barcode)
	}

	// Get the full item details by ID to get populated effectiveLocation
	itemID := items.Items[0].ID
	return ic.GetItemByID(ctx, token, itemID)
}

// GetItemByID retrieves an item by ID
func (ic *InventoryClient) GetItemByID(ctx context.Context, token string, itemID string) (*models.Item, error) {
	path := fmt.Sprintf("/inventory/items/%s", itemID)

	var item models.Item
	err := ic.client.Get(ctx, path, token, &item)
	if err != nil {
		return nil, fmt.Errorf("failed to get item: %w", err)
	}

	return &item, nil
}

// GetInstanceByID retrieves an instance by ID
func (ic *InventoryClient) GetInstanceByID(ctx context.Context, token string, instanceID string) (*models.Instance, error) {
	path := fmt.Sprintf("/inventory/instances/%s", instanceID)

	var instance models.Instance
	err := ic.client.Get(ctx, path, token, &instance)
	if err != nil {
		return nil, fmt.Errorf("failed to get instance: %w", err)
	}

	return &instance, nil
}

// GetHoldingsByID retrieves holdings by ID
func (ic *InventoryClient) GetHoldingsByID(ctx context.Context, token string, holdingsID string) (*models.Holdings, error) {
	path := fmt.Sprintf("/holdings-storage/holdings/%s", holdingsID)

	var holdings models.Holdings
	err := ic.client.Get(ctx, path, token, &holdings)
	if err != nil {
		return nil, fmt.Errorf("failed to get holdings: %w", err)
	}

	return &holdings, nil
}

// GetLocationByID retrieves a location by ID
func (ic *InventoryClient) GetLocationByID(ctx context.Context, token string, locationID string) (*models.Location, error) {
	path := fmt.Sprintf("/locations/%s", locationID)

	var location models.Location
	err := ic.client.Get(ctx, path, token, &location)
	if err != nil {
		return nil, fmt.Errorf("failed to get location: %w", err)
	}

	return &location, nil
}

// GetMaterialTypeByID retrieves a material type by ID
func (ic *InventoryClient) GetMaterialTypeByID(ctx context.Context, token string, materialTypeID string) (*models.MaterialType, error) {
	path := fmt.Sprintf("/material-types/%s", materialTypeID)

	var materialType models.MaterialType
	err := ic.client.Get(ctx, path, token, &materialType)
	if err != nil {
		return nil, fmt.Errorf("failed to get material type: %w", err)
	}

	return &materialType, nil
}

// GetServicePointByID retrieves a service point by ID
func (ic *InventoryClient) GetServicePointByID(ctx context.Context, token string, servicePointID string) (*models.ServicePoint, error) {
	path := fmt.Sprintf("/service-points/%s", servicePointID)

	var servicePoint models.ServicePoint
	err := ic.client.Get(ctx, path, token, &servicePoint)
	if err != nil {
		return nil, fmt.Errorf("failed to get service point: %w", err)
	}

	return &servicePoint, nil
}

// UpdateItemStatus updates the status of an item
func (ic *InventoryClient) UpdateItemStatus(ctx context.Context, token string, itemID string, status string) error {
	// Get the current item
	item, err := ic.GetItemByID(ctx, token, itemID)
	if err != nil {
		return err
	}

	// Update the status
	item.Status.Name = status

	// Send update
	path := fmt.Sprintf("/inventory/items/%s", itemID)
	err = ic.client.Put(ctx, path, token, item, nil)
	if err != nil {
		return fmt.Errorf("failed to update item status: %w", err)
	}

	return nil
}

// SearchInstances searches for instances
func (ic *InventoryClient) SearchInstances(ctx context.Context, token string, query string) ([]models.Instance, error) {
	path := fmt.Sprintf("/search/instances?query=%s", url.QueryEscape(query))

	// Search endpoint returns different structure
	var result struct {
		Instances    []models.Instance `json:"instances"`
		TotalRecords int               `json:"totalRecords"`
	}

	err := ic.client.Get(ctx, path, token, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to search instances: %w", err)
	}

	return result.Instances, nil
}

// SearchInstancesByTitle searches for instances by title
func (ic *InventoryClient) SearchInstancesByTitle(ctx context.Context, token string, title string) ([]models.Instance, error) {
	query := fmt.Sprintf("title all \"%s\"", title)
	return ic.SearchInstances(ctx, token, query)
}

// SearchInstancesByISBN searches for instances by ISBN
func (ic *InventoryClient) SearchInstancesByISBN(ctx context.Context, token string, isbn string) ([]models.Instance, error) {
	query := fmt.Sprintf("isbn=\"%s\"", isbn)
	return ic.SearchInstances(ctx, token, query)
}
