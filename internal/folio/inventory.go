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

// PLACEHOLDER – NOT FOR USE. UpdateItemStatus is reserved for future SIP 19
// Item Status Update and always returns ErrNotImplemented without calling FOLIO.
//   - NOT FUNCTIONAL: a typed full PUT of the item would erase fields not in
//     models.Item. The future build must use POST
//     /inventory/items/{id}/mark-<status> (missing, withdrawn, in-process,
//     in-process-non-requestable, intellectual-item, long-missing, restricted,
//     unavailable, unknown), never the full item PUT.
//   - NOT IN ANY ROLE. Each status needs inventory.items.item.mark-<status>.post
//     (capability inventory_items_item_mark-<status>.execute); the role is
//     decided when SIP 19 is built.
//   - Open design points: mapping SIP 19 CH to a status, which statuses a kiosk
//     may set. The SIP 19 handler's current ok=1 response is unchanged.
func (ic *InventoryClient) UpdateItemStatus(ctx context.Context, token string, itemID string, status string) error {
	return ErrNotImplemented
}

// PLACEHOLDER – NOT FOR USE. SearchInstances is reserved for future SIP 15
// title-level holds and always returns ErrNotImplemented without calling FOLIO.
//   - NOT IN ANY ROLE as a caller (search.instances.collection.get is already
//     in Role A).
//   - When built: GET /search/instances, build CQL values with cqlQuote (plan 04) and
//     pass a limit (mod-search's default page size is unverified).
func (ic *InventoryClient) SearchInstances(ctx context.Context, token string, query string) ([]models.Instance, error) {
	return nil, ErrNotImplemented
}

// PLACEHOLDER – NOT FOR USE. SearchInstancesByTitle is reserved for future
// SIP 15 title-level holds and always returns ErrNotImplemented without calling FOLIO.
//   - NOT IN ANY ROLE. When built: quote the title with cqlQuote (plan 04) and pass a limit.
func (ic *InventoryClient) SearchInstancesByTitle(ctx context.Context, token string, title string) ([]models.Instance, error) {
	return nil, ErrNotImplemented
}

// PLACEHOLDER – NOT FOR USE. SearchInstancesByISBN is reserved for future
// SIP 15 title-level holds and always returns ErrNotImplemented without calling FOLIO.
//   - NOT IN ANY ROLE. When built: quote the ISBN with cqlQuote (plan 04) and pass a limit.
func (ic *InventoryClient) SearchInstancesByISBN(ctx context.Context, token string, isbn string) ([]models.Instance, error) {
	return nil, ErrNotImplemented
}
