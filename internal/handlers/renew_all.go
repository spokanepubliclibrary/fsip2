package handlers

import (
	"context"
	"time"

	"github.com/spokanepubliclibrary/fsip2/internal/config"
	"github.com/spokanepubliclibrary/fsip2/internal/folio"
	"github.com/spokanepubliclibrary/fsip2/internal/logging"
	"github.com/spokanepubliclibrary/fsip2/internal/sip2/builder"
	"github.com/spokanepubliclibrary/fsip2/internal/sip2/parser"
	"github.com/spokanepubliclibrary/fsip2/internal/types"
	"go.uber.org/zap"
)

// RenewAllHandler handles SIP2 Renew All requests (65)
type RenewAllHandler struct {
	*BaseHandler
	logger *zap.Logger
}

// NewRenewAllHandler creates a new renew all handler
func NewRenewAllHandler(logger *zap.Logger, tenantConfig *config.TenantConfig) *RenewAllHandler {
	return &RenewAllHandler{
		BaseHandler: NewBaseHandler(logger, tenantConfig),
		logger:      logger.With(logging.TypeField(logging.TypeApplication)),
	}
}

// Handle processes a Renew All request (65) and returns a Renew All response (66)
func (h *RenewAllHandler) Handle(ctx context.Context, msg *parser.Message, session *types.Session) (string, error) {
	h.logRequest(msg, session)

	// Validate required fields
	if err := h.validateRequiredFields(msg, map[parser.FieldCode]string{
		parser.PatronIdentifier: "Patron Identifier",
	}); err != nil {
		h.logger.Error("Renew all validation failed", zap.Error(err))
		return h.buildErrorResponse(msg, session), nil
	}

	// Extract fields
	institutionID := msg.GetField(parser.InstitutionID)
	patronIdentifier := msg.GetField(parser.PatronIdentifier)
	patronPassword := msg.GetField(parser.PatronPassword)

	h.logger.Info("Renew all request",
		zap.String("institution_id", institutionID),
		zap.String("patron_identifier", patronIdentifier),
	)

	// Get authenticated FOLIO client
	_, token, err := h.getAuthenticatedFolioClient(ctx, session)
	if err != nil {
		h.logger.Error("Failed to get authenticated client", zap.Error(err))
		return h.buildErrorResponse(msg, session), nil
	}

	// Create clients
	patronClient := h.getPatronClient(session)
	circClient := h.getCirculationClient(session)
	inventoryClient := h.getInventoryClient(session)

	// Get patron ID and user info (from session or lookup)
	patronID := session.GetPatronID()
	if patronID != "" && session.GetPatronBarcode() != patronIdentifier {
		h.logger.Info("Patron barcode mismatch — invalidating cached patron ID",
			zap.String("cached_barcode", session.GetPatronBarcode()),
			zap.String("request_barcode", patronIdentifier),
		)
		patronID = ""
	}
	var userID string
	if patronID == "" {
		// Look up patron by barcode
		user, err := patronClient.GetUserByBarcode(ctx, token, patronIdentifier)
		if err != nil {
			h.logger.Error("Failed to get patron",
				zap.String("patron_identifier", patronIdentifier),
				zap.Error(err),
			)
			return h.buildErrorResponseWithMessage(msg, session, GetVerificationErrorMessage()), nil
		}
		patronID = user.ID
		userID = user.ID
		session.SetPatronBarcode(patronIdentifier)
		session.SetPatronID(user.ID)
	} else {
		userID = patronID
	}

	// Verify patron password/PIN if required (only once at beginning)
	verifyResult := VerifyPatronCredentials(
		ctx,
		h.logger,
		session,
		patronClient,
		token,
		userID,
		patronIdentifier,
		patronPassword,
	)

	if verifyResult.Required && !verifyResult.Verified {
		h.logger.Info("Renew all blocked due to failed patron verification",
			zap.String("patron_identifier", patronIdentifier),
		)
		return h.buildErrorResponseWithMessage(msg, session, GetVerificationErrorMessage()), nil
	}

	// Get open loans for the patron
	loanCollection, err := circClient.GetOpenLoansByUser(ctx, token, patronID)
	if err != nil {
		h.logger.Error("Failed to get patron loans",
			zap.String("patron_id", patronID),
			zap.Error(err),
		)
		return h.buildErrorResponse(msg, session), nil
	}

	// Limit to configured max items (default: 50)
	maxItems := session.TenantConfig.GetRenewAllMaxItems()
	loans := loanCollection.Loans
	if len(loans) > maxItems {
		loans = loans[:maxItems]
		h.logger.Info("Limited renewal to max items",
			zap.Int("total_loans", len(loanCollection.Loans)),
			zap.Int("max_items", maxItems),
		)
	}

	// Track renewed and unrenewed items
	var renewedItems []string
	var unrenewedItems []string

	// Process each loan individually
	for _, loan := range loans {
		// Try to renew using RenewByID
		renewReq := folio.RenewByIDRequest{
			UserID: patronID,
			ItemID: loan.ItemID,
		}

		renewedLoan, err := circClient.RenewByID(ctx, token, renewReq)
		if err != nil {
			// Renewal failed - get item barcode for BN field
			h.logger.Warn("Renewal failed for item",
				zap.String("item_id", loan.ItemID),
				zap.Error(err),
			)

			// Fetch item barcode
			item, itemErr := inventoryClient.GetItemByID(ctx, token, loan.ItemID)
			if itemErr != nil {
				h.logger.Error("Failed to get item barcode for failed renewal",
					zap.String("item_id", loan.ItemID),
					zap.Error(itemErr),
				)
				// Use item UUID as fallback
				unrenewedItems = append(unrenewedItems, loan.ItemID)
			} else {
				unrenewedItems = append(unrenewedItems, item.Barcode)
			}
		} else {
			// Renewal succeeded - get item barcode for BM field
			h.logger.Info("Renewal succeeded",
				zap.String("item_id", renewedLoan.ItemID),
				zap.Time("due_date", *renewedLoan.DueDate),
			)

			// Fetch item barcode
			item, itemErr := inventoryClient.GetItemByID(ctx, token, renewedLoan.ItemID)
			if itemErr != nil {
				h.logger.Error("Failed to get item barcode for successful renewal",
					zap.String("item_id", renewedLoan.ItemID),
					zap.Error(itemErr),
				)
				// Use item UUID as fallback
				renewedItems = append(renewedItems, renewedLoan.ItemID)
			} else {
				renewedItems = append(renewedItems, item.Barcode)
			}
		}
	}

	h.logger.Info("Renew all completed",
		zap.String("patron_id", patronID),
		zap.Int("renewed", len(renewedItems)),
		zap.Int("unrenewed", len(unrenewedItems)),
	)

	h.logResponse(string(parser.RenewAllResponse), session, nil)

	// Build response using ResponseBuilder
	responseBuilder := builder.NewResponseBuilder(session.TenantConfig)

	ok := len(renewedItems) > 0
	screenMessages := []string{}
	if len(renewedItems) > 0 && len(unrenewedItems) > 0 {
		screenMessages = append(screenMessages, "Some items could not be renewed")
	} else if len(renewedItems) == 0 && len(unrenewedItems) > 0 {
		screenMessages = append(screenMessages, "No items could be renewed")
	}

	response, err := responseBuilder.BuildRenewAllResponse(
		ok,
		len(renewedItems),
		len(unrenewedItems),
		time.Now(),
		institutionID,
		patronIdentifier,
		renewedItems,
		unrenewedItems,
		screenMessages,
		msg.SequenceNumber,
	)
	if err != nil {
		h.logger.Error("Failed to build renew all response", zap.Error(err))
		return h.buildErrorResponse(msg, session), nil
	}

	return response, nil
}

