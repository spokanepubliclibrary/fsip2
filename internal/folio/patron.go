package folio

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/spokanepubliclibrary/fsip2/internal/folio/models"
)

// PatronClient handles patron (user) operations
type PatronClient struct {
	client *Client
}

// NewPatronClient creates a new patron client
func NewPatronClient(baseURL, tenant string) *PatronClient {
	return &PatronClient{
		client: NewClient(baseURL, tenant),
	}
}

// GetUserByBarcode retrieves a user by barcode
func (pc *PatronClient) GetUserByBarcode(ctx context.Context, token string, barcode string) (*models.User, error) {
	query := fmt.Sprintf("barcode==%s", barcode)
	path := fmt.Sprintf("/users?query=%s", url.QueryEscape(query))

	var users models.UserCollection
	err := pc.client.Get(ctx, path, token, &users)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	if users.TotalRecords == 0 {
		return nil, fmt.Errorf("user not found with barcode: %s", barcode)
	}

	return &users.Users[0], nil
}

// GetUserByID retrieves a user by ID
func (pc *PatronClient) GetUserByID(ctx context.Context, token string, userID string) (*models.User, error) {
	path := fmt.Sprintf("/users/%s", userID)

	var user models.User
	err := pc.client.Get(ctx, path, token, &user)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return &user, nil
}

// GetUserByUsername retrieves a user by username
func (pc *PatronClient) GetUserByUsername(ctx context.Context, token string, username string) (*models.User, error) {
	query := fmt.Sprintf("username==%s", username)
	path := fmt.Sprintf("/users?query=%s", url.QueryEscape(query))

	var users models.UserCollection
	err := pc.client.Get(ctx, path, token, &users)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	if users.TotalRecords == 0 {
		return nil, fmt.Errorf("user not found with username: %s", username)
	}

	return &users.Users[0], nil
}

// GetManualBlocks retrieves manual blocks for a user
func (pc *PatronClient) GetManualBlocks(ctx context.Context, token string, userID string) (*models.ManualBlockCollection, error) {
	query := fmt.Sprintf("userId==%s", userID)
	path := fmt.Sprintf("/manualblocks?query=%s", url.QueryEscape(query))

	var blocks models.ManualBlockCollection
	err := pc.client.Get(ctx, path, token, &blocks)
	if err != nil {
		return nil, fmt.Errorf("failed to get manual blocks: %w", err)
	}

	return &blocks, nil
}

// GetAutomatedPatronBlocks retrieves automated patron blocks
func (pc *PatronClient) GetAutomatedPatronBlocks(ctx context.Context, token string, userID string) (*models.AutomatedPatronBlock, error) {
	path := fmt.Sprintf("/automated-patron-blocks/%s", userID)

	var blocks models.AutomatedPatronBlock
	err := pc.client.Get(ctx, path, token, &blocks)
	if err != nil {
		// If 404, user has no automated blocks (this is normal)
		if httpErr, ok := err.(*HTTPError); ok && httpErr.IsNotFound() {
			return &models.AutomatedPatronBlock{
				AutomatedPatronBlocks: []models.AutomatedBlock{},
			}, nil
		}
		return nil, fmt.Errorf("failed to get automated blocks: %w", err)
	}

	return &blocks, nil
}

// HasBlocks checks if a user has any active blocks
func (pc *PatronClient) HasBlocks(ctx context.Context, token string, userID string) (bool, error) {
	// Check manual blocks
	manualBlocks, err := pc.GetManualBlocks(ctx, token, userID)
	if err != nil {
		return false, err
	}

	if len(manualBlocks.ManualBlocks) > 0 {
		return true, nil
	}

	// Check automated blocks
	automatedBlocks, err := pc.GetAutomatedPatronBlocks(ctx, token, userID)
	if err != nil {
		return false, err
	}

	return len(automatedBlocks.AutomatedPatronBlocks) > 0, nil
}

