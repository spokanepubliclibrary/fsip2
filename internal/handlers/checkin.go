package handlers

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/spokanepubliclibrary/fsip2/internal/config"
	"github.com/spokanepubliclibrary/fsip2/internal/folio"
	"github.com/spokanepubliclibrary/fsip2/internal/folio/models"
	"github.com/spokanepubliclibrary/fsip2/internal/logging"
	"github.com/spokanepubliclibrary/fsip2/internal/sip2/builder"
	"github.com/spokanepubliclibrary/fsip2/internal/sip2/mediatype"
	"github.com/spokanepubliclibrary/fsip2/internal/sip2/parser"
	"github.com/spokanepubliclibrary/fsip2/internal/sip2/protocol"
	"github.com/spokanepubliclibrary/fsip2/internal/types"
	"go.uber.org/zap"
)

// CheckinHandler handles SIP2 Checkin requests (09)
type CheckinHandler struct {
	*BaseHandler
	logger *zap.Logger
}

// checkinResponseData holds all data needed to build a complete checkin response
type checkinResponseData struct {
	ok                  bool
	institutionID       string
	itemBarcode         string
	currentLocation     string
	permanentLocation   string
	title               string
	materialType        string
	mediaTypeCode       string
	callNumber          string
	alertType           string
	destinationLocation string
	checkinNotes        []string
	holdShelfExpiration string
	requestorName       string
	sequenceNumber      string
	item                *models.Item
}

// NewCheckinHandler creates a new checkin handler
func NewCheckinHandler(logger *zap.Logger, tenantConfig *config.TenantConfig) *CheckinHandler {
	return &CheckinHandler{
		BaseHandler: NewBaseHandler(logger, tenantConfig),
		logger:      logger.With(logging.TypeField(logging.TypeApplication)),
	}
}

// Handle processes a Checkin request (09) and returns a Checkin response (10)
func (h *CheckinHandler) Handle(ctx context.Context, msg *parser.Message, session *types.Session) (string, error) {
	h.logRequest(msg, session)

	// Validate required fields
	if err := h.validateRequiredFields(msg, map[parser.FieldCode]string{
		parser.ItemIdentifier: "Item Identifier",
	}); err != nil {
		h.logger.Error("Checkin validation failed", zap.Error(err))
		return h.buildCheckinResponse(false, "", "", "", msg, session), nil
	}

	// Extract fields
	currentLocation := msg.GetField(parser.CurrentLocation) // AP: echoed in 10 response only
	institutionID := msg.GetField(parser.InstitutionID)
	itemIdentifier := msg.GetField(parser.ItemIdentifier)
	servicePointID := session.GetLocationCode() // CP: FOLIO service point UUID

	// Validate that CP is set in session — fail fast before any FOLIO calls
	if servicePointID == "" {
		h.logger.Error("Checkin failed: service point ID (CP field) is required but not set in session",
			zap.String("item_identifier", itemIdentifier),
		)
		return h.buildCheckinResponse(false, institutionID, itemIdentifier, currentLocation, msg, session), nil
	}

	h.logger.Info("Checkin request",
		zap.String("institution_id", institutionID),
		zap.String("item_identifier", itemIdentifier),
		zap.String("current_location_ap", currentLocation),
		zap.String("service_point_cp", servicePointID),
	)

	// Get authenticated FOLIO client
	_, token, err := h.getAuthenticatedFolioClient(ctx, session)
	if err != nil {
		h.logger.Error("Failed to get authenticated client", zap.Error(err))
		return h.buildCheckinResponse(false, institutionID, itemIdentifier, currentLocation, msg, session), nil
	}

	// Check if item is claimed returned and handle based on config
	invClient := h.getInventoryClient(session)
	item, err := invClient.GetItemByBarcode(ctx, token, itemIdentifier)
	if err == nil && item != nil && item.Status.Name == "Claimed returned" {
		resolution := session.TenantConfig.GetClaimedReturnedResolution()
		if resolution == "none" {
			h.logger.Warn("Checkin blocked: item is claimed returned",
				zap.String("item_identifier", itemIdentifier),
				zap.String("resolution_policy", resolution),
			)
			// Return error response with specific message
			return h.buildCheckinResponseWithMessage(false, institutionID, itemIdentifier,
				currentLocation, "Checkin failed - Item is claimed returned", msg, session), nil
		}
		h.logger.Info("Processing claimed returned item with resolution",
			zap.String("item_identifier", itemIdentifier),
			zap.String("resolution", resolution),
		)
	}

	// Create circulation client
	circClient := h.getCirculationClient(session)

	// Build checkin request for FOLIO
	checkinReq := folio.CheckinRequest{
		ItemBarcode:               itemIdentifier,
		ServicePointID:            servicePointID,
		CheckInDate:               time.Now().Format(time.RFC3339),
		ClaimedReturnedResolution: session.TenantConfig.MapClaimedReturnedResolutionToFOLIO(),
	}

	// Perform checkin
	loan, err := circClient.Checkin(ctx, token, checkinReq)
	if err != nil {
		h.logger.Error("Checkin failed",
			zap.String("item_identifier", itemIdentifier),
			zap.Error(err),
		)
		return h.buildCheckinResponse(false, institutionID, itemIdentifier, currentLocation, msg, session), nil
	}

	h.logger.Info("Checkin successful",
		zap.String("item_identifier", itemIdentifier),
		zap.String("loan_id", loan.ID),
	)

	// Fetch enhanced item data for response (Steps 2 & 3: permanent location and title)
	responseData := h.fetchCheckinResponseData(ctx, token, itemIdentifier, session)
	responseData.ok = true
	responseData.institutionID = institutionID
	responseData.currentLocation = currentLocation
	responseData.sequenceNumber = msg.SequenceNumber

	h.logResponse(string(parser.CheckinResponse), session, nil)

	return h.buildCheckinResponseWithData(responseData, session), nil
}

