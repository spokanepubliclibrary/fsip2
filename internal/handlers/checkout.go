package handlers

import (
	"context"
	"sync"
	"time"

	"github.com/spokanepubliclibrary/fsip2/internal/config"
	"github.com/spokanepubliclibrary/fsip2/internal/folio"
	"github.com/spokanepubliclibrary/fsip2/internal/logging"
	"github.com/spokanepubliclibrary/fsip2/internal/sip2/builder"
	"github.com/spokanepubliclibrary/fsip2/internal/sip2/parser"
	"github.com/spokanepubliclibrary/fsip2/internal/sip2/protocol"
	"github.com/spokanepubliclibrary/fsip2/internal/types"
	"go.uber.org/zap"
)

// CheckoutHandler handles SIP2 Checkout requests (11)
type CheckoutHandler struct {
	*BaseHandler
	logger *zap.Logger
}

// NewCheckoutHandler creates a new checkout handler
func NewCheckoutHandler(logger *zap.Logger, tenantConfig *config.TenantConfig) *CheckoutHandler {
	return &CheckoutHandler{
		BaseHandler: NewBaseHandler(logger, tenantConfig),
		logger:      logger.With(logging.TypeField(logging.TypeApplication)),
	}
}

// Handle processes a Checkout request (11) and returns a Checkout response (12)
func (h *CheckoutHandler) Handle(ctx context.Context, msg *parser.Message, session *types.Session) (string, error) {
	h.logRequest(msg, session)

	// Validate required fields
	if err := h.validateRequiredFields(msg, map[parser.FieldCode]string{
		parser.PatronIdentifier: "Patron Identifier",
		parser.ItemIdentifier:   "Item Identifier",
	}); err != nil {
		h.logger.Error("Checkout validation failed", zap.Error(err))
		return h.buildCheckoutResponse(false, "", "", "", "", time.Time{}, msg, session, "Checkout validation failed"), nil
	}

	// Extract fields
	institutionID := msg.GetField(parser.InstitutionID)
	patronIdentifier := msg.GetField(parser.PatronIdentifier)
	itemIdentifier := msg.GetField(parser.ItemIdentifier)
	patronPassword := msg.GetField(parser.PatronPassword)

	// Get service point UUID from session (CP field set at LOGIN) — not from AO
	servicePointID := session.GetLocationCode()
	if servicePointID == "" {
		h.logger.Error("Checkout failed: service point ID (CP field) is required but not set in session",
			zap.String("patron_identifier", patronIdentifier),
			zap.String("item_identifier", itemIdentifier),
		)
		return h.buildCheckoutResponse(false, institutionID, patronIdentifier, itemIdentifier, itemIdentifier, time.Time{}, msg, session, "Checkout failed: service point not configured"), nil
	}

	h.logger.Info("Checkout request",
		zap.String("institution_id", institutionID),
		zap.String("patron_identifier", patronIdentifier),
		zap.String("item_identifier", itemIdentifier),
	)

	// Get authenticated FOLIO client
	_, token, err := h.getAuthenticatedFolioClient(ctx, session)
	if err != nil {
		h.logger.Error("Failed to get authenticated client", zap.Error(err))
		return h.buildCheckoutResponse(false, institutionID, patronIdentifier, itemIdentifier, itemIdentifier, time.Time{}, msg, session, "Authentication failed"), nil
	}

	// Create patron client to get patron ID
	patronClient := h.getPatronClient(session)

	// Get patron ID and user info (from session or lookup)
	patronID := session.GetPatronID()
	var userID string
	if patronID == "" {
		// Look up patron by barcode
		user, err := patronClient.GetUserByBarcode(ctx, token, patronIdentifier)
		if err != nil {
			h.logger.Error("Failed to get patron",
				zap.String("patron_identifier", patronIdentifier),
				zap.Error(err),
			)
			return h.buildCheckoutResponse(false, institutionID, patronIdentifier, itemIdentifier, itemIdentifier, time.Time{}, msg, session, GetVerificationErrorMessage()), nil
		}
		patronID = user.ID
		userID = user.ID
	} else {
		userID = patronID
	}

	// Verify patron password/PIN if required
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
		h.logger.Info("Checkout blocked due to failed patron verification",
			zap.String("patron_identifier", patronIdentifier),
		)
		return h.buildCheckoutResponse(false, institutionID, patronIdentifier, itemIdentifier, itemIdentifier, time.Time{}, msg, session, GetVerificationErrorMessage()), nil
	}

	// Create circulation client
	circClient := h.getCirculationClient(session)

	// Build checkout request
	checkoutReq := folio.CheckoutRequest{
		ItemBarcode:    itemIdentifier,
		UserBarcode:    patronIdentifier,
		ServicePointID: servicePointID,
		LoanDate:       time.Now().Format(time.RFC3339),
	}

	// Perform checkout
	loan, err := circClient.Checkout(ctx, token, checkoutReq)
	if err != nil {
		h.logger.Error("Checkout failed",
			zap.String("patron_id", patronID),
			zap.String("item_identifier", itemIdentifier),
			zap.Error(err),
		)
		return h.buildCheckoutResponse(false, institutionID, patronIdentifier, itemIdentifier, itemIdentifier, time.Time{}, msg, session, "Checkout failed"), nil
	}

	h.logger.Info("Checkout successful",
		zap.String("patron_id", patronID),
		zap.String("item_identifier", itemIdentifier),
		zap.String("loan_id", loan.ID),
	)

	h.logResponse(string(parser.CheckoutResponse), session, nil)

	// Parse due date
	var dueDate time.Time
	if loan.DueDate != nil {
		dueDate = *loan.DueDate
	}

	// Fetch item details for response fields (title, media type, etc.)
	titleFetchStart := time.Now()
	h.logger.Debug("Fetching item details for title",
		zap.String("item_barcode", itemIdentifier),
	)
	invClient := h.getInventoryClient(session)

	itemFetchStart := time.Now()
	item, err := invClient.GetItemByBarcode(ctx, token, itemIdentifier)
	itemFetchDuration := time.Since(itemFetchStart)

	if err != nil {
		h.logger.Warn("Failed to fetch item details",
			zap.String("item_barcode", itemIdentifier),
			zap.Duration("duration_ms", itemFetchDuration),
			zap.Error(err),
		)
		// Continue with fallback values (item barcode as title)
		return h.buildCheckoutResponse(true, institutionID, patronIdentifier, itemIdentifier, itemIdentifier, dueDate, msg, session, "Checkout successful"), nil
	}

	h.logger.Debug("Successfully fetched item details",
		zap.String("item_barcode", itemIdentifier),
		zap.String("item_id", item.ID),
		zap.String("holdings_record_id", item.HoldingsRecordID),
		zap.Duration("duration_ms", itemFetchDuration),
	)

	var wg sync.WaitGroup
	var title string
	var holdingsFetchDuration time.Duration
	var instanceFetchDuration time.Duration

	// Fetch instance title (holdings → instance) in goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		if item.HoldingsRecordID == "" {
			h.logger.Debug("Item has no holdings record, cannot fetch title",
				zap.String("item_id", item.ID),
				zap.String("item_barcode", itemIdentifier),
			)
			return
		}

		h.logger.Debug("Fetching holdings record",
			zap.String("holdings_id", item.HoldingsRecordID),
		)
		holdingsStart := time.Now()
		holdings, err := invClient.GetHoldingsByID(ctx, token, item.HoldingsRecordID)
		holdingsFetchDuration = time.Since(holdingsStart)

		if err != nil {
			h.logger.Warn("Failed to fetch holdings",
				zap.String("holdings_id", item.HoldingsRecordID),
				zap.String("item_barcode", itemIdentifier),
				zap.Duration("duration_ms", holdingsFetchDuration),
				zap.Error(err),
			)
			return
		}

		h.logger.Debug("Successfully fetched holdings",
			zap.String("holdings_id", item.HoldingsRecordID),
			zap.String("instance_id", holdings.InstanceID),
			zap.Duration("duration_ms", holdingsFetchDuration),
		)

		if holdings.InstanceID == "" {
			h.logger.Debug("Holdings record has no instance ID, cannot fetch title",
				zap.String("holdings_id", item.HoldingsRecordID),
				zap.String("item_barcode", itemIdentifier),
			)
			return
		}

		h.logger.Debug("Fetching instance",
			zap.String("instance_id", holdings.InstanceID),
		)
		instanceStart := time.Now()
		instance, err := invClient.GetInstanceByID(ctx, token, holdings.InstanceID)
		instanceFetchDuration = time.Since(instanceStart)

		if err != nil {
			h.logger.Warn("Failed to fetch instance",
				zap.String("instance_id", holdings.InstanceID),
				zap.String("item_barcode", itemIdentifier),
				zap.Duration("duration_ms", instanceFetchDuration),
				zap.Error(err),
			)
			return
		}

		h.logger.Debug("Successfully fetched instance",
			zap.String("instance_id", holdings.InstanceID),
			zap.String("original_title", instance.Title),
			zap.Int("original_title_length", len(instance.Title)),
			zap.Duration("duration_ms", instanceFetchDuration),
		)

		// Truncate title to 60 characters per SIP2 spec
		title = truncateString(instance.Title, 60)
		wasTruncated := len(instance.Title) > 60
		h.logger.Debug("Title prepared for response",
			zap.String("instance_id", holdings.InstanceID),
			zap.String("title", title),
			zap.Bool("truncated", wasTruncated),
		)
	}()

	wg.Wait()

	// Log total title fetching performance
	totalTitleFetchDuration := time.Since(titleFetchStart)
	h.logger.Info("Title fetching completed",
		zap.String("item_barcode", itemIdentifier),
		zap.Duration("total_duration_ms", totalTitleFetchDuration),
		zap.Duration("item_fetch_ms", itemFetchDuration),
		zap.Duration("holdings_fetch_ms", holdingsFetchDuration),
		zap.Duration("instance_fetch_ms", instanceFetchDuration),
		zap.Bool("title_retrieved", title != ""),
	)

	// Use fetched title or fallback to item barcode
	if title == "" {
		title = itemIdentifier
		h.logger.Info("Using item barcode as title fallback",
			zap.String("item_barcode", itemIdentifier),
			zap.String("item_id", item.ID),
			zap.String("reason", "Title fetch failed or returned empty"),
		)
	} else {
		h.logger.Info("Successfully retrieved title for checkout response",
			zap.String("item_barcode", itemIdentifier),
			zap.String("title", title),
		)
	}

	return h.buildCheckoutResponse(true, institutionID, patronIdentifier, itemIdentifier, title, dueDate, msg, session, "Checkout successful"), nil
}

