package handlers

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/spokanepubliclibrary/fsip2/internal/config"
	"github.com/spokanepubliclibrary/fsip2/internal/folio/models"
	"github.com/spokanepubliclibrary/fsip2/internal/sip2/builder"
	"github.com/spokanepubliclibrary/fsip2/internal/sip2/mediatype"
	"github.com/spokanepubliclibrary/fsip2/internal/sip2/parser"
	"github.com/spokanepubliclibrary/fsip2/internal/sip2/protocol"
	"github.com/spokanepubliclibrary/fsip2/internal/types"
	"go.uber.org/zap"
)

var (
	// uuidPattern matches UUID format: 8-4-4-4-12 hexadecimal characters
	uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
)

// itemInformationResponseData holds all data for building an item information response
type itemInformationResponseData struct {
	circulationStatus   string
	securityMarker      string
	feeType             string
	institutionID       string
	itemID              string
	title               string
	permanentLocation   string
	currentLocation     string
	dueDate             string
	mediaType           string
	materialType        string
	callNumber          string
	routingLocation     string
	holdQueueLength     string
	primaryContributor  string
	workDescription     string
	isbns               []string
	upcs                []string
	holdShelfExpiration string
	requestorBarcode    string
	requestorName       string
	screenMessage       []string
	printLine           []string
}

// ItemInformationHandler handles SIP2 Item Information requests (17)
type ItemInformationHandler struct {
	*BaseHandler
	logger *zap.Logger
	// NOTE: responseBuilder is removed - we create it per-request using session config
}

// NewItemInformationHandler creates a new item information handler
func NewItemInformationHandler(logger *zap.Logger, tenantConfig *config.TenantConfig) *ItemInformationHandler {
	return &ItemInformationHandler{
		BaseHandler: NewBaseHandler(logger, tenantConfig),
		logger:      logger,
	}
}

