package folio

import (
	"context"
	"fmt"
	"net/url"

	"github.com/spokanepubliclibrary/fsip2/internal/folio/models"
)

// CirculationClient handles circulation operations
type CirculationClient struct {
	client *Client
}

// NewCirculationClient creates a new circulation client
func NewCirculationClient(baseURL, tenant string) *CirculationClient {
	return &CirculationClient{
		client: NewClient(baseURL, tenant),
	}
}

// CheckoutRequest represents a checkout request
type CheckoutRequest struct {
	ItemBarcode    string `json:"itemBarcode"`
	UserBarcode    string `json:"userBarcode"`
	ServicePointID string `json:"servicePointId"`
	LoanDate       string `json:"loanDate,omitempty"`
}

// CheckinRequest represents a checkin request
type CheckinRequest struct {
	ItemBarcode               string `json:"itemBarcode"`
	ServicePointID            string `json:"servicePointId"`
	CheckInDate               string `json:"checkInDate,omitempty"`
	ClaimedReturnedResolution string `json:"claimedReturnedResolution,omitempty"`
}

// RenewRequest represents a renewal request
type RenewRequest struct {
	ItemBarcode string `json:"itemBarcode"`
	UserBarcode string `json:"userBarcode"`
}

// RenewByIDRequest represents a renewal request by ID
type RenewByIDRequest struct {
	ItemID string `json:"itemId"`
	UserID string `json:"userId"`
}

// Checkout checks out an item to a patron
func (cc *CirculationClient) Checkout(ctx context.Context, token string, req CheckoutRequest) (*models.Loan, error) {
	var loan models.Loan
	err := cc.client.Post(ctx, "/circulation/check-out-by-barcode", token, req, &loan)
	if err != nil {
		return nil, fmt.Errorf("checkout failed: %w", err)
	}

	return &loan, nil
}

// Checkin checks in an item
func (cc *CirculationClient) Checkin(ctx context.Context, token string, req CheckinRequest) (*models.Loan, error) {
	var loan models.Loan
	err := cc.client.Post(ctx, "/circulation/check-in-by-barcode", token, req, &loan)
	if err != nil {
		return nil, fmt.Errorf("checkin failed: %w", err)
	}

	return &loan, nil
}

// Renew renews a loan
func (cc *CirculationClient) Renew(ctx context.Context, token string, req RenewRequest) (*models.Loan, error) {
	var loan models.Loan
	err := cc.client.Post(ctx, "/circulation/renew-by-barcode", token, req, &loan)
	if err != nil {
		return nil, err
	}

	return &loan, nil
}

// RenewByID renews a loan by user ID and item ID
func (cc *CirculationClient) RenewByID(ctx context.Context, token string, req RenewByIDRequest) (*models.Loan, error) {
	var loan models.Loan
	err := cc.client.Post(ctx, "/circulation/renew-by-id", token, req, &loan)
	if err != nil {
		return nil, fmt.Errorf("renewal by ID failed: %w", err)
	}

	return &loan, nil
}

// PLACEHOLDER – NOT FOR USE. RenewAll is a wireframe for a future FOLIO
// renew-all endpoint and always returns ErrNotImplemented without calling FOLIO.
//   - NOT FUNCTIONAL: the endpoint it was written against
//     (POST /circulation/renew-by-barcode-all) does not exist in Trillium
//     (mod-circulation v24.5.2).
//   - NOT IN ANY ROLE.
//   - Live SIP 65 Renew All renews each loan with RenewByID (renew_all.go).
func (cc *CirculationClient) RenewAll(ctx context.Context, token string, userBarcode string) (*models.LoanCollection, error) {
	return nil, ErrNotImplemented
}

// GetOpenLoansByUser retrieves open loans for a user
func (cc *CirculationClient) GetOpenLoansByUser(ctx context.Context, token string, userID string, limit int) (*models.LoanCollection, error) {
	query := fmt.Sprintf("userId==%s and status.name==Open", userID)
	path := fmt.Sprintf("/circulation/loans?limit=%d&query=%s", limit, url.QueryEscape(query))

	var loans models.LoanCollection
	err := cc.client.Get(ctx, path, token, &loans)
	if err != nil {
		return nil, fmt.Errorf("failed to get open loans: %w", err)
	}

	return &loans, nil
}