// fetchCheckinResponseData fetches all necessary data for building a complete checkin response
// Uses parallel goroutines for independent API calls to improve performance
func (h *CheckinHandler) fetchCheckinResponseData(ctx context.Context, token string, itemBarcode string, session *types.Session) *checkinResponseData {
	data := &checkinResponseData{
		itemBarcode: itemBarcode,
	}

	// Create inventory and circulation clients
	invClient := h.getInventoryClient(session)

	// STEP 1: Fetch item by barcode (MUST be first - all other calls depend on this)
	item, err := invClient.GetItemByBarcode(ctx, token, itemBarcode)
	if err != nil {
		h.logger.Warn("Failed to fetch item by barcode",
			zap.String("barcode", itemBarcode),
			zap.Error(err),
		)
		return data
	}
	data.item = item

	// Extract checkin notes if available
	if item != nil {
		data.checkinNotes = item.GetCheckinNotes()
		if len(data.checkinNotes) > 0 {
			h.logger.Debug("Found checkin notes",
				zap.Int("count", len(data.checkinNotes)),
				zap.Strings("notes", data.checkinNotes),
			)
		}
	}

	h.logger.Debug("Item fetched",
		zap.String("item_id", item.ID),
		zap.String("effective_location_id", item.EffectiveLocationID),
		zap.String("permanent_location_id", item.PermanentLocationID),
		zap.Bool("has_location_obj", item.Location != nil),
	)

	// Extract effective call number (CS field - no API call needed)
	if item.EffectiveCallNumberComponents.CallNumber != "" {
		data.callNumber = item.EffectiveCallNumberComponents.CallNumber
		h.logger.Debug("Using effective call number from item record",
			zap.String("call_number", item.EffectiveCallNumberComponents.CallNumber),
		)
	}

	// STEP 2: Parallelize independent API calls using goroutines
	var wg sync.WaitGroup

	// Goroutine 1: Fetch location (if needed)
	wg.Add(1)
	go func() {
		defer wg.Done()
		if item.Location != nil && item.Location.Name != "" {
			data.permanentLocation = item.Location.Name
			h.logger.Debug("Using populated location from item record",
				zap.String("location_name", item.Location.Name),
			)
		} else {
			locationID := item.EffectiveLocationID
			if locationID == "" {
				locationID = item.PermanentLocationID
			}

			if locationID != "" {
				h.logger.Debug("Location not populated in item, fetching by ID",
					zap.String("location_id", locationID),
				)
				location, err := invClient.GetLocationByID(ctx, token, locationID)
				if err != nil {
					h.logger.Warn("Failed to fetch location",
						zap.String("location_id", locationID),
						zap.Error(err),
					)
				} else {
					data.permanentLocation = location.Name
					h.logger.Debug("Fetched location by ID",
						zap.String("location_name", location.Name),
						zap.String("location_code", location.Code),
					)
				}
			} else {
				h.logger.Warn("No location information available for item",
					zap.String("barcode", itemBarcode),
				)
			}
		}
	}()

	// Goroutine 2: Fetch material type (if needed)
	wg.Add(1)
	go func() {
		defer wg.Done()
		if item.MaterialType != nil && item.MaterialType.Name != "" {
			data.materialType = item.MaterialType.Name
			h.logger.Debug("Using populated material type from item record",
				zap.String("material_type_name", item.MaterialType.Name),
			)
		} else if item.MaterialTypeID != "" {
			h.logger.Debug("Material type not populated in item, fetching by ID",
				zap.String("material_type_id", item.MaterialTypeID),
			)
			materialType, err := invClient.GetMaterialTypeByID(ctx, token, item.MaterialTypeID)
			if err != nil {
				h.logger.Warn("Failed to fetch material type",
					zap.String("material_type_id", item.MaterialTypeID),
					zap.Error(err),
				)
			} else {
				data.materialType = materialType.Name
				h.logger.Debug("Fetched material type by ID",
					zap.String("material_type_name", materialType.Name),
				)
			}
		}
	}()

	// Goroutine 3: Fetch instance title (holdings → instance)
	wg.Add(1)
	go func() {
		defer wg.Done()
		if item.HoldingsRecordID != "" {
			holdings, err := invClient.GetHoldingsByID(ctx, token, item.HoldingsRecordID)
			if err != nil {
				h.logger.Warn("Failed to fetch holdings",
					zap.String("holdings_id", item.HoldingsRecordID),
					zap.Error(err),
				)
			} else if holdings.InstanceID != "" {
				instance, err := invClient.GetInstanceByID(ctx, token, holdings.InstanceID)
				if err != nil {
					h.logger.Warn("Failed to fetch instance",
						zap.String("instance_id", holdings.InstanceID),
						zap.Error(err),
					)
				} else {
					// Truncate title to 60 characters per SIP2 spec
					data.title = truncateString(instance.Title, 60)
				}
			}
		}
	}()

	// Goroutine 4: Fetch requests (for alert type, routing, hold info)
	// This is stored in a local var to avoid race conditions
	var requests *models.RequestCollection
	var requestsErr error
	var topRequest *models.Request
	var hasHoldOrRecall bool

	wg.Add(1)
	go func() {
		defer wg.Done()
		circClient := h.getCirculationClient(session)
		requests, requestsErr = circClient.GetRequestsByItem(ctx, token, item.ID)
		if requestsErr != nil {
			h.logger.Warn("Failed to fetch requests for item",
				zap.String("item_id", item.ID),
				zap.Error(requestsErr),
			)
		} else if requests != nil {
			// Check if there are any open holds or recalls
			for _, req := range requests.Requests {
				if req.IsOpen() && req.IsHold() {
					if !hasHoldOrRecall {
						reqCopy := req
						topRequest = &reqCopy
						hasHoldOrRecall = true
						h.logger.Debug("Found open hold/recall for item",
							zap.String("request_id", req.ID),
							zap.String("request_type", req.RequestType),
							zap.String("status", req.Status),
							zap.String("pickup_service_point_id", req.PickupServicePointID),
						)
					}
				}
			}
		}
	}()

	// Wait for all parallel calls to complete
	wg.Wait()

	// Map material type to SIP2 media type code (CK field - Step 4)
	if data.materialType != "" {
		data.mediaTypeCode = mediatype.MapToSIP2MediaType(data.materialType)
		h.logger.Debug("Mapped material type to SIP2 media type",
			zap.String("material_type", data.materialType),
			zap.String("media_type_code", data.mediaTypeCode),
		)
	}

	// Calculate alert type (CV field - Step 7)
	inTransit := item.Status.Name == "In transit"
	data.alertType = calculateAlertType(inTransit, hasHoldOrRecall)
	if data.alertType != "" {
		h.logger.Debug("Alert type calculated",
			zap.String("alert_type", data.alertType),
			zap.Bool("in_transit", inTransit),
			zap.Bool("has_hold_or_recall", hasHoldOrRecall),
		)
	}

	// STEP 3: Determine routing (destination) location (CT field - Step 8)
	// This depends on requests, so it must run after wg.Wait()
	if topRequest != nil && topRequest.PickupServicePointID != "" {
		servicePoint, err := invClient.GetServicePointByID(ctx, token, topRequest.PickupServicePointID)
		if err != nil {
			h.logger.Warn("Failed to fetch pickup service point",
				zap.String("service_point_id", topRequest.PickupServicePointID),
				zap.Error(err),
			)
		} else {
			data.destinationLocation = servicePoint.Name
			h.logger.Debug("Routing to pickup service point for hold",
				zap.String("service_point_name", servicePoint.Name),
				zap.String("service_point_code", servicePoint.Code),
			)
		}
	} else if inTransit && item.InTransitDestinationServicePointID != "" {
		servicePoint, err := invClient.GetServicePointByID(ctx, token, item.InTransitDestinationServicePointID)
		if err != nil {
			h.logger.Warn("Failed to fetch in-transit destination service point",
				zap.String("service_point_id", item.InTransitDestinationServicePointID),
				zap.Error(err),
			)
		} else {
			data.destinationLocation = servicePoint.Name
			h.logger.Debug("Routing to in-transit destination service point",
				zap.String("service_point_name", servicePoint.Name),
				zap.String("service_point_code", servicePoint.Code),
			)
		}
	}

	// Extract hold information from first awaiting pickup request (CM and DA fields)
	if requests != nil {
		for _, req := range requests.Requests {
			if req.IsAwaitingPickup() {
				// CM - Hold Shelf Expiration Date
				if req.HoldShelfExpirationDate != nil {
					data.holdShelfExpiration = protocol.FormatSIP2DateTime(*req.HoldShelfExpirationDate, h.tenantConfig.Timezone)
					h.logger.Debug("Found hold shelf expiration date",
						zap.Time("expiration_date", *req.HoldShelfExpirationDate),
					)
				}

				// DA - Requestor Name (lastName, firstName format)
				if req.Requester != nil {
					if req.Requester.LastName != "" {
						data.requestorName = req.Requester.LastName
						if req.Requester.FirstName != "" {
							data.requestorName += ", " + req.Requester.FirstName
						}
						h.logger.Debug("Found requestor name",
							zap.String("requestor_name", data.requestorName),
						)
					}
				}

				// Only need the first awaiting pickup request
				break
			}
		}
	}

	return data
}