// buildErrorResponse builds an error response for renew all
func (h *RenewAllHandler) buildErrorResponse(msg *parser.Message, session *types.Session) string {
	return h.buildErrorResponseWithMessage(msg, session, "Renewal failed")
}

// buildErrorResponseWithMessage builds an error response for renew all with a custom message
func (h *RenewAllHandler) buildErrorResponseWithMessage(msg *parser.Message, session *types.Session, errorMessage string) string {
	institutionID := msg.GetField(parser.InstitutionID)
	patronIdentifier := msg.GetField(parser.PatronIdentifier)

	responseBuilder := builder.NewResponseBuilder(session.TenantConfig)

	response, err := responseBuilder.BuildRenewAllResponse(
		false, // ok
		0,     // renewedCount
		0,     // unrenewedCount
		time.Now(),
		institutionID,
		patronIdentifier,
		[]string{},             // renewedItems
		[]string{},             // unrenewedItems
		[]string{errorMessage}, // screenMessage
		msg.SequenceNumber,
	)
	if err != nil {
		h.logger.Error("Failed to build error response", zap.Error(err))
		return "66N00000000" + time.Now().Format("20060102    150405") + "|AO" + institutionID + "|AA" + patronIdentifier + "|AF" + errorMessage
	}

	return response
}
