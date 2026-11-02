package folio

import (
	"context"
	"fmt"
	"net/url"

	"github.com/spokanepubliclibrary/fsip2/internal/config"
	"github.com/spokanepubliclibrary/fsip2/internal/folio/models"
	"github.com/spokanepubliclibrary/fsip2/internal/logging"
	"go.uber.org/zap"
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

// eligibleAccountCQL is the CQL predicate for an account that can be paid:
// open, and not suspended as claim returned. It is shared by the bulk and
// single-account queries so the two can never disagree.
const eligibleAccountCQL = `status.name=="Open" and paymentStatus.name<>"Suspended claim returned"`

// GetOpenAccountsExcludingSuspended retrieves open accounts for a user, excluding suspended claim returned items
func (fc *FeesClient) GetOpenAccountsExcludingSuspended(ctx context.Context, token string, userID string) (*models.AccountCollection, error) {
	query := "userId==" + cqlQuote(userID) + " and " + eligibleAccountCQL
	// Fetch every open account; without an explicit limit FOLIO returns only its default page (10)
	path := fmt.Sprintf("/accounts?limit=%d&query=%s", config.OKAPIUnlimitedLimit, url.QueryEscape(query))

	var accounts models.AccountCollection
	err := fc.client.Get(ctx, path, token, &accounts)
	if err != nil {
		return nil, fmt.Errorf("failed to get open accounts excluding suspended: %w", err)
	}

	// Totals are computed from the returned accounts, so a short page would under-report fees
	if accounts.TotalRecords > len(accounts.Accounts) {
		if fc.client.logger != nil {
			fc.client.logger.Warn("fee list truncated",
				logging.TypeField(logging.TypeApplication),
				zap.String("user_id", userID),
				zap.Int("total_records", accounts.TotalRecords),
				zap.Int("returned", len(accounts.Accounts)),
			)
		}
	}

	return &accounts, nil
}

// GetEligibleAccountByID retrieves an account by ID and checks that it is eligible for payment.
// An account is eligible if it belongs to userID and matches eligibleAccountCQL
// (open, and not suspended as claim returned).
// Returns nil, nil if no eligible account is found, including when accountID is
// not a UUID (no FOLIO call is made) or the account belongs to another user.
func (fc *FeesClient) GetEligibleAccountByID(ctx context.Context, token string, userID string, accountID string) (*models.Account, error) {
	// accountID is device-supplied (SIP 37 CG); anything FOLIO would not issue is not eligible
	if !isUUID(accountID) {
		return nil, nil
	}

	query := "id==" + cqlQuote(accountID) + " and userId==" + cqlQuote(userID) + " and " + eligibleAccountCQL
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
	path := fmt.Sprintf("/accounts/%s/pay", url.PathEscape(accountID))

	var response models.PaymentResponse
	err := fc.client.Post(ctx, path, token, payment, &response)
	if err != nil {
		return nil, fmt.Errorf("failed to pay account: %w", err)
	}

	return &response, nil
}