// GetBorrowingBlocks checks if user has blocks preventing borrowing
func (pc *PatronClient) GetBorrowingBlocks(ctx context.Context, token string, userID string) (bool, []string, error) {
	messages := []string{}

	// Check manual blocks
	manualBlocks, err := pc.GetManualBlocks(ctx, token, userID)
	if err != nil {
		return false, nil, err
	}

	for _, block := range manualBlocks.ManualBlocks {
		if block.Borrowing {
			if block.PatronMessage != "" {
				messages = append(messages, block.PatronMessage)
			} else if block.Desc != "" {
				messages = append(messages, block.Desc)
			}
		}
	}

	// Check automated blocks
	automatedBlocks, err := pc.GetAutomatedPatronBlocks(ctx, token, userID)
	if err != nil {
		return false, nil, err
	}

	for _, block := range automatedBlocks.AutomatedPatronBlocks {
		if block.BlockBorrowing {
			if block.Message != "" {
				messages = append(messages, block.Message)
			}
		}
	}

	return len(messages) > 0, messages, nil
}

// GetRenewalsBlocks checks if user has blocks preventing renewals
func (pc *PatronClient) GetRenewalsBlocks(ctx context.Context, token string, userID string) (bool, []string, error) {
	messages := []string{}

	// Check manual blocks
	manualBlocks, err := pc.GetManualBlocks(ctx, token, userID)
	if err != nil {
		return false, nil, err
	}

	for _, block := range manualBlocks.ManualBlocks {
		if block.Renewals {
			if block.PatronMessage != "" {
				messages = append(messages, block.PatronMessage)
			} else if block.Desc != "" {
				messages = append(messages, block.Desc)
			}
		}
	}

	// Check automated blocks
	automatedBlocks, err := pc.GetAutomatedPatronBlocks(ctx, token, userID)
	if err != nil {
		return false, nil, err
	}

	for _, block := range automatedBlocks.AutomatedPatronBlocks {
		if block.BlockRenewals {
			if block.Message != "" {
				messages = append(messages, block.Message)
			}
		}
	}

	return len(messages) > 0, messages, nil
}

// GetRequestsBlocks checks if user has blocks preventing requests
func (pc *PatronClient) GetRequestsBlocks(ctx context.Context, token string, userID string) (bool, []string, error) {
	messages := []string{}

	// Check manual blocks
	manualBlocks, err := pc.GetManualBlocks(ctx, token, userID)
	if err != nil {
		return false, nil, err
	}

	for _, block := range manualBlocks.ManualBlocks {
		if block.Requests {
			if block.PatronMessage != "" {
				messages = append(messages, block.PatronMessage)
			} else if block.Desc != "" {
				messages = append(messages, block.Desc)
			}
		}
	}

	// Check automated blocks
	automatedBlocks, err := pc.GetAutomatedPatronBlocks(ctx, token, userID)
	if err != nil {
		return false, nil, err
	}

	for _, block := range automatedBlocks.AutomatedPatronBlocks {
		if block.BlockRequests {
			if block.Message != "" {
				messages = append(messages, block.Message)
			}
		}
	}

	return len(messages) > 0, messages, nil
}

// UpdateUser updates a user
func (pc *PatronClient) UpdateUser(ctx context.Context, token string, user *models.User) error {
	path := fmt.Sprintf("/users/%s", user.ID)

	err := pc.client.Put(ctx, path, token, user, nil)
	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}

	return nil
}

// VerifyPatronPasswordWithLogin verifies patron credentials using /authn/login-with-expiry
// Used when usePinForPatronVerification is false
func (pc *PatronClient) VerifyPatronPasswordWithLogin(ctx context.Context, username, password string) (bool, error) {
	loginReq := models.LoginRequest{
		Username: username,
		Password: password,
	}

	var loginResp models.LoginResponse
	err := pc.client.Post(ctx, "/authn/login-with-expiry", "", loginReq, &loginResp)
	if err != nil {
		// Check if it's an authentication error (401/422)
		if httpErr, ok := err.(*HTTPError); ok {
			if httpErr.IsUnauthorized() || httpErr.StatusCode == 422 {
				return false, nil // Invalid credentials
			}
		}
		return false, fmt.Errorf("failed to verify patron password: %w", err)
	}

	return true, nil
}