// Handle processes an Item Information request (17) and returns an Item Information response (18)
func (h *ItemInformationHandler) Handle(ctx context.Context, msg *parser.Message, session *types.Session) (string, error) {
	h.logRequest(msg, session)

	// Validate required fields
	if err := h.validateRequiredFields(msg, map[parser.FieldCode]string{
		parser.InstitutionID:  "Institution ID",
		parser.ItemIdentifier: "Item Identifier",
	}); err != nil {
		h.logger.Error("Item information validation failed", zap.Error(err))
		return h.buildErrorResponse(msg), fmt.Errorf("validation failed: %w", err)
	}

	// Extract fields
	institutionID := msg.GetField(parser.InstitutionID)
	itemIdentifier := msg.GetField(parser.ItemIdentifier)

	h.logger.Info("Item information request",
		zap.String("institution_id", institutionID),
		zap.String("item_identifier", itemIdentifier),
	)

	// Get authenticated FOLIO client
	_, token, err := h.getAuthenticatedFolioClient(ctx, session)
	if err != nil {
		h.logger.Error("Failed to get authenticated client", zap.Error(err))
		return h.buildItemInformationResponse(session, nil, institutionID, itemIdentifier, nil, nil, "", nil, "", "", false), nil
	}

	// Create inventory client
	inventoryClient := h.getInventoryClient(session)

	// Detect whether AB field contains a barcode or instance UUID
	if isUUID(itemIdentifier) {
		// Instance UUID path - bibliographic lookup only
		h.logger.Info("Detected instance UUID format, performing instance-level lookup",
			zap.String("instance_uuid", itemIdentifier),
		)

		instance, err := inventoryClient.GetInstanceByID(ctx, token, itemIdentifier)
		if err != nil {
			h.logger.Error("Failed to get instance information",
				zap.String("instance_uuid", itemIdentifier),
				zap.Error(err),
			)
			return h.buildInstanceInformationResponse(session, nil, institutionID, itemIdentifier, false), nil
		}

		h.logger.Info("Instance information retrieved",
			zap.String("instance_id", instance.ID),
			zap.String("title", instance.Title),
		)

		h.logResponse(string(parser.ItemInformationResponse), session, nil)
		return h.buildInstanceInformationResponse(session, instance, institutionID, itemIdentifier, true), nil
	}

	// Barcode path - standard item lookup
	h.logger.Info("Detected barcode format, performing item-level lookup",
		zap.String("barcode", itemIdentifier),
	)

	item, err := inventoryClient.GetItemByBarcode(ctx, token, itemIdentifier)
	if err != nil {
		h.logger.Error("Failed to get item information",
			zap.String("item_identifier", itemIdentifier),
			zap.Error(err),
		)
		return h.buildItemInformationResponse(session, nil, institutionID, itemIdentifier, nil, nil, "", nil, "", "", false), nil
	}

	h.logger.Info("Item information retrieved",
		zap.String("item_id", item.ID),
		zap.String("status", item.Status.Name),
	)

	// Fetch instance title if AJ field is enabled (Step 3)
	// Path: Item → Holdings → Instance → Title
	if h.tenantConfig.IsFieldEnabled("17", "AJ") && item.HoldingsRecordID != "" {
		holdings, err := inventoryClient.GetHoldingsByID(ctx, token, item.HoldingsRecordID)
		if err != nil {
			h.logger.Warn("Failed to get holdings for title",
				zap.String("holdings_id", item.HoldingsRecordID),
				zap.Error(err),
			)
		} else if holdings.InstanceID != "" {
			instance, err := inventoryClient.GetInstanceByID(ctx, token, holdings.InstanceID)
			if err != nil {
				h.logger.Warn("Failed to get instance for title",
					zap.String("instance_id", holdings.InstanceID),
					zap.Error(err),
				)
			} else {
				// Populate instance in item for use in response builder
				item.Instance = instance
			}
		}
	}

	// Fetch due date if AH field is enabled and item is checked out (Step 5)
	var dueDate *time.Time
	if h.tenantConfig.IsFieldEnabled("17", "AH") && item.IsCheckedOut() {
		circClient := h.getCirculationClient(session)
		loans, err := circClient.GetLoansByItem(ctx, token, item.ID)
		if err != nil {
			h.logger.Warn("Failed to get loans for due date",
				zap.String("item_id", item.ID),
				zap.Error(err),
			)
		} else if loans.TotalRecords > 0 && loans.Loans[0].DueDate != nil {
			dueDate = loans.Loans[0].DueDate
		}
	}

	// Fetch requests if CF, CM, CY, or DA fields are enabled (Steps 10, 15, 16, 17)
	var requests *models.RequestCollection
	if h.tenantConfig.IsFieldEnabled("17", "CF") ||
		h.tenantConfig.IsFieldEnabled("17", "CM") ||
		h.tenantConfig.IsFieldEnabled("17", "CY") ||
		h.tenantConfig.IsFieldEnabled("17", "DA") {
		circClient := h.getCirculationClient(session)
		var err error
		requests, err = circClient.GetRequestsByItem(ctx, token, item.ID)
		if err != nil {
			h.logger.Warn("Failed to get requests for hold fields",
				zap.String("item_id", item.ID),
				zap.Error(err),
			)
		}
	}

	// Extract hold information from first awaiting pickup request (Steps 15, 16, 17)
	var holdShelfExpirationDate *time.Time
	var requestorBarcode string
	var requestorName string
	if requests != nil {
		for _, request := range requests.Requests {
			if request.IsAwaitingPickup() {
				// CM - Hold Shelf Expiration Date (Step 15)
				if request.HoldShelfExpirationDate != nil {
					holdShelfExpirationDate = request.HoldShelfExpirationDate
				}

				// CY - Requestor Barcode (Step 16)
				if request.Requester != nil && request.Requester.Barcode != "" {
					requestorBarcode = request.Requester.Barcode
				}

				// DA - Requestor Name (Step 17)
				if request.Requester != nil {
					if request.Requester.LastName != "" {
						requestorName = request.Requester.LastName
						if request.Requester.FirstName != "" {
							requestorName += ", " + request.Requester.FirstName
						}
					}
				}

				// Only need the first awaiting pickup request
				break
			}
		}
	}

	// Determine routing location from in-transit destination (Step 9)
	var routingLocation string
	if h.tenantConfig.IsFieldEnabled("17", "CT") {
		h.logger.Debug("Checking routing location",
			zap.String("item_id", item.ID),
			zap.String("in_transit_destination_sp_id", item.InTransitDestinationServicePointID),
			zap.Bool("has_destination", item.InTransitDestinationServicePointID != ""),
		)

		if item.InTransitDestinationServicePointID != "" {
			servicePoint, err := inventoryClient.GetServicePointByID(ctx, token, item.InTransitDestinationServicePointID)
			if err != nil {
				h.logger.Warn("Failed to get in-transit destination service point",
					zap.String("service_point_id", item.InTransitDestinationServicePointID),
					zap.Error(err),
				)
			} else {
				routingLocation = servicePoint.Name
				h.logger.Info("Routing location determined",
					zap.String("service_point_id", item.InTransitDestinationServicePointID),
					zap.String("service_point_name", routingLocation),
				)
			}
		} else {
			h.logger.Debug("No in-transit destination service point ID found for item",
				zap.String("item_id", item.ID),
				zap.String("item_status", item.Status.Name),
			)
		}
	}

	h.logResponse(string(parser.ItemInformationResponse), session, nil)

	return h.buildItemInformationResponse(session, item, institutionID, itemIdentifier, dueDate, requests, routingLocation, holdShelfExpirationDate, requestorBarcode, requestorName, true), nil
}

