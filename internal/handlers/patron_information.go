package handlers

import (
	"context"
	"fmt"
	"time"

	"github.com/spokanepubliclibrary/fsip2/internal/config"
	"github.com/spokanepubliclibrary/fsip2/internal/folio/models"
	"github.com/spokanepubliclibrary/fsip2/internal/logging"
	"github.com/spokanepubliclibrary/fsip2/internal/sip2/builder"
	"github.com/spokanepubliclibrary/fsip2/internal/sip2/customfields"
	"github.com/spokanepubliclibrary/fsip2/internal/sip2/parser"
	"github.com/spokanepubliclibrary/fsip2/internal/sip2/protocol"
	"github.com/spokanepubliclibrary/fsip2/internal/types"
	"go.uber.org/zap"
)

// PatronInformationHandler handles SIP2 Patron Information requests (63)
type PatronInformationHandler struct {
	*BaseHandler
	logger *zap.Logger
}

// NewPatronInformationHandler creates a new patron information handler
func NewPatronInformationHandler(logger *zap.Logger, tenantConfig *config.TenantConfig) *PatronInformationHandler {
	return &PatronInformationHandler{
		BaseHandler: NewBaseHandler(logger, tenantConfig),
		logger:      logger.With(logging.TypeField(logging.TypeApplication)),
	}
}

