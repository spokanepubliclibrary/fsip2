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

// GetAccountsByUser retrieves all accounts (fees/fines) for a user
func (fc *FeesClient) GetAccountsByUser(ctx context.Context, token string, userID string) (*models.AccountCollection, error) {
	query := fmt.Sprintf("userId==%s", userID)
	path := fmt.Sprintf("/accounts?query=%s", url.QueryEscape(query))

	var accounts models.AccountCollection
	err := fc.client.Get(ctx, path, token, &accounts)
	if err != nil {
		return nil, fmt.Errorf("failed to get accounts: %w", err)
	}

	return &accounts, nil
}

// GetOpenAccountsByUser retrieves open accounts for a user
func (fc *FeesClient) GetOpenAccountsByUser(ctx context.Context, token string, userID string) (*models.AccountCollection, error) {
	query := fmt.Sprintf("userId==%s and status.name==Open", userID)
	path := fmt.Sprintf("/accounts?query=%s", url.QueryEscape(query))

	var accounts models.AccountCollection
	err := fc.client.Get(ctx, path, token, &accounts)
	if err != nil {
		return nil, fmt.Errorf("failed to get open accounts: %w", err)
	}

	return &accounts, nil
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

// GetAccountByID retrieves an account by ID
func (fc *FeesClient) GetAccountByID(ctx context.Context, token string, accountID string) (*models.Account, error) {
	path := fmt.Sprintf("/accounts/%s", accountID)

	var account models.Account
	err := fc.client.Get(ctx, path, token, &account)
	if err != nil {
		return nil, fmt.Errorf("failed to get account: %w", err)
	}

	return &account, nil
}

// GetFeeFineActions retrieves fee/fine actions for an account
func (fc *FeesClient) GetFeeFineActions(ctx context.Context, token string, accountID string) (*models.FeeFineActionCollection, error) {
	query := fmt.Sprintf("accountId==%s", accountID)
	path := fmt.Sprintf("/feefineactions?query=%s", url.QueryEscape(query))

	var actions models.FeeFineActionCollection
	err := fc.client.Get(ctx, path, token, &actions)
	if err != nil {
		return nil, fmt.Errorf("failed to get fee/fine actions: %w", err)
	}

	return &actions, nil
}

// PayFee records a fee payment
func (fc *FeesClient) PayFee(ctx context.Context, token string, payment *models.Payment) error {
	err := fc.client.Post(ctx, "/feefineactions", token, payment, nil)
	if err != nil {
		return fmt.Errorf("failed to record payment: %w", err)
	}

	return nil
}

// WaiveFee waives a fee
func (fc *FeesClient) WaiveFee(ctx context.Context, token string, accountID string, amount float64, servicePointID string, userName string, comments string) error {
	waiveRequest := map[string]interface{}{
		"amount":         amount,
		"servicePointId": servicePointID,
		"userName":       userName,
		"comments":       comments,
		"notifyPatron":   false,
		"accountIds":     []string{accountID},
	}

	err := fc.client.Post(ctx, "/waives", token, waiveRequest, nil)
	if err != nil {
		return fmt.Errorf("failed to waive fee: %w", err)
	}

	return nil
}

// RefundFee processes a refund
func (fc *FeesClient) RefundFee(ctx context.Context, token string, accountID string, amount float64, paymentMethod string, servicePointID string, userName string, comments string) error {
	refundRequest := map[string]interface{}{
		"amount":         amount,
		"paymentMethod":  paymentMethod,
		"servicePointId": servicePointID,
		"userName":       userName,
		"comments":       comments,
		"notifyPatron":   false,
		"accountIds":     []string{accountID},
	}

	err := fc.client.Post(ctx, "/refunds", token, refundRequest, nil)
	if err != nil {
		return fmt.Errorf("failed to refund fee: %w", err)
	}

	return nil
}

// GetTotalOutstanding calculates the total outstanding balance for a user
func (fc *FeesClient) GetTotalOutstanding(ctx context.Context, token string, userID string) (float64, error) {
	accounts, err := fc.GetOpenAccountsByUser(ctx, token, userID)
	if err != nil {
		return 0, err
	}

	return accounts.GetTotalOutstanding(), nil
}

// GetOutstandingAccounts retrieves accounts with outstanding balance
func (fc *FeesClient) GetOutstandingAccounts(ctx context.Context, token string, userID string) ([]models.Account, error) {
	accounts, err := fc.GetOpenAccountsByUser(ctx, token, userID)
	if err != nil {
		return nil, err
	}

	var outstanding []models.Account
	for _, account := range accounts.Accounts {
		if account.IsOutstanding() {
			outstanding = append(outstanding, account)
		}
	}

	return outstanding, nil
}
