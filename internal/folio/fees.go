package folio

import (
	"context"
	"fmt"
	"net/url"

	"github.com/spokanepubliclibrary/fsip2/internal/folio/models"
)

// FeesClient handles fees and fines operations
type FeesClient struct {
	client *Client
}

// NewFeesClient creates a new fees client
func NewFeesClient(baseURL, tenant string) *FeesClient {
	return &FeesClient{
		client: NewClient(baseURL, tenant),
	}
}

// GetOpenAccountsExcludingSuspended retrieves open accounts for a user, excluding suspended claim returned items
func (fc *FeesClient) GetOpenAccountsExcludingSuspended(ctx context.Context, token string, userID string) (*models.AccountCollection, error) {
	query := fmt.Sprintf(`userId=="%s" and status.name=="Open" and paymentStatus.name<>"Suspended claim returned"`, userID)
	path := fmt.Sprintf("/accounts?query=%s", url.QueryEscape(query))

	var accounts models.AccountCollection
	err := fc.client.Get(ctx, path, token, &accounts)
	if err != nil {
		return nil, fmt.Errorf("failed to get open accounts excluding suspended: %w", err)
	}

	return &accounts, nil
}

// GetEligibleAccountByID retrieves an account by ID and checks if it's eligible for payment
// An account is eligible if: status.name=="open" AND paymentStatus.name<>"Suspended claim returned"
// Returns nil, nil if no eligible account is found
func (fc *FeesClient) GetEligibleAccountByID(ctx context.Context, token string, accountID string) (*models.Account, error) {
	query := fmt.Sprintf(`id=="%s" AND status.name=="open" AND paymentStatus.name<>"Suspended claim returned"`, accountID)
	path := fmt.Sprintf("/accounts?query=%s", url.QueryEscape(query))

	var accounts models.AccountCollection
	err := fc.client.Get(ctx, path, token, &accounts)
	if err != nil {
		return nil, fmt.Errorf("failed to get eligible account: %w", err)
	}

	// Return the first account if found, nil otherwise
	if len(accounts.Accounts) > 0 {
		return &accounts.Accounts[0], nil
	}

	return nil, nil
}

// PayBulkAccounts pays multiple accounts via the FOLIO /accounts-bulk/pay endpoint.
// FOLIO handles balance-aware distribution server-side.
func (fc *FeesClient) PayBulkAccounts(ctx context.Context, token string, payment *models.Payment) error {
	return fc.client.Post(ctx, "/accounts-bulk/pay", token, payment, nil)
}

// PayAccount pays a specific account using the /accounts/{accountId}/pay endpoint
// Returns the payment response with remainingAmount and feefineactions
func (fc *FeesClient) PayAccount(ctx context.Context, token string, accountID string, payment *models.PaymentRequest) (*models.PaymentResponse, error) {
	path := fmt.Sprintf("/accounts/%s/pay", accountID)

	var response models.PaymentResponse
	err := fc.client.Post(ctx, path, token, payment, &response)
	if err != nil {
		return nil, fmt.Errorf("failed to pay account: %w", err)
	}

	return &response, nil
}
