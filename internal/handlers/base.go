package handlers

import (
	"context"
	"fmt"
	"time"

	"github.com/spokanepubliclibrary/fsip2/internal/config"
	"github.com/spokanepubliclibrary/fsip2/internal/folio"
	"github.com/spokanepubliclibrary/fsip2/internal/folio/models"
	"github.com/spokanepubliclibrary/fsip2/internal/logging"
	"github.com/spokanepubliclibrary/fsip2/internal/renewal"
	"github.com/spokanepubliclibrary/fsip2/internal/sip2/builder"
	"github.com/spokanepubliclibrary/fsip2/internal/sip2/parser"
	"github.com/spokanepubliclibrary/fsip2/internal/types"
	"go.uber.org/zap"
)

// BaseHandler provides common functionality for all handlers
type BaseHandler struct {
	logger       *zap.Logger
	builder      *builder.ResponseBuilder
	tenantConfig *config.TenantConfig
	// Client factories — overridable in tests (same package access)
	newPatronClient      func(*types.Session) PatronLookup
	newCirculationClient func(*types.Session) CirculationLookup
	newInventoryClient   func(*types.Session) InventoryLookup
	newFeesClient        func(*types.Session) FeesOps
}

// NewBaseHandler creates a new base handler
func NewBaseHandler(logger *zap.Logger, tenantConfig *config.TenantConfig) *BaseHandler {
	h := &BaseHandler{
		logger:       logger,
		builder:      builder.NewResponseBuilder(tenantConfig),
		tenantConfig: tenantConfig,
	}
	h.newPatronClient = func(s *types.Session) PatronLookup {
		return folio.NewPatronClient(s.TenantConfig.OkapiURL, s.TenantConfig.OkapiTenant)
	}
	h.newCirculationClient = func(s *types.Session) CirculationLookup {
		return folio.NewCirculationClient(s.TenantConfig.OkapiURL, s.TenantConfig.OkapiTenant)
	}
	h.newInventoryClient = func(s *types.Session) InventoryLookup {
		return folio.NewInventoryClient(s.TenantConfig.OkapiURL, s.TenantConfig.OkapiTenant)
	}
	h.newFeesClient = func(s *types.Session) FeesOps {
		return folio.NewFeesClient(s.TenantConfig.OkapiURL, s.TenantConfig.OkapiTenant)
	}
	return h
}

// getResponseBuilder returns the response builder
func (h *BaseHandler) getResponseBuilder() *builder.ResponseBuilder {
	return h.builder
}

// getFolioClient creates a FOLIO client for the session's tenant
func (h *BaseHandler) getFolioClient(session *types.Session) (*folio.Client, error) {
	if session.TenantConfig == nil {
		return nil, fmt.Errorf("tenant config not set in session")
	}

	client := folio.NewClient(
		session.TenantConfig.OkapiURL,
		session.TenantConfig.OkapiTenant,
	)

	return client, nil
}