// isUUID checks if a string matches UUID format (8-4-4-4-12)
func isUUID(s string) bool {
	return uuidPattern.MatchString(strings.ToLower(s))
}

// buildItemInformationResponse builds an Item Information Response (18)
func (h *ItemInformationHandler) buildItemInformationResponse(
	session *types.Session,
	item *models.Item,
	institutionID string,
	itemIdentifier string,
	dueDate *time.Time,
	requests *models.RequestCollection,
	routingLocation string,
	holdShelfExpirationDate *time.Time,
	requestorBarcode string,
	requestorName string,
	valid bool,
) string {
	// Prepare response data
	data := h.prepareItemResponseData(item, institutionID, itemIdentifier, dueDate, requests, routingLocation, holdShelfExpirationDate, requestorBarcode, requestorName, valid)

	// Create ResponseBuilder using session's tenant config (not handler's default config)
	responseBuilder := builder.NewResponseBuilder(session.TenantConfig)

	// Build response using ResponseBuilder
	response, err := responseBuilder.BuildItemInformationResponse(
		data.circulationStatus,
		data.securityMarker,
		data.feeType,
		time.Now(),
		data.institutionID,
		data.itemID,
		data.title,
		data.permanentLocation,
		data.currentLocation,
		data.dueDate,
		data.mediaType,
		data.materialType,
		data.callNumber,
		data.routingLocation,
		data.holdQueueLength,
		data.primaryContributor,
		data.workDescription,
		data.isbns,
		data.upcs,
		data.holdShelfExpiration,
		data.requestorBarcode,
		data.requestorName,
		data.screenMessage,
		data.printLine,
		"0", // sequence number
	)

	if err != nil {
		h.logger.Error("Failed to build item information response", zap.Error(err))
		return h.buildErrorResponse(nil)
	}

	return response
}

