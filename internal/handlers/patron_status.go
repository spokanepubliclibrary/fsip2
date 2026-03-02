package handlers

import (
	"context"
	"fmt"
	"time"

	"github.com/spokanepubliclibrary/fsip2/internal/config"
	"github.com/spokanepubliclibrary/fsip2/internal/folio/models"
	"github.com/spokanepubliclibrary/fsip2/internal/sip2/builder"
	"github.com/spokanepubliclibrary/fsip2/internal/sip2/parser"
	"github.com/spokanepubliclibrary/fsip2/internal/sip2/protocol"
	"github.com/spokanepubliclibrary/fsip2/internal/types"
	"go.uber.org/zap"
)

// PatronStatusHandler handles SIP2 Patron Status requests (23)
type PatronStatusHandler struct {
	*BaseHandler
	logger *zap.Logger
}

// NewPatronStatusHandler creates a new patron status handler
func NewPatronStatusHandler(logger *zap.Logger, tenantConfig *config.TenantConfig) *PatronStatusHandler {
	return &PatronStatusHandler{
		BaseHandler: NewBaseHandler(logger, tenantConfig),
		logger:      logger,
	}
}

// Handle processes a Patron Status request (23) and returns a Patron Status response (24)
func (h *PatronStatusHandler) Handle(ctx context.Context, msg *parser.Message, session *types.Session) (string, error) {
	h.logRequest(msg, session)

	// Validate required fields
	if err := h.validateRequiredFields(msg, map[parser.FieldCode]string{
		parser.InstitutionID:    "Institution ID",
		parser.PatronIdentifier: "Patron Identifier",
	}); err != nil {
		h.logger.Error("Patron status validation failed", zap.Error(err))
		return h.buildErrorResponse(msg), fmt.Errorf("validation failed: %w", err)
	}

	// Extract fields
	institutionID := msg.GetField(parser.InstitutionID)
	patronIdentifier := msg.GetField(parser.PatronIdentifier)
	// terminalPassword := msg.GetField(parser.TerminalPassword)
	patronPassword := msg.GetField(parser.PatronPassword)

	h.logger.Info("Patron status request",
		zap.String("institution_id", institutionID),
		zap.String("patron_identifier", patronIdentifier),
		zap.Bool("has_patron_password", patronPassword != ""),
	)

	// Get system-level authentication token from session (should be set by Login 93)
	_, token, err := h.getAuthenticatedFolioClient(ctx, session)
	if err != nil {
		h.logger.Error("Failed to get system authentication token",
			zap.Error(err),
			zap.String("hint", "Login (93) message must be sent first to authenticate the system user"),
		)
		return h.buildPatronStatusResponse(nil, nil, nil, nil, institutionID, patronIdentifier, false, false, msg.SequenceNumber, session), nil
	}

	// Create patron client (uses system token for all lookups)
	patronClient := h.getPatronClient(session)

	// Get patron information from FOLIO using system token
	user, err := patronClient.GetUserByBarcode(ctx, token, patronIdentifier)
	if err != nil {
		h.logger.Error("Failed to get patron information",
			zap.String("patron_identifier", patronIdentifier),
			zap.Error(err),
		)
		return h.buildPatronStatusResponse(nil, nil, nil, nil, institutionID, patronIdentifier, false, false, msg.SequenceNumber, session), nil
	}

	// Store original active status before attempting rolling renewal
	wasInactive := !user.Active

	// Attempt rolling renewal BEFORE checking active status
	// This allows inactive expired accounts to be reactivated if extendExpired is enabled
	h.attemptRollingRenewal(ctx, user, token, session)

	// If user was inactive and rolling renewals with extendExpired is enabled, re-fetch user to get updated status
	if wasInactive && session.TenantConfig.IsRollingRenewalEnabled() {
		renewalConfig := session.TenantConfig.GetRollingRenewalConfig()
		if renewalConfig != nil && renewalConfig.ExtendExpired {
			h.logger.Debug("Re-fetching user after rolling renewal attempt (was inactive)",
				zap.String("patron_id", user.ID),
			)
			updatedUser, err := patronClient.GetUserByID(ctx, token, user.ID)
			if err != nil {
				h.logger.Warn("Failed to re-fetch user after rolling renewal",
					zap.String("patron_id", user.ID),
					zap.Error(err),
				)
				// Continue with original user data
			} else {
				user = updatedUser
				h.logger.Debug("User re-fetched successfully",
					zap.String("patron_id", user.ID),
					zap.Bool("now_active", user.Active),
				)
			}
		}
	}

	// Check if patron account is inactive (after rolling renewal attempt)
	if !user.Active {
		h.logger.Info("Patron account is inactive",
			zap.String("patron_identifier", patronIdentifier),
			zap.String("patron_id", user.ID),
		)
		return h.buildPatronStatusResponse(nil, nil, nil, nil, institutionID, patronIdentifier, false, false, msg.SequenceNumber, session), nil
	}

	// Verify patron password/PIN if required
	verifyResult := VerifyPatronCredentials(
		ctx,
		h.logger,
		session,
		patronClient,
		token,
		user.ID,
		patronIdentifier,
		patronPassword,
	)

	patronValid := true
	pinVerified := false

	if verifyResult.Required && !verifyResult.Verified {
		h.logger.Info("Patron verification failed",
			zap.String("patron_identifier", patronIdentifier),
		)
		patronValid = false
	} else if verifyResult.Verified {
		pinVerified = true
	}

	// If patron verification failed, return invalid patron response
	if !patronValid {
		return h.buildPatronStatusResponse(nil, nil, nil, nil, institutionID, patronIdentifier, false, false, msg.SequenceNumber, session), nil
	}

	// Get patron blocks
	manualBlocks, err := patronClient.GetManualBlocks(ctx, token, user.ID)
	if err != nil {
		h.logger.Warn("Failed to get manual blocks, continuing without blocks",
			zap.String("patron_id", user.ID),
			zap.Error(err),
		)
	}

	automatedBlocks, err := patronClient.GetAutomatedPatronBlocks(ctx, token, user.ID)
	if err != nil {
		h.logger.Warn("Failed to get automated blocks, continuing without blocks",
			zap.String("patron_id", user.ID),
			zap.Error(err),
		)
	}

	// Get patron's fines/fees
	var accounts []*models.Account
	feesClient := h.getFeesClient(session)
	accountsCollection, err := feesClient.GetOpenAccountsExcludingSuspended(ctx, token, user.ID)
	if err != nil {
		h.logger.Warn("Failed to get patron accounts",
			zap.String("patron_id", user.ID),
			zap.Error(err),
		)
	} else {
		// Convert to slice of pointers for easier handling
		for i := range accountsCollection.Accounts {
			accounts = append(accounts, &accountsCollection.Accounts[i])
		}
	}

	// Update session with patron information using SetAuthenticated
	currentUsername := session.GetUsername()
	if currentUsername == "" {
		currentUsername = patronIdentifier
	}
	currentToken := session.GetAuthToken()
	currentExpiresAt := session.GetTokenExpiresAt()
	session.SetAuthenticated(currentUsername, user.ID, user.Barcode, currentToken, currentExpiresAt)

	h.logger.Info("Patron status retrieved",
		zap.String("patron_id", user.ID),
		zap.String("patron_name", user.Personal.LastName),
		zap.Int("accounts", len(accounts)),
	)

	h.logResponse(string(parser.PatronStatusResponse), session, nil)

	return h.buildPatronStatusResponse(user, manualBlocks, automatedBlocks, accounts, institutionID, patronIdentifier, true, pinVerified, msg.SequenceNumber, session), nil
}