// VerifyPatronPin verifies patron PIN using /patron-pin/verify endpoint
// Used when usePinForPatronVerification is true
func (pc *PatronClient) VerifyPatronPin(ctx context.Context, token string, userID string, pin string) (bool, error) {
	pinReq := map[string]string{
		"id":  userID,
		"pin": pin,
	}

	// The /patron-pin/verify endpoint returns 200 for success, 422 for invalid pin
	// This endpoint requires Accept: text/plain header
	err := pc.client.PostWithTextPlainAccept(ctx, "/patron-pin/verify", token, pinReq)
	if err != nil {
		// Check if it's a validation error (422) - invalid PIN
		if httpErr, ok := err.(*HTTPError); ok {
			if httpErr.StatusCode == 422 {
				return false, nil // Invalid PIN
			}
		}
		return false, fmt.Errorf("failed to verify patron pin: %w", err)
	}

	return true, nil
}

// GetPatronGroupByID retrieves a patron group by ID
func (pc *PatronClient) GetPatronGroupByID(ctx context.Context, token string, groupID string) (*models.PatronGroup, error) {
	path := fmt.Sprintf("/groups/%s", groupID)

	var group models.PatronGroup
	err := pc.client.Get(ctx, path, token, &group)
	if err != nil {
		return nil, fmt.Errorf("failed to get patron group: %w", err)
	}

	return &group, nil
}

// UpdateUserExpiration updates a user's expiration date and optionally reactivates the account
// This method fetches the full user record, updates the expirationDate field (and optionally the active field), and PUTs it back
// IMPORTANT: Preserves the exact structure from GET - does not add fields that weren't in the original response
// Parameters:
//   - reactivate: if true, sets active=true when updating expiration (used for rolling renewals with extendExpired=true)
//
// Returns an error if the update fails, including permission errors
func (pc *PatronClient) UpdateUserExpiration(ctx context.Context, token string, userID string, newExpiration string, reactivate bool) error {
	// Fetch the full user record as raw JSON to preserve exact structure
	path := fmt.Sprintf("/users/%s", userID)

	// Get the raw user data as a map to preserve all fields exactly as received
	var userData map[string]interface{}
	err := pc.client.Get(ctx, path, token, &userData)
	if err != nil {
		return fmt.Errorf("failed to fetch user for expiration update: %w", err)
	}

	// Parse the new expiration date string into time.Time
	// The newExpiration should be in FOLIO format: YYYY-MM-DDT00:00:00.000+00:00
	parsedDate, err := parseExpirationDate(newExpiration)
	if err != nil {
		return fmt.Errorf("failed to parse expiration date: %w", err)
	}

	// Update only the expirationDate field in the map
	// This preserves all other fields exactly as they were in the GET response
	userData["expirationDate"] = parsedDate.Format("2006-01-02T15:04:05.000-07:00")

	// Optionally reactivate the account (used for rolling renewals with extendExpired=true)
	if reactivate {
		userData["active"] = true
	}

	// PUT the updated user back to FOLIO
	// Note: FOLIO's PUT /users/{id} endpoint requires Accept: text/plain
	err = pc.client.PutWithTextPlainAccept(ctx, path, token, userData)
	if err != nil {
		// Check for permission errors
		if httpErr, ok := err.(*HTTPError); ok {
			if httpErr.IsForbidden() {
				return &PermissionError{
					Operation: "update user expiration",
					UserID:    userID,
					Err:       httpErr,
				}
			}
		}
		return fmt.Errorf("failed to update user expiration: %w", err)
	}

	return nil
}

// PermissionError represents a permission-related error
type PermissionError struct {
	Operation string
	UserID    string
	Err       error
}

// Error implements the error interface
func (e *PermissionError) Error() string {
	return fmt.Sprintf("permission denied for %s (user: %s): %v", e.Operation, e.UserID, e.Err)
}

// IsPermissionError checks if an error is a permission error
func IsPermissionError(err error) bool {
	_, ok := err.(*PermissionError)
	return ok
}

// parseExpirationDate parses a FOLIO-formatted date string into time.Time
// Expected format: YYYY-MM-DDT00:00:00.000+00:00
func parseExpirationDate(dateStr string) (time.Time, error) {
	// Try multiple common date formats
	formats := []string{
		"2006-01-02T15:04:05.000-07:00", // FOLIO format with timezone
		"2006-01-02T15:04:05.000Z",      // FOLIO format with Z
		"2006-01-02T15:04:05-07:00",     // ISO 8601 with timezone
		"2006-01-02T15:04:05Z",          // ISO 8601 with Z
		time.RFC3339,                    // Standard RFC3339
	}

	for _, format := range formats {
		if t, err := time.Parse(format, dateStr); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("unable to parse date: %s", dateStr)
}