// truncateString truncates a string to maxLen characters
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	// Truncate and handle multi-byte UTF-8 characters properly
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen])
}

// calculateAlertType determines the alert type based on item status and requests
// Alert Type Codes:
// 01 = Hold exists but item not in transit
// 02 = Item in transit AND (hold or recall exists)
// 04 = Item in transit ONLY (no holds/recalls)
// (empty) = No alert condition
func calculateAlertType(inTransit bool, hasHoldOrRecall bool) string {
	if inTransit || hasHoldOrRecall {
		if !inTransit {
			return "01"
		} else if hasHoldOrRecall {
			return "02"
		} else {
			return "04"
		}
	}
	return ""
}

// buildCheckinResponse builds a Checkin Response (10)
func (h *CheckinHandler) buildCheckinResponse(ok bool, institutionID, itemIdentifier, currentLocation string, msg *parser.Message, session *types.Session) string {
	// Use session's tenant config for proper delimiter configuration
	var tenantConfig *config.TenantConfig
	if session != nil && session.TenantConfig != nil {
		tenantConfig = session.TenantConfig
	} else {
		tenantConfig = h.tenantConfig
	}

	timestamp := protocol.FormatSIP2DateTime(time.Now(), tenantConfig.Timezone)

	// Checkin Response format:
	// 10<ok><resensitize><magnetic_media><alert><transaction_date>
	// <institution_id><item_identifier><permanent_location><title_identifier>
	// <sort_bin><patron_identifier><media_type><item_properties>
	// <screen_message><print_line>

	okValue := "0"
	if ok {
		okValue = "1"
	}

	resensitize := "Y"   // Resensitize the item
	magneticMedia := "U" // Unknown
	alert := "N"         // No alert

	content := fmt.Sprintf("%s%s%s%s%s",
		okValue,
		resensitize,
		magneticMedia,
		alert,
		timestamp,
	)

	// Add variable fields
	delimiter := tenantConfig.FieldDelimiter
	content += protocol.BuildField("AO", institutionID, delimiter)
	content += protocol.BuildField("AB", itemIdentifier, delimiter)

	if currentLocation != "" {
		content += protocol.BuildField("AP", currentLocation, delimiter)
	}

	// Add title if available
	// Note: Error path uses item identifier for faster response; title is fetched in success path
	content += protocol.BuildField("AJ", itemIdentifier, delimiter)

	// Add screen message
	if ok {
		content += protocol.BuildField("AF", "Checkin successful", delimiter)
	} else {
		content += protocol.BuildField("AF", "Checkin failed", delimiter)
	}

	// Use ResponseBuilder with session's tenant config to add AY (sequence number) and AZ (checksum) if enabled
	sessionBuilder := builder.NewResponseBuilder(tenantConfig)
	response, err := sessionBuilder.Build(parser.CheckinResponse, content, msg.SequenceNumber)
	if err != nil {
		h.logger.Error("Failed to build checkin response", zap.Error(err))
		// Fallback to response without error detection
		return "10" + content + tenantConfig.MessageDelimiter
	}

	return response
}