// buildCheckoutResponse builds a Checkout Response (12)
func (h *CheckoutHandler) buildCheckoutResponse(ok bool, institutionID, patronIdentifier, itemIdentifier, titleID string, dueDate time.Time, msg *parser.Message, session *types.Session, screenMessage string) string {
	// Use session's tenant config for proper delimiter and error detection settings
	tenantConfig := h.tenantConfig
	if session != nil && session.TenantConfig != nil {
		tenantConfig = session.TenantConfig
	}

	h.logger.Debug("Building checkout response",
		zap.String("sequence_number", msg.SequenceNumber),
		zap.Bool("error_detection_enabled", tenantConfig.ErrorDetectionEnabled),
	)

	// Create response builder with session's config
	responseBuilder := builder.NewResponseBuilder(tenantConfig)

	// Use ResponseBuilder.BuildCheckoutResponse for consistent, optimized response construction
	response, err := responseBuilder.BuildCheckoutResponse(
		ok,                      // ok
		false,                   // renewalOK (not a renewal)
		false,                   // magneticMedia (unknown = false, builder will format as 'U')
		false,                   // desensitize (unknown = false, builder will format as 'U')
		time.Now(),              // transactionDate
		institutionID,           // institutionID
		patronIdentifier,        // patronID
		itemIdentifier,          // itemID
		titleID,                 // titleID (actual instance title, truncated to 60 chars, or item barcode fallback)
		dueDate,                 // dueDate
		"",                      // feeType (empty)
		false,                   // securityInhibit
		"",                      // currencyType (empty)
		"",                      // feeAmount (empty)
		"",                      // mediaType (empty)
		"",                      // itemProperties (empty)
		"",                      // transactionID (empty)
		[]string{screenMessage}, // screenMessage
		[]string{},              // printLine (empty)
		msg.SequenceNumber,      // sequenceNumber
	)

	if err != nil {
		h.logger.Error("Failed to build checkout response", zap.Error(err))
		// Fallback to basic error response
		return "12NU  " + protocol.FormatSIP2DateTime(time.Now(), tenantConfig.Timezone) +
			tenantConfig.FieldDelimiter + "AO" + institutionID +
			tenantConfig.FieldDelimiter + "AA" + patronIdentifier +
			tenantConfig.FieldDelimiter + "AB" + itemIdentifier +
			tenantConfig.FieldDelimiter + "AF" + screenMessage +
			tenantConfig.MessageDelimiter
	}

	h.logger.Debug("Successfully built checkout response")
	return response
}