// getAuthenticatedFolioClient creates a FOLIO client and authenticates
// If the token is expired, it will automatically attempt to refresh using stored credentials (Option A)
func (h *BaseHandler) getAuthenticatedFolioClient(ctx context.Context, session *types.Session) (*folio.Client, string, error) {
	client, err := h.getFolioClient(session)
	if err != nil {
		return nil, "", err
	}

	// Use cached token if available and valid (use thread-safe getter)
	authToken := session.GetAuthToken()
	tokenExpiresAt := session.GetTokenExpiresAt()
	isExpired := session.IsTokenExpired()

	// Debug logging for token expiration troubleshooting (Phase 1.1)
	if authToken != "" {
		timeUntilExpiry := time.Until(tokenExpiresAt)
		effectiveTimeRemaining := timeUntilExpiry - (90 * time.Second)

		h.logger.Debug("Token expiration check",
			logging.TypeField(logging.TypeApplication),
			zap.String("session_id", session.ID),
			zap.Bool("is_authenticated", session.IsAuth()),
			zap.Time("token_expires_at", tokenExpiresAt),
			zap.Duration("time_until_actual_expiry", timeUntilExpiry),
			zap.Duration("effective_time_remaining_with_90s_buffer", effectiveTimeRemaining),
			zap.Bool("is_token_expired_with_buffer", isExpired),
		)
	}

	if authToken != "" && !isExpired {
		h.logger.Debug("Using cached authentication token",
			logging.TypeField(logging.TypeApplication),
			zap.String("session_id", session.ID),
			zap.Bool("is_authenticated", session.IsAuth()),
		)
		return client, authToken, nil
	}

	// Token is expired or missing - attempt automatic refresh (Option A - Phase 3)
	if authToken != "" && isExpired {
		h.logger.Debug("Token expired, attempting automatic refresh",
			logging.TypeField(logging.TypeApplication),
			zap.String("session_id", session.ID),
			zap.Time("token_expires_at", tokenExpiresAt),
			zap.Duration("time_since_expiry", time.Since(tokenExpiresAt)),
		)

		// Try to refresh using stored credentials
		newToken, refreshErr := h.refreshToken(ctx, session)
		if refreshErr == nil {
			h.logger.Debug("Token refresh successful",
				logging.TypeField(logging.TypeApplication),
				zap.String("session_id", session.ID),
				zap.Time("new_token_expires_at", session.GetTokenExpiresAt()),
			)
			return client, newToken, nil
		}

		h.logger.Warn("Token refresh failed",
			logging.TypeField(logging.TypeApplication),
			zap.String("session_id", session.ID),
			zap.Error(refreshErr),
		)
	}

	h.logger.Warn("No valid authentication token available in session",
		logging.TypeField(logging.TypeApplication),
		zap.String("session_id", session.ID),
		zap.Bool("is_authenticated", session.IsAuth()),
	)
	return client, "", fmt.Errorf("no authentication token available")
}

// refreshToken attempts to refresh an expired token using stored credentials
// Implements retry with exponential backoff for resilience
func (h *BaseHandler) refreshToken(ctx context.Context, session *types.Session) (string, error) {
	// Check if credentials are stored
	if !session.HasAuthCredentials() {
		return "", fmt.Errorf("no credentials stored for token refresh")
	}

	username, password := session.GetAuthCredentials()

	// Retry configuration
	maxRetries := 3
	baseBackoff := 100 * time.Millisecond

	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff: 100ms, 200ms, 400ms
			backoff := baseBackoff * time.Duration(1<<uint(attempt-1))
			h.logger.Debug("Token refresh retry",
				logging.TypeField(logging.TypeApplication),
				zap.String("session_id", session.ID),
				zap.Int("attempt", attempt+1),
				zap.Duration("backoff", backoff),
			)

			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(backoff):
			}
		}

		// Create auth client and attempt login
		authClient := folio.NewAuthClient(
			session.TenantConfig.OkapiURL,
			session.TenantConfig.OkapiTenant,
			100, // Token cache capacity
		)

		authResp, err := authClient.Login(ctx, username, password)
		if err != nil {
			lastErr = fmt.Errorf("refresh attempt %d failed: %w", attempt+1, err)
			h.logger.Debug("Token refresh attempt failed",
				logging.TypeField(logging.TypeApplication),
				zap.String("session_id", session.ID),
				zap.Int("attempt", attempt+1),
				zap.Error(err),
			)
			continue
		}

		// Determine token to use (prefer OkapiToken, fallback to AccessToken)
		token := authResp.OkapiToken
		if token == "" {
			token = authResp.AccessToken
		}

		if token == "" {
			lastErr = fmt.Errorf("refresh attempt %d: no token received", attempt+1)
			continue
		}

		// Update session with new token
		session.UpdateToken(token, authResp.ExpiresAt)

		h.logger.Debug("Token refresh succeeded",
			logging.TypeField(logging.TypeApplication),
			zap.String("session_id", session.ID),
			zap.Int("attempts", attempt+1),
			zap.Time("new_expires_at", authResp.ExpiresAt),
		)

		return token, nil
	}

	return "", fmt.Errorf("token refresh failed after %d attempts: %w", maxRetries, lastErr)
}

// logRequest logs an incoming SIP2 request
func (h *BaseHandler) logRequest(msg *parser.Message, session *types.Session) {
	h.logger.Info("Handling SIP2 message",
		logging.TypeField(logging.TypeSIPRequest),
		zap.String("message_code", string(msg.Code)),
		zap.String("session_id", session.ID),
		zap.String("tenant", session.TenantConfig.Tenant),
	)
}