// buildCheckinResponseWithMessage builds a Checkin Response (10) with a custom screen message
func (h *CheckinHandler) buildCheckinResponseWithMessage(ok bool, institutionID, itemIdentifier, currentLocation, screenMessage string, msg *parser.Message, session *types.Session) string {
	// Use session's tenant config for proper delimiter configuration
	var tenantConfig *config.TenantConfig
	if session != nil && session.TenantConfig != nil {
		tenantConfig = session.TenantConfig
	} else {
		tenantConfig = h.tenantConfig
	}

	timestamp := protocol.FormatSIP2DateTime(time.Now(), tenantConfig.Timezone)

	okValue := "0"
	if ok {
		okValue = "1"
	}

	resensitize := "Y"   // Resensitize the item
	magneticMedia := "U" // Unknown
	alert := "N"         // No alert

	content := fmt.Sprintf("%s%s%s%s%s",
		okValue,
		resensitize,
		magneticMedia,
		alert,
		timestamp,
	)

	// Add variable fields
	delimiter := tenantConfig.FieldDelimiter
	content += protocol.BuildField("AO", institutionID, delimiter)
	content += protocol.BuildField("AB", itemIdentifier, delimiter)

	if currentLocation != "" {
		content += protocol.BuildField("AP", currentLocation, delimiter)
	}

	// Add title if available
	content += protocol.BuildField("AJ", itemIdentifier, delimiter)

	// Add custom screen message
	content += protocol.BuildField("AF", screenMessage, delimiter)

	// Use ResponseBuilder with session's tenant config to add AY (sequence number) and AZ (checksum) if enabled
	sessionBuilder := builder.NewResponseBuilder(tenantConfig)
	response, err := sessionBuilder.Build(parser.CheckinResponse, content, msg.SequenceNumber)
	if err != nil {
		h.logger.Error("Failed to build checkin response", zap.Error(err))
		// Fallback to response without error detection
		return "10" + content + tenantConfig.MessageDelimiter
	}

	return response
}