// Handle processes a Patron Information request (63) and returns a Patron Information response (64)
func (h *PatronInformationHandler) Handle(ctx context.Context, msg *parser.Message, session *types.Session) (string, error) {
	h.logRequest(msg, session)

	// Validate required fields
	if err := h.validateRequiredFields(msg, map[parser.FieldCode]string{
		parser.PatronIdentifier: "Patron Identifier",
	}); err != nil {
		h.logger.Error("Patron information validation failed", zap.Error(err))
		return h.buildErrorResponse(msg), fmt.Errorf("validation failed: %w", err)
	}

	// Extract fields
	institutionID := msg.GetField(parser.InstitutionID)
	patronIdentifier := msg.GetField(parser.PatronIdentifier)
	patronPassword := msg.GetField(parser.PatronPassword)

	// Extract fixed fields from the message
	language := msg.Fields["language"]
	if language == "" {
		language = "000" // Default to English
	}

	summary := msg.Fields["summary"]
	if summary == "" {
		summary = "          " // 10 spaces for default (no specific info requested)
	}

	h.logger.Info("Patron information request",
		zap.String("institution_id", institutionID),
		zap.String("patron_identifier", patronIdentifier),
		zap.String("session_id", session.ID),
		zap.Bool("is_authenticated", session.IsAuth()),
		zap.String("username", session.GetUsername()),
		zap.Bool("has_patron_password", patronPassword != ""),
		zap.String("language", language),
		zap.String("summary", summary),
	)

	// Get system-level authentication token from session (should be set by Login 93)
	_, token, err := h.getAuthenticatedFolioClient(ctx, session)
	if err != nil {
		h.logger.Error("Failed to get system authentication token",
			zap.Error(err),
			zap.String("hint", "Login (93) message must be sent first to authenticate the system user"),
		)
		return h.buildPatronInformationResponse(nil, nil, nil, nil, nil, nil, nil, nil, nil, institutionID, patronIdentifier, language, summary, false, false, msg.SequenceNumber, session), nil
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
		return h.buildPatronInformationResponse(nil, nil, nil, nil, nil, nil, nil, nil, nil, institutionID, patronIdentifier, language, summary, false, false, msg.SequenceNumber, session), nil
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
		return h.buildPatronInformationResponse(nil, nil, nil, nil, nil, nil, nil, nil, nil, institutionID, patronIdentifier, language, summary, false, false, msg.SequenceNumber, session), nil
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
		return h.buildPatronInformationResponse(nil, nil, nil, nil, nil, nil, nil, nil, nil, institutionID, patronIdentifier, language, summary, false, false, msg.SequenceNumber, session), nil
	}

	// Parse summary to determine what information to retrieve
	// Position 0: hold items (Y/N)
	// Position 1: overdue items (Y/N)
	// Position 2: charged items (Y/N)
	// Position 3: fine items (Y/N)
	// Position 4: recall items (Y/N)
	// Position 5: unavailable hold items (Y/N)

	includeHolds := len(summary) > 0 && summary[0] == 'Y'
	includeOverdue := len(summary) > 1 && summary[1] == 'Y'
	includeCharged := len(summary) > 2 && summary[2] == 'Y'
	includeFines := len(summary) > 3 && summary[3] == 'Y'
	includeUnavailableHolds := len(summary) > 5 && summary[5] == 'Y'

	h.logger.Debug("Parsed summary flags",
		zap.String("summary_raw", summary),
		zap.Int("summary_length", len(summary)),
		zap.String("summary_bytes", fmt.Sprintf("%v", []byte(summary))),
		zap.Bool("include_holds", includeHolds),
		zap.Bool("include_overdue", includeOverdue),
		zap.Bool("include_charged", includeCharged),
		zap.Bool("include_fines", includeFines),
		zap.Bool("include_unavailable_holds", includeUnavailableHolds),
	)

	// Create circulation and inventory clients for retrieving loans and items
	circulationClient := h.getCirculationClient(session)
	inventoryClient := h.getInventoryClient(session)

	// ============================================================================
	// DATA FETCHING - Per SIP2 specification, counts in fixed fields (positions
	// 37-60) must ALWAYS reflect actual totals on the patron account, regardless
	// of what the summary field requests. The summary field only controls whether
	// item details (barcodes) appear in variable fields (AS, AT, AU, AV, CD).
	// ============================================================================

	// ALWAYS fetch patron's holds for accurate counts (available holds - ready for pickup)
	var holds []*models.Request
	availableHolds, err := circulationClient.GetAvailableHolds(ctx, token, user.ID)
	if err != nil {
		h.logger.Warn("Failed to get available holds",
			zap.String("patron_id", user.ID),
			zap.Error(err),
		)
	} else {
		// Convert to slice of pointers for easier handling
		for i := range availableHolds.Requests {
			holds = append(holds, &availableHolds.Requests[i])
		}
	}

	// ALWAYS fetch patron's loans for accurate counts
	var loans []*models.Loan
	var overdueLoans []*models.Loan
	loansCollection, err := circulationClient.GetOpenLoansByUser(ctx, token, user.ID)
	if err != nil {
		h.logger.Warn("Failed to get patron loans",
			zap.String("patron_id", user.ID),
			zap.Error(err),
		)
	} else {
		// Get item details for each loan
		for i := range loansCollection.Loans {
			loan := &loansCollection.Loans[i]

			// Fetch item details to get barcode
			item, err := inventoryClient.GetItemByID(ctx, token, loan.ItemID)
			if err != nil {
				h.logger.Warn("Failed to get item for loan",
					zap.String("loan_id", loan.ID),
					zap.String("item_id", loan.ItemID),
					zap.Error(err),
				)
				continue
			}

			// Attach item to loan
			loan.Item = item

			// Always add to all loans for count
			loans = append(loans, loan)

			// Always separate overdue loans for count
			if loan.IsOverdue() {
				overdueLoans = append(overdueLoans, loan)
			}
		}
	}

	// ALWAYS fetch patron's fines for accurate counts
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

	// ALWAYS fetch patron's unavailable holds for accurate counts (holds not yet filled or in transit)
	var unavailableHolds []*models.Request
	unavailableHoldsCollection, err := circulationClient.GetUnavailableHolds(ctx, token, user.ID)
	if err != nil {
		h.logger.Warn("Failed to get unavailable holds",
			zap.String("patron_id", user.ID),
			zap.Error(err),
		)
	} else {
		// Convert to slice of pointers for easier handling
		for i := range unavailableHoldsCollection.Requests {
			unavailableHolds = append(unavailableHolds, &unavailableHoldsCollection.Requests[i])
		}
	}

	// Get patron group information (only if FU or FV fields are enabled)
	var patronGroup *models.PatronGroup
	if user.PatronGroup != "" && (session.TenantConfig.IsFieldEnabled("63", "FU") || session.TenantConfig.IsFieldEnabled("63", "FV")) {
		group, err := patronClient.GetPatronGroupByID(ctx, token, user.PatronGroup)
		if err != nil {
			h.logger.Warn("Failed to get patron group information",
				zap.String("patron_id", user.ID),
				zap.String("patron_group_id", user.PatronGroup),
				zap.Error(err),
			)
			// Continue without patron group info - it's not critical
		} else {
			patronGroup = group
		}
	}

	// Get patron blocks for building the 14-character patron status
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

	h.logger.Info("Patron information retrieved",
		zap.String("patron_id", user.ID),
		zap.Int("holds", len(holds)),
		zap.Int("loans", len(loans)),
		zap.Int("overdue_loans", len(overdueLoans)),
		zap.Int("accounts", len(accounts)),
		zap.Int("unavailable_holds", len(unavailableHolds)),
	)

	h.logResponse(string(parser.PatronInformationResponse), session, nil)

	return h.buildPatronInformationResponse(user, manualBlocks, automatedBlocks, holds, loans, overdueLoans, accounts, unavailableHolds, patronGroup, institutionID, patronIdentifier, language, summary, true, pinVerified, msg.SequenceNumber, session), nil
}