// prepareItemResponseData prepares all data needed for building an item information response
func (h *ItemInformationHandler) prepareItemResponseData(
	item *models.Item,
	institutionID string,
	itemIdentifier string,
	dueDate *time.Time,
	requests *models.RequestCollection,
	routingLocation string,
	holdShelfExpirationDate *time.Time,
	requestorBarcode string,
	requestorName string,
	valid bool,
) itemInformationResponseData {
	data := itemInformationResponseData{
		circulationStatus: "01", // Other (default for invalid/not found)
		securityMarker:    "00", // No security marker
		feeType:           "01", // Other/unknown
		institutionID:     institutionID,
		itemID:            itemIdentifier,
		screenMessage:     make([]string, 0),
		printLine:         make([]string, 0),
	}

	if !valid || item == nil {
		data.screenMessage = append(data.screenMessage, "Item not found")
		return data
	}

	// Map FOLIO item status to SIP2 circulation status
	data.circulationStatus = h.tenantConfig.MapCirculationStatus(item.Status.Name)
	h.logger.Debug("Mapped circulation status",
		zap.String("folio_status", item.Status.Name),
		zap.String("sip2_status", data.circulationStatus),
	)

	// AJ - Instance Title (truncated to 60 chars)
	if item.Instance != nil {
		data.title = truncateString(item.Instance.Title, 60)
	}

	// AQ/AP - Permanent/Current Location
	if item.Location != nil {
		data.permanentLocation = item.Location.Name
		data.currentLocation = item.Location.Name
	}

	// AH - Due Date
	if dueDate != nil {
		data.dueDate = protocol.FormatSIP2DateTime(*dueDate, "    ")
	}

	// CK - Media Type, CH - Material Type
	if item.MaterialType != nil {
		data.mediaType = mediatype.MapToSIP2MediaType(item.MaterialType.Name)
		data.materialType = item.MaterialType.Name
	}

	// CS - Call Number
	data.callNumber = item.GetEffectiveCallNumber()

	// CT - Routing Location
	data.routingLocation = routingLocation

	// CF - Hold Queue Length
	if requests != nil {
		holdQueueLength := 0
		for _, request := range requests.Requests {
			if strings.HasPrefix(request.Status, "Open") {
				holdQueueLength++
			}
		}
		data.holdQueueLength = fmt.Sprintf("%04d", holdQueueLength)
	} else {
		data.holdQueueLength = "0000"
	}

	// Instance-level fields (EA, DE, IN, NB)
	if item.Instance != nil {
		data.primaryContributor = getPrimaryContributor(item.Instance)
		if summary := getSummaryNote(item.Instance); summary != "" {
			data.workDescription = truncateString(summary, 255)
		}
		data.isbns = getISBNs(item.Instance)
		data.upcs = getUPCs(item.Instance)
	}

	// Hold-related fields (CM, CY, DA)
	if holdShelfExpirationDate != nil {
		data.holdShelfExpiration = protocol.FormatSIP2DateTime(*holdShelfExpirationDate, "    ")
	}
	data.requestorBarcode = requestorBarcode
	data.requestorName = requestorName

	data.screenMessage = append(data.screenMessage, "Item found")
	return data
}

// buildInstanceInformationResponse builds an Item Information Response (18) for instance-level lookups
// When an instance UUID is provided, we return bibliographic data only (no item-level fields)
func (h *ItemInformationHandler) buildInstanceInformationResponse(
	session *types.Session,
	instance *models.Instance,
	institutionID string,
	instanceUUID string,
	valid bool,
) string {
	// Prepare response data for instance-level lookup
	data := h.prepareInstanceResponseData(instance, institutionID, instanceUUID, valid)

	// Create ResponseBuilder using session's tenant config (not handler's default config)
	responseBuilder := builder.NewResponseBuilder(session.TenantConfig)

	// Build response using ResponseBuilder
	response, err := responseBuilder.BuildItemInformationResponse(
		data.circulationStatus,
		data.securityMarker,
		data.feeType,
		time.Now(),
		data.institutionID,
		data.itemID,
		data.title,
		data.permanentLocation,
		data.currentLocation,
		data.dueDate,
		data.mediaType,
		data.materialType,
		data.callNumber,
		data.routingLocation,
		data.holdQueueLength,
		data.primaryContributor,
		data.workDescription,
		data.isbns,
		data.upcs,
		data.holdShelfExpiration,
		data.requestorBarcode,
		data.requestorName,
		data.screenMessage,
		data.printLine,
		"0", // sequence number
	)

	if err != nil {
		h.logger.Error("Failed to build instance information response", zap.Error(err))
		return h.buildErrorResponse(nil)
	}

	return response
}