// buildPatronStatusResponse builds a Patron Status Response (24)
func (h *PatronStatusHandler) buildPatronStatusResponse(
	user interface{},
	manualBlocks interface{},
	automatedBlocks interface{},
	accounts []*models.Account,
	institutionID string,
	patronIdentifier string,
	valid bool,
	pinVerified bool,
	sequenceNumber string,
	session *types.Session,
) string {
	timestamp := protocol.FormatSIP2DateTime(time.Now(), "    ")

	// Build 14-character patron status
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

	// Build 14-character patron status using helper (consolidates duplicated logic)
	patronStatus := h.buildPatronStatusString(valid, user, manualBlocks, automatedBlocks)

	language := "000" // English

	// Build response
	response := fmt.Sprintf("24%s%s%s",
		patronStatus,
		language,
		timestamp,
	)

	// Add variable fields
	response += fmt.Sprintf("|AO%s", institutionID)
	response += fmt.Sprintf("|AA%s", patronIdentifier)

	if valid && user != nil {
		// Add patron name using helper (consolidates duplicated logic)
		if u, ok := user.(*models.User); ok {
			patronName := h.formatPatronName(u)
			response += fmt.Sprintf("|AE%s", patronName)
		}

		// Add valid patron flag
		response += "|BLY" // Valid patron

		// CQ - Valid patron PIN (Y/N)
		if pinVerified {
			response += "|CQY"
		} else {
			response += "|CQN"
		}

		// Add currency type and fee amount if there are fines
		if len(accounts) > 0 {
			// BV - Calculate total outstanding balance
			totalOutstanding := 0.0
			for _, account := range accounts {
				totalOutstanding += account.Remaining.Float64()
			}
			response += fmt.Sprintf("|BV%.2f", totalOutstanding)

			// BH - Currency type from tenant config
			currency := session.TenantConfig.Currency
			if currency == "" {
				currency = "USD" // Default to USD if not configured
			}
			response += fmt.Sprintf("|BH%s", currency)
		}
	} else {
		response += "|BLN" // Invalid patron
		response += "|CQN" // Invalid patron PIN
	}

	// Add screen message if patron not found
	if !valid {
		response += "|AFPatron not found"
	}

	// Use ResponseBuilder to add AY (sequence number) and AZ (checksum) if error detection is enabled
	sessionBuilder := h.builder
	if session != nil && session.TenantConfig != nil {
		sessionBuilder = builder.NewResponseBuilder(session.TenantConfig)
	}

	// Remove the "24" prefix from response as the builder will add it
	content := response[2:]

	// Use builder to add sequence number, checksum, and delimiter
	finalResponse, err := sessionBuilder.Build(parser.PatronStatusResponse, content, sequenceNumber)
	if err != nil {
		h.logger.Error("Failed to build patron status response", zap.Error(err))
		// Fallback to simple response without checksum
		return response
	}

	return finalResponse
}