// logResponse logs an outgoing SIP2 response
func (h *BaseHandler) logResponse(responseCode string, session *types.Session, err error) {
	if err != nil {
		h.logger.Error("Handler error",
			logging.TypeField(logging.TypeSIPResponse),
			zap.String("response_code", responseCode),
			zap.String("session_id", session.ID),
			zap.Error(err),
		)
	} else {
		h.logger.Info("Handler success",
			logging.TypeField(logging.TypeSIPResponse),
			zap.String("response_code", responseCode),
			zap.String("session_id", session.ID),
		)
	}
}

// buildErrorResponse builds a generic error response
func (h *BaseHandler) buildErrorResponse(msg *parser.Message) string {
	// Return appropriate error response based on message type
	// For now, return a simple error indicator
	return "96" // Request SC/ACS Resend
}

// getCurrentTimestamp returns the current time formatted for SIP2
func (h *BaseHandler) getCurrentTimestamp() time.Time {
	return time.Now()
}

// validateRequiredField checks if a required field is present
func (h *BaseHandler) validateRequiredField(msg *parser.Message, fieldCode parser.FieldCode, fieldName string) error {
	value := msg.GetField(fieldCode)
	if value == "" {
		return fmt.Errorf("required field %s (%s) is missing", fieldName, fieldCode)
	}
	return nil
}

// validateRequiredFields checks multiple required fields
func (h *BaseHandler) validateRequiredFields(msg *parser.Message, fields map[parser.FieldCode]string) error {
	for fieldCode, fieldName := range fields {
		if err := h.validateRequiredField(msg, fieldCode, fieldName); err != nil {
			return err
		}
	}
	return nil
}

// buildPatronStatusString builds the 14-character patron status string based on FOLIO patron blocks.
//
// This function consolidates duplicated logic from patron_status and patron_information handlers,
// mapping FOLIO patron block types to the SIP2 14-character patron status format.
//
// SIP2 Patron Status Format (14 characters, each position Y or space):
//
//	Position  0: Charge privileges denied (checkout blocked)
//	Position  1: Renewal privileges denied (renewals blocked)
//	Position  2: Recall privileges denied (not used in FOLIO)
//	Position  3: Hold privileges denied (requests blocked)
//	Position  4: Card reported lost (not used in FOLIO)
//	Position  5: Too many items charged (not used in FOLIO)
//	Position  6: Too many items overdue (not used in FOLIO)
//	Position  7: Too many renewals (not used in FOLIO)
//	Position  8: Too many claims of items returned (not used in FOLIO)
//	Position  9: Too many items lost (not used in FOLIO)
//	Position 10: Excessive outstanding fines (not used in FOLIO)
//	Position 11: Excessive outstanding fees (not used in FOLIO)
//	Position 12: Recall overdue (not used in FOLIO)
//	Position 13: Too many items billed (not used in FOLIO)
//
// FOLIO Block Mapping:
//   - Manual block with Borrowing=true → Position 0 set to 'Y'
//   - Manual block with Renewals=true → Position 1 set to 'Y'
//   - Manual block with Requests=true → Position 3 set to 'Y'
//   - Automated block with BlockBorrowing=true → Position 0 set to 'Y'
//   - Automated block with BlockRenewals=true → Position 1 set to 'Y'
//   - Automated block with BlockRequests=true → Position 3 set to 'Y'
//
// Invalid Patron Handling:
//
//	If valid=false or user=nil, all 14 positions are set to 'Y' to block all operations.
//
// Example return values:
//
//	"              " - No blocks (all spaces)
//	"Y             " - Checkout blocked only
//	"YY            " - Checkout and renewal blocked
//	"Y Y           " - Checkout and holds blocked
//	"YYYYYYYYYYYYYY" - All privileges denied (invalid patron)
//
// Parameters:
//
//	valid: Whether patron was successfully authenticated
//	user: FOLIO user object (interface{} for flexibility)
//	manualBlocks: FOLIO manual blocks (*models.ManualBlockCollection)
//	automatedBlocks: FOLIO automated blocks (*models.AutomatedPatronBlock)
//
// Returns:
//
//	14-character string with 'Y' for blocked positions, space for allowed
func (h *BaseHandler) buildPatronStatusString(valid bool, user interface{}, manualBlocks interface{}, automatedBlocks interface{}) string {
	// Initialize patron status array (14 characters)
	// Position 0: charge privileges denied (Y/N)
	// Position 1: renewal privileges denied (Y/N)
	// Position 2: recall privileges denied (Y/N)
	// Position 3: hold privileges denied (Y/N)
	// Position 4: card reported lost (Y/N)
	// Position 5: too many items charged (Y/N)
	// Position 6: too many items overdue (Y/N)
	// Position 7: too many renewals (Y/N)
	// Position 8: too many claims of items returned (Y/N)
	// Position 9: too many items lost (Y/N)
	// Position 10: excessive outstanding fines (Y/N)
	// Position 11: excessive outstanding fees (Y/N)
	// Position 12: recall overdue (Y/N)
	// Position 13: too many items billed (Y/N)
	status := make([]byte, 14)
	for i := range status {
		status[i] = ' ' // Default: no blocks
	}

	if !valid || user == nil {
		// If patron not found or invalid, set all blocks
		for i := 0; i < 14; i++ {
			status[i] = 'Y'
		}
	} else {
		// Check actual patron blocks from FOLIO
		if manualBlocks != nil || automatedBlocks != nil {
			var manualBlocksList *models.ManualBlockCollection
			var automatedBlocksList *models.AutomatedPatronBlock

			// Type assert the blocks
			if manualBlocks != nil {
				if mb, ok := manualBlocks.(*models.ManualBlockCollection); ok {
					manualBlocksList = mb
				}
			}
			if automatedBlocks != nil {
				if ab, ok := automatedBlocks.(*models.AutomatedPatronBlock); ok {
					automatedBlocksList = ab
				}
			}

			// Check manual blocks
			if manualBlocksList != nil {
				for _, block := range manualBlocksList.ManualBlocks {
					if block.Borrowing {
						status[0] = 'Y' // Charge privileges denied
					}
					if block.Renewals {
						status[1] = 'Y' // Renewal privileges denied
					}
					if block.Requests {
						status[3] = 'Y' // Hold privileges denied
					}
				}
			}

			// Check automated blocks
			if automatedBlocksList != nil {
				for _, block := range automatedBlocksList.AutomatedPatronBlocks {
					if block.BlockBorrowing {
						status[0] = 'Y' // Charge privileges denied
					}
					if block.BlockRenewals {
						status[1] = 'Y' // Renewal privileges denied
					}
					if block.BlockRequests {
						status[3] = 'Y' // Hold privileges denied
					}
				}
			}
		}
	}

	return string(status)
}