// prepareInstanceResponseData prepares all data needed for building an instance information response
func (h *ItemInformationHandler) prepareInstanceResponseData(
	instance *models.Instance,
	institutionID string,
	instanceUUID string,
	valid bool,
) itemInformationResponseData {
	data := itemInformationResponseData{
		circulationStatus: "01", // Other (always for instance-level)
		securityMarker:    "00", // No security marker
		feeType:           "01", // Other/unknown
		institutionID:     institutionID,
		itemID:            instanceUUID, // Return UUID as-is
		holdQueueLength:   "0000",       // No request lookup for instance-level
		screenMessage:     make([]string, 0),
		printLine:         make([]string, 0),
	}

	if !valid || instance == nil {
		data.screenMessage = append(data.screenMessage, "Instance not found")
		return data
	}

	// AJ - Instance Title (truncated to 60 chars)
	data.title = truncateString(instance.Title, 60)

	// Instance-level fields (EA, DE, IN, NB)
	data.primaryContributor = getPrimaryContributor(instance)
	if summary := getSummaryNote(instance); summary != "" {
		data.workDescription = truncateString(summary, 255)
	}
	data.isbns = getISBNs(instance)
	data.upcs = getUPCs(instance)

	// Item-level fields remain blank (AQ, AP, CK, CH, CS, CT already default to empty strings)
	// Hold-related fields remain blank (CM, CY, DA already default to empty strings)

	data.screenMessage = append(data.screenMessage, "Instance-level information only (no item data)")
	return data
}

// getPrimaryContributor extracts the primary contributor name from an instance
func getPrimaryContributor(instance *models.Instance) string {
	for _, contributor := range instance.Contributors {
		if contributor.Primary {
			return contributor.Name
		}
	}
	return ""
}

// getSummaryNote extracts the summary note from an instance
// Summary note type UUID: 10e2e11b-450f-45c8-b09b-0f819999966e
func getSummaryNote(instance *models.Instance) string {
	const summaryNoteTypeID = "10e2e11b-450f-45c8-b09b-0f819999966e"
	for _, note := range instance.Notes {
		if note.NoteTypeID == summaryNoteTypeID {
			return note.Note
		}
	}
	return ""
}

// getISBNs extracts all ISBN identifiers from an instance
// ISBN identifier type UUID: 8261054f-be78-422d-bd51-4ed9f33c3422
func getISBNs(instance *models.Instance) []string {
	const isbnTypeID = "8261054f-be78-422d-bd51-4ed9f33c3422"
	var isbns []string
	for _, identifier := range instance.Identifiers {
		if identifier.IdentifierTypeID == isbnTypeID {
			isbns = append(isbns, identifier.Value)
		}
	}
	return isbns
}

// getUPCs extracts all UPC identifiers from an instance
// UPC identifier type UUIDs:
//   - 2e8b3b6c-0e7d-4e48-bca2-b0b23b376af5 (Other standard identifier)
//   - 1795ea23-6856-48a5-a772-f356e16a8a6c (Other standard identifier)
func getUPCs(instance *models.Instance) []string {
	const upcTypeID1 = "2e8b3b6c-0e7d-4e48-bca2-b0b23b376af5"
	const upcTypeID2 = "1795ea23-6856-48a5-a772-f356e16a8a6c"
	var upcs []string
	for _, identifier := range instance.Identifiers {
		if identifier.IdentifierTypeID == upcTypeID1 || identifier.IdentifierTypeID == upcTypeID2 {
			upcs = append(upcs, identifier.Value)
		}
	}
	return upcs
}
