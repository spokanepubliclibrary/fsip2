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

// RenewAll renews all loans for a patron
func (cc *CirculationClient) RenewAll(ctx context.Context, token string, userBarcode string) (*models.LoanCollection, error) {
	path := fmt.Sprintf("/circulation/renew-by-barcode-all?userBarcode=%s", url.QueryEscape(userBarcode))

	var result models.LoanCollection
	err := cc.client.Post(ctx, path, token, nil, &result)
	if err != nil {
		return nil, fmt.Errorf("renew all failed: %w", err)
	}

	return &result, nil
}

// GetLoansByUser retrieves all loans for a user
func (cc *CirculationClient) GetLoansByUser(ctx context.Context, token string, userID string) (*models.LoanCollection, error) {
	query := fmt.Sprintf("userId==%s", userID)
	path := fmt.Sprintf("/circulation/loans?query=%s", url.QueryEscape(query))

	var loans models.LoanCollection
	err := cc.client.Get(ctx, path, token, &loans)
	if err != nil {
		return nil, fmt.Errorf("failed to get loans: %w", err)
	}

	return &loans, nil
}

// GetOpenLoansByUser retrieves open loans for a user
func (cc *CirculationClient) GetOpenLoansByUser(ctx context.Context, token string, userID string) (*models.LoanCollection, error) {
	query := fmt.Sprintf("userId==%s and status.name==Open", userID)
	path := fmt.Sprintf("/circulation/loans?query=%s", url.QueryEscape(query))

	var loans models.LoanCollection
	err := cc.client.Get(ctx, path, token, &loans)
	if err != nil {
		return nil, fmt.Errorf("failed to get open loans: %w", err)
	}

	return &loans, nil
}

// GetLoanByID retrieves a loan by ID
func (cc *CirculationClient) GetLoanByID(ctx context.Context, token string, loanID string) (*models.Loan, error) {
	path := fmt.Sprintf("/circulation/loans/%s", loanID)

	var loan models.Loan
	err := cc.client.Get(ctx, path, token, &loan)
	if err != nil {
		return nil, fmt.Errorf("failed to get loan: %w", err)
	}

	return &loan, nil
}

// GetRequestsByUser retrieves requests for a user
func (cc *CirculationClient) GetRequestsByUser(ctx context.Context, token string, userID string) (*models.RequestCollection, error) {
	query := fmt.Sprintf("requesterId==%s", userID)
	path := fmt.Sprintf("/circulation/requests?query=%s", url.QueryEscape(query))

	var requests models.RequestCollection
	err := cc.client.Get(ctx, path, token, &requests)
	if err != nil {
		return nil, fmt.Errorf("failed to get requests: %w", err)
	}

	return &requests, nil
}

// GetOpenRequestsByUser retrieves open requests (holds) for a user
func (cc *CirculationClient) GetOpenRequestsByUser(ctx context.Context, token string, userID string) (*models.RequestCollection, error) {
	query := fmt.Sprintf("requesterId==%s and status=Open*", userID)
	path := fmt.Sprintf("/circulation/requests?query=%s", url.QueryEscape(query))

	var requests models.RequestCollection
	err := cc.client.Get(ctx, path, token, &requests)
	if err != nil {
		return nil, fmt.Errorf("failed to get open requests: %w", err)
	}

	return &requests, nil
}

// GetAvailableHolds retrieves requests that are ready for pickup for a user
func (cc *CirculationClient) GetAvailableHolds(ctx context.Context, token string, userID string) (*models.RequestCollection, error) {
	query := fmt.Sprintf(`requesterId=="%s" and status=="Open - Awaiting pickup"`, userID)
	path := fmt.Sprintf("/circulation/requests?query=%s", url.QueryEscape(query))

	var requests models.RequestCollection
	err := cc.client.Get(ctx, path, token, &requests)
	if err != nil {
		return nil, fmt.Errorf("failed to get available holds: %w", err)
	}

	return &requests, nil
}

// GetUnavailableHolds retrieves requests that are not yet filled or in transit for a user
func (cc *CirculationClient) GetUnavailableHolds(ctx context.Context, token string, userID string) (*models.RequestCollection, error) {
	query := fmt.Sprintf(`requesterId=="%s" and (status=="Open - Not yet filled" or status=="Open - In transit")`, userID)
	path := fmt.Sprintf("/circulation/requests?limit=1000&query=%s", url.QueryEscape(query))

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
	query := fmt.Sprintf("itemId==%s", itemID)
	path := fmt.Sprintf("/circulation/requests?query=%s", url.QueryEscape(query))

	var requests models.RequestCollection
	err := cc.client.Get(ctx, path, token, &requests)
	if err != nil {
		return nil, fmt.Errorf("failed to get item requests: %w", err)
	}

	return &requests, nil
}

// CreateRequest creates a new request (hold)
func (cc *CirculationClient) CreateRequest(ctx context.Context, token string, request *models.Request) (*models.Request, error) {
	var result models.Request
	err := cc.client.Post(ctx, "/circulation/requests", token, request, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	return &result, nil
}

// CancelRequest cancels a request
func (cc *CirculationClient) CancelRequest(ctx context.Context, token string, requestID string, cancellationReasonID string, additionalInfo string) error {
	path := fmt.Sprintf("/circulation/requests/%s/cancel", requestID)

	cancelData := map[string]string{
		"cancellationReasonId":              cancellationReasonID,
		"cancellationAdditionalInformation": additionalInfo,
	}

	err := cc.client.Post(ctx, path, token, cancelData, nil)
	if err != nil {
		return fmt.Errorf("failed to cancel request: %w", err)
	}

	return nil
}