// buildCheckinResponseWithData builds a Checkin Response (10) with enhanced data using ResponseBuilder
func (h *CheckinHandler) buildCheckinResponseWithData(data *checkinResponseData, session *types.Session) string {
	// Create builder with session's tenant config for proper error detection settings
	sessionBuilder := h.builder
	if session != nil && session.TenantConfig != nil {
		sessionBuilder = builder.NewResponseBuilder(session.TenantConfig)
	}

	// Prepare screen messages
	screenMessages := []string{}
	if data.ok {
		screenMessages = append(screenMessages, "Checkin successful")
	} else {
		screenMessages = append(screenMessages, "Checkin failed")
	}

	// Set alert flag based on alert type
	alert := data.alertType != ""

	// Use the builder to construct the response
	response, err := sessionBuilder.BuildCheckinResponse(
		data.ok,                  // ok
		true,                     // resensitize
		false,                    // magneticMedia (U/unknown represented as false)
		alert,                    // alert
		time.Now(),               // transactionDate
		data.institutionID,       // institutionID
		data.itemBarcode,         // itemID
		data.permanentLocation,   // permanentLocation
		data.currentLocation,     // currentLocation
		data.title,               // titleID (will use itemBarcode as fallback in builder)
		data.materialType,        // materialType (CH field)
		data.mediaTypeCode,       // mediaType (CK field)
		data.callNumber,          // callNumber (CS field)
		data.alertType,           // alertType (CV field)
		data.destinationLocation, // destinationLocation (CT field)
		"",                       // sortBin (not used)
		"",                       // patronID (not used in checkin)
		"",                       // itemProperties (not used)
		data.checkinNotes,        // checkinNotes (AG field, repeatable)
		data.holdShelfExpiration, // holdShelfExpiration (CM field)
		data.requestorName,       // requestorName (DA field)
		screenMessages,           // screenMessage
		[]string{},               // printLine
		data.sequenceNumber,      // sequenceNumber
	)

	if err != nil {
		h.logger.Error("Failed to build checkin response", zap.Error(err))
		// Fallback to simple error response
		return h.buildCheckinResponse(false, data.institutionID, data.itemBarcode, data.currentLocation, &parser.Message{SequenceNumber: data.sequenceNumber}, session)
	}

	return response
}