// formatPatronName formats a patron's name in "LastName, FirstName" format
// Uses PreferredFirstName if available, otherwise FirstName, with fallback to username
// This consolidates duplicated logic from patron_status and patron_information handlers
func (h *BaseHandler) formatPatronName(user *models.User) string {
	if user == nil {
		return ""
	}

	// Determine first name: use PreferredFirstName if available, otherwise FirstName
	firstName := user.Personal.PreferredFirstName
	if firstName == "" {
		firstName = user.Personal.FirstName
	}

	// Build patron name
	if user.Personal.LastName != "" || firstName != "" {
		if firstName != "" && user.Personal.LastName != "" {
			return user.Personal.LastName + ", " + firstName
		} else if user.Personal.LastName != "" {
			return user.Personal.LastName
		} else {
			return firstName
		}
	}

	// Fallback to username if no personal name
	return user.Username
}

// getPatronClient returns a PatronLookup for the session's tenant
func (h *BaseHandler) getPatronClient(session *types.Session) PatronLookup {
	return h.newPatronClient(session)
}

// getCirculationClient returns a CirculationLookup for the session's tenant
func (h *BaseHandler) getCirculationClient(session *types.Session) CirculationLookup {
	return h.newCirculationClient(session)
}

// getInventoryClient returns an InventoryLookup for the session's tenant
func (h *BaseHandler) getInventoryClient(session *types.Session) InventoryLookup {
	return h.newInventoryClient(session)
}

// getFeesClient returns a FeesOps for the session's tenant
func (h *BaseHandler) getFeesClient(session *types.Session) FeesOps {
	return h.newFeesClient(session)
}