// GetAvailableHolds retrieves requests that are ready for pickup for a user.
// A limit of 0 fetches no request records and returns only TotalRecords, for an
// efficient count-only query when itemized hold details are not needed.
func (cc *CirculationClient) GetAvailableHolds(ctx context.Context, token string, userID string, limit int) (*models.RequestCollection, error) {
	query := fmt.Sprintf(`requesterId=="%s" and status=="Open - Awaiting pickup"`, userID)
	path := fmt.Sprintf("/circulation/requests?limit=%d&query=%s", limit, url.QueryEscape(query))

	var requests models.RequestCollection
	err := cc.client.Get(ctx, path, token, &requests)
	if err != nil {
		return nil, fmt.Errorf("failed to get available holds: %w", err)
	}

	return &requests, nil
}

// GetUnavailableHolds retrieves requests that are not yet filled or in transit for a user.
// A limit of 0 fetches no request records and returns only TotalRecords, for an
// efficient count-only query when itemized hold details are not needed.
func (cc *CirculationClient) GetUnavailableHolds(ctx context.Context, token string, userID string, limit int) (*models.RequestCollection, error) {
	query := fmt.Sprintf(`requesterId=="%s" and (status=="Open - Not yet filled" or status=="Open - In transit")`, userID)
	path := fmt.Sprintf("/circulation/requests?limit=%d&query=%s", limit, url.QueryEscape(query))

	var requests models.RequestCollection
	err := cc.client.Get(ctx, path, token, &requests)
	if err != nil {
		return nil, fmt.Errorf("failed to get unavailable holds: %w", err)
	}

	return &requests, nil
}

// GetLoansByItem retrieves loans for a specific item
func (cc *CirculationClient) GetLoansByItem(ctx context.Context, token string, itemID string) (*models.LoanCollection, error) {
	query := fmt.Sprintf(`itemId==%s and status.name=="Open"`, itemID)
	path := fmt.Sprintf("/circulation/loans?query=%s", url.QueryEscape(query))

	var loans models.LoanCollection
	err := cc.client.Get(ctx, path, token, &loans)
	if err != nil {
		return nil, fmt.Errorf("failed to get item loans: %w", err)
	}

	return &loans, nil
}

// GetRequestsByItem retrieves requests for an item
func (cc *CirculationClient) GetRequestsByItem(ctx context.Context, token string, itemID string) (*models.RequestCollection, error) {
	query := fmt.Sprintf(
		`(itemId==%s) and (status==("Open - In transit" or "Open - Awaiting pickup")) sortby requestDate/sort.descending`,
		itemID,
	)
	path := fmt.Sprintf("/circulation/requests?query=%s&limit=20", url.QueryEscape(query))

	var requests models.RequestCollection
	err := cc.client.Get(ctx, path, token, &requests)
	if err != nil {
		return nil, fmt.Errorf("failed to get item requests: %w", err)
	}

	return &requests, nil
}

// PLACEHOLDER – NOT FOR USE. CreateRequest is reserved for a future SIP 15
// Hold "+" (create) and always returns ErrNotImplemented without calling FOLIO.
//   - NOT FUNCTIONAL: SIP 15 is not implemented.
//   - NOT IN ANY ROLE. Future capability: circulation.requests.item.post ->
//     circulation_requests_item.create.
//   - FOLIO requires requesterId, requestType, requestDate and
//     fulfillmentPreference; in practice also item/instance ids, requestLevel
//     and pickupServicePointId. Item vs title level, request type and pickup
//     service point are open design points (plan 02, B3).
//   - Build together with CancelRequest and SIP 15 "*" (change) as one feature.
func (cc *CirculationClient) CreateRequest(ctx context.Context, token string, request *models.Request) (*models.Request, error) {
	return nil, ErrNotImplemented
}

// PLACEHOLDER – NOT FOR USE. CancelRequest is reserved for future request
// cancel / SIP 15 Hold "-" and always returns ErrNotImplemented without calling FOLIO.
//   - NOT FUNCTIONAL: the cancel sub-resource it was written against
//     (POST /circulation/requests/{id}/cancel) does not exist in Trillium.
//   - NOT IN ANY ROLE. Future capabilities: circulation.requests.item.get ->
//     circulation_requests_item.view, circulation.requests.item.put ->
//     circulation_requests_item.edit.
//   - Future spec: GET /circulation/requests/{id}; on the raw map (preserving
//     every other field) set status "Closed - Cancelled", cancellationReasonId
//     from a new tenant config option (proposed requestCancellationReasonId),
//     and cancelledByUserId = the kiosk user's UUID (via GetUserByUsername);
//     then PUT /circulation/requests/{id}. Replace this signature when built.
func (cc *CirculationClient) CancelRequest(ctx context.Context, token string, requestID string, cancellationReasonID string, additionalInfo string) error {
	return ErrNotImplemented
}