// buildPatronInformationResponse builds a Patron Information Response (64)
//
// Per SIP2 specification, the fixed field counts (hold_items_count, overdue_items_count,
// charged_items_count, fine_items_count, recall_items_count, unavailable_holds_count)
// ALWAYS reflect the actual totals on the patron account.
//
// The summary parameter controls only the variable field details:
//   - Position 0 (Y): Include AS (hold item barcodes)
//   - Position 1 (Y): Include AT (overdue item barcodes)
//   - Position 2 (Y): Include AU (charged item barcodes)
//   - Position 3 (Y): Include AV (fine details)
//   - Position 5 (Y): Include CD (unavailable hold items)
//
// Note: BV (total balance) and BH (currency) are always included when accounts exist,
// as they represent aggregate patron information, not individual item details.
func (h *PatronInformationHandler) buildPatronInformationResponse(
	user interface{},
	manualBlocks interface{},
	automatedBlocks interface{},
	holds []*models.Request,
	loans []*models.Loan,
	overdueLoans []*models.Loan,
	accounts []*models.Account,
	unavailableHolds []*models.Request,
	patronGroup *models.PatronGroup,
	institutionID string,
	patronIdentifier string,
	language string,
	summary string,
	valid bool,
	pinVerified bool,
	sequenceNumber string,
	session *types.Session,
) string {
	timestamp := protocol.FormatSIP2DateTime(time.Now(), "    ")

	// Patron Information Response format:
	// 64<patron_status><language><transaction_date><hold_items_count>
	// <overdue_items_count><charged_items_count><fine_items_count>
	// <recall_items_count><unavailable_holds_count><institution_id>
	// <patron_identifier><personal_name><hold_items_limit><overdue_items_limit>
	// <charged_items_limit><valid_patron><valid_patron_password>
	// <currency_type><fee_amount><fee_limit><items><hold_items>
	// <overdue_items><charged_items><fine_items><recall_items>
	// <unavailable_hold_items><home_address><email><phone><screen_message>
	// <print_line>

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

	if language == "" {
		language = "000" // English
	}

	// Count items
	holdItemsCount := fmt.Sprintf("%04d", len(holds))
	overdueItemsCount := fmt.Sprintf("%04d", len(overdueLoans))
	chargedItemsCount := fmt.Sprintf("%04d", len(loans))
	fineItemsCount := fmt.Sprintf("%04d", len(accounts))
	recallItemsCount := "0000"
	unavailableHoldsCount := fmt.Sprintf("%04d", len(unavailableHolds))

	// Parse summary to determine what variable field details to include
	// Position 0: hold items (Y/N)
	// Position 1: overdue items (Y/N)
	// Position 2: charged items (Y/N)
	// Position 3: fine items (Y/N)
	// Position 4: recall items (Y/N)
	// Position 5: unavailable hold items (Y/N)
	includeHolds := len(summary) > 0 && summary[0] == 'Y'
	includeOverdue := len(summary) > 1 && summary[1] == 'Y'
	includeCharged := len(summary) > 2 && summary[2] == 'Y'
	includeFines := len(summary) > 3 && summary[3] == 'Y'
	includeUnavailableHolds := len(summary) > 5 && summary[5] == 'Y'

	response := fmt.Sprintf("64%s%s%s%s%s%s%s%s%s",
		patronStatus,
		language,
		timestamp,
		holdItemsCount,
		overdueItemsCount,
		chargedItemsCount,
		fineItemsCount,
		recallItemsCount,
		unavailableHoldsCount,
	)

	// Add variable fields
	response += fmt.Sprintf("|AO%s", institutionID)
	response += fmt.Sprintf("|AA%s", patronIdentifier)

	if valid && user != nil {
		// Add patron name using helper (consolidates duplicated logic)
		if u, ok := user.(*models.User); ok {
			patronName := h.formatPatronName(u)
			response += fmt.Sprintf("|AE%s", patronName)

			// Add contact information fields
			// BD - Primary address
			if addr := u.GetPrimaryAddress(); addr != nil {
				addressParts := []string{}
				if addr.AddressLine1 != "" {
					addressParts = append(addressParts, addr.AddressLine1)
				}
				if addr.AddressLine2 != "" {
					addressParts = append(addressParts, addr.AddressLine2)
				}
				if addr.City != "" {
					addressParts = append(addressParts, addr.City)
				}
				if addr.Region != "" {
					addressParts = append(addressParts, addr.Region)
				}
				if addr.PostalCode != "" {
					addressParts = append(addressParts, addr.PostalCode)
				}
				if len(addressParts) > 0 {
					address := ""
					for i, part := range addressParts {
						if i > 0 {
							address += ", "
						}
						address += part
					}
					response += fmt.Sprintf("|BD%s", address)
				}
			}

			// BE - Email
			if u.Personal.Email != "" {
				response += fmt.Sprintf("|BE%s", u.Personal.Email)
			}

			// BF - Phone (configurable)
			if session.TenantConfig.IsFieldEnabled("63", "BF") && u.Personal.Phone != "" {
				response += fmt.Sprintf("|BF%s", u.Personal.Phone)
			}

			// BG - Mobile Phone (configurable)
			if session.TenantConfig.IsFieldEnabled("63", "BG") && u.Personal.MobilePhone != "" {
				response += fmt.Sprintf("|BG%s", u.Personal.MobilePhone)
			}

			// PC - Patron Group UUID (configurable)
			if session.TenantConfig.IsFieldEnabled("63", "PC") && u.PatronGroup != "" {
				response += fmt.Sprintf("|PC%s", u.PatronGroup)
			}

			// PB - Birthdate in MMDDYYYY format (configurable)
			if session.TenantConfig.IsFieldEnabled("63", "PB") && u.Personal.DateOfBirth != nil {
				birthdate := u.Personal.DateOfBirth.Format("01022006")
				response += fmt.Sprintf("|PB%s", birthdate)
			}
		}

		// Add patron group information if available
		if patronGroup != nil {
			// FU - Patron Group name (configurable)
			if session.TenantConfig.IsFieldEnabled("63", "FU") && patronGroup.Group != "" {
				response += fmt.Sprintf("|FU%s", patronGroup.Group)
			}

			// FV - Patron Group description (configurable)
			if session.TenantConfig.IsFieldEnabled("63", "FV") && patronGroup.Desc != "" {
				response += fmt.Sprintf("|FV%s", patronGroup.Desc)
			}
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

	// AS - Add available holds (item barcodes for items ready for pickup)
	// Only include details if summary requests holds
	if includeHolds && len(holds) > 0 {
		for _, hold := range holds {
			if hold.Item != nil && hold.Item.Barcode != "" {
				response += fmt.Sprintf("|AS%s", hold.Item.Barcode)
			}
		}
	}

	// AT - Add overdue items (item barcodes)
	// Only include details if summary requests overdue items
	if includeOverdue && len(overdueLoans) > 0 {
		for _, loan := range overdueLoans {
			if loan.Item != nil && loan.Item.Barcode != "" {
				response += fmt.Sprintf("|AT%s", loan.Item.Barcode)
			}
		}
	}

	// AU - Add charged items (item barcodes)
	// Only include details if summary requests charged items
	if includeCharged && len(loans) > 0 {
		for _, loan := range loans {
			if loan.Item != nil && loan.Item.Barcode != "" {
				response += fmt.Sprintf("|AU%s", loan.Item.Barcode)
			}
		}
	}

	// CD - Add unavailable holds (item barcode or instanceId for holds not yet filled or in transit)
	// Prefer item.barcode if available, otherwise use instanceId (configurable)
	// Only include details if summary requests unavailable holds
	if includeUnavailableHolds && session.TenantConfig.IsFieldEnabled("63", "CD") && len(unavailableHolds) > 0 {
		for _, hold := range unavailableHolds {
			if hold.Item != nil && hold.Item.Barcode != "" {
				response += fmt.Sprintf("|CD%s", hold.Item.Barcode)
			} else if hold.InstanceID != "" {
				response += fmt.Sprintf("|CD%s", hold.InstanceID)
			}
		}
	}

	// AV - Add account/fee details if requested and available
	// Format: <accountID> <remaining> "<feeFineType>" <title>
	// Only include details if summary requests fine items
	if includeFines && len(accounts) > 0 {
		for _, account := range accounts {
			// Format remaining amount without currency symbol
			remainingStr := fmt.Sprintf("%.2f", account.Remaining.Float64())

			// Truncate title to 60 characters if needed
			title := account.Title
			if len(title) > 60 {
				title = title[:60]
			}

			// Format: accountID remaining "feeFineType" title
			// FeeFineType is always quoted, title can be blank
			avValue := fmt.Sprintf("%s %s \"%s\" %s", account.ID, remainingStr, account.FeeFineType, title)
			response += fmt.Sprintf("|AV%s", avValue)
		}
	}

	// Add custom fields if configured
	if session.TenantConfig.PatronCustomFields != nil &&
		session.TenantConfig.PatronCustomFields.Enabled {

		if u, ok := user.(*models.User); ok {
			customFieldsList := customfields.ProcessCustomFields(
				u,
				session.TenantConfig.PatronCustomFields,
				session.TenantConfig.FieldDelimiter,
				h.logger,
			)

			for _, field := range customFieldsList {
				response += field
			}

			h.logger.Debug("Added custom fields to patron response",
				zap.Int("count", len(customFieldsList)),
				zap.String("patron_id", u.ID),
			)
		}
	}

	// Use ResponseBuilder to add AY (sequence number) and AZ (checksum) if error detection is enabled
	sessionBuilder := h.builder
	if session != nil && session.TenantConfig != nil {
		sessionBuilder = builder.NewResponseBuilder(session.TenantConfig)
	}

	// Remove the "64" prefix from response as the builder will add it
	content := response[2:]

	// Use builder to add sequence number, checksum, and delimiter
	finalResponse, err := sessionBuilder.Build(parser.PatronInformationResponse, content, sequenceNumber)
	if err != nil {
		h.logger.Error("Failed to build patron information response", zap.Error(err))
		// Fallback to simple response without checksum
		return response
	}

	return finalResponse
}