// fetchItemTitle retrieves item title by fetching item → holdings → instance
// Returns the title string and any error encountered. On error, returns empty string.
// This consolidates duplicated title-fetching logic from multiple handlers.
func (h *BaseHandler) fetchItemTitle(
	ctx context.Context,
	invClient InventoryLookup,
	token string,
	itemBarcode string,
) (string, error) {
	// Fetch item
	item, err := invClient.GetItemByBarcode(ctx, token, itemBarcode)
	if err != nil {
		return "", fmt.Errorf("failed to get item: %w", err)
	}

	if item.HoldingsRecordID == "" {
		return "", fmt.Errorf("item has no holdings record ID")
	}

	// Fetch holdings
	holdings, err := invClient.GetHoldingsByID(ctx, token, item.HoldingsRecordID)
	if err != nil {
		return "", fmt.Errorf("failed to get holdings: %w", err)
	}

	if holdings.InstanceID == "" {
		return "", fmt.Errorf("holdings record has no instance ID")
	}

	// Fetch instance
	instance, err := invClient.GetInstanceByID(ctx, token, holdings.InstanceID)
	if err != nil {
		return "", fmt.Errorf("failed to get instance: %w", err)
	}

	return instance.Title, nil
}

// attemptRollingRenewal attempts to renew a patron's expiration date if eligible
// This function does not block the SIP response - errors are logged but do not propagate
func (h *BaseHandler) attemptRollingRenewal(ctx context.Context, user *models.User, token string, session *types.Session) {
	h.attemptRollingRenewalWithTime(ctx, user, token, session, time.Now())
}

// attemptRollingRenewalWithTime is the internal implementation with time injection for testing
func (h *BaseHandler) attemptRollingRenewalWithTime(ctx context.Context, user *models.User, token string, session *types.Session, today time.Time) {
	// Check if rolling renewals are enabled for this tenant
	if !session.TenantConfig.IsRollingRenewalEnabled() {
		return
	}

	renewalConfig := session.TenantConfig.GetRollingRenewalConfig()
	if renewalConfig == nil {
		return
	}

	// Create renewal service
	renewalService := renewal.NewRollingRenewalService()

	// Check if user should be renewed
	decision := renewalService.ShouldRenew(user, renewalConfig, today)

	// Log the decision
	logFields := []zap.Field{
		zap.String("user_id", user.ID),
		zap.String("username", user.Username),
		zap.String("patron_group", user.PatronGroup),
		zap.Bool("should_renew", decision.ShouldRenew),
		zap.String("reason", decision.Reason),
		zap.Bool("dry_run", renewalConfig.DryRun),
	}

	if user.ExpirationDate != nil {
		logFields = append(logFields, zap.Time("current_expiration", *user.ExpirationDate))
	}

	if !decision.ShouldRenew {
		h.logger.Debug("Rolling renewal not eligible",
			append([]zap.Field{logging.TypeField(logging.TypeApplication)}, logFields...)...,
		)
		return
	}

	// Calculate new expiration date
	newExpiration, err := renewalService.CalculateNewExpiration(today, renewalConfig)
	if err != nil {
		h.logger.Error("Failed to calculate new expiration date",
			append([]zap.Field{logging.TypeField(logging.TypeApplication)}, append(logFields, zap.Error(err))...)...,
		)
		return
	}

	// Format new expiration for FOLIO API
	newExpirationStr := config.FormatDate(newExpiration)
	logFields = append(logFields, zap.Time("new_expiration", newExpiration))

	// Dry-run mode: log what would happen without updating
	if renewalConfig.DryRun {
		h.logger.Info("Rolling renewal (DRY RUN) - would update expiration",
			append([]zap.Field{logging.TypeField(logging.TypeApplication)}, logFields...)...,
		)
		return
	}

	// Attempt to update the user's expiration date in FOLIO
	// If extendExpired is true, also reactivate the account by setting active=true
	patronClient := h.getPatronClient(session)
	err = patronClient.UpdateUserExpiration(ctx, token, user.ID, newExpirationStr, renewalConfig.ExtendExpired)

	if err != nil {
		// Check if it's a permission error
		if folio.IsPermissionError(err) {
			h.logger.Warn("Rolling renewal failed - permission denied",
				append([]zap.Field{logging.TypeField(logging.TypeApplication)}, append(logFields, zap.Error(err))...)...,
			)
		} else {
			h.logger.Error("Rolling renewal failed - FOLIO API error",
				append([]zap.Field{logging.TypeField(logging.TypeApplication)}, append(logFields, zap.Error(err))...)...,
			)
		}
		return
	}

	// Success!
	h.logger.Info("Rolling renewal successful",
		append([]zap.Field{logging.TypeField(logging.TypeApplication)}, logFields...)...,
	)
}
