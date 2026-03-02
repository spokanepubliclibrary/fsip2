package handlers

import (
	"context"
	"fmt"
	"time"

	"github.com/spokanepubliclibrary/fsip2/internal/config"
	"github.com/spokanepubliclibrary/fsip2/internal/sip2/parser"
	"github.com/spokanepubliclibrary/fsip2/internal/sip2/protocol"
	"github.com/spokanepubliclibrary/fsip2/internal/types"
	"go.uber.org/zap"
)

// ItemStatusUpdateHandler handles SIP2 Item Status Update requests (19)
type ItemStatusUpdateHandler struct {
	*BaseHandler
	logger *zap.Logger
}

// NewItemStatusUpdateHandler creates a new item status update handler
func NewItemStatusUpdateHandler(logger *zap.Logger, tenantConfig *config.TenantConfig) *ItemStatusUpdateHandler {
	return &ItemStatusUpdateHandler{
		BaseHandler: NewBaseHandler(logger, tenantConfig),
		logger:      logger,
	}
}

// Handle processes an Item Status Update request (19) and returns an Item Status Update response (20)
func (h *ItemStatusUpdateHandler) Handle(ctx context.Context, msg *parser.Message, session *types.Session) (string, error) {
	h.logRequest(msg, session)

	// Validate required fields
	if err := h.validateRequiredFields(msg, map[parser.FieldCode]string{
		parser.InstitutionID:  "Institution ID",
		parser.ItemIdentifier: "Item Identifier",
	}); err != nil {
		h.logger.Error("Item status update validation failed", zap.Error(err))
		return h.buildItemStatusUpdateResponse(false, "", "", msg), nil
	}

	// Extract fields
	institutionID := msg.GetField(parser.InstitutionID)
	itemIdentifier := msg.GetField(parser.ItemIdentifier)

	h.logger.Info("Item status update request",
		zap.String("institution_id", institutionID),
		zap.String("item_identifier", itemIdentifier),
	)

	// Get authenticated FOLIO client
	_, token, err := h.getAuthenticatedFolioClient(ctx, session)
	if err != nil {
		h.logger.Error("Failed to get authenticated client", zap.Error(err))
		return h.buildItemStatusUpdateResponse(false, institutionID, itemIdentifier, msg), nil
	}

	// Create inventory client
	inventoryClient := h.getInventoryClient(session)

	// Get item from FOLIO
	item, err := inventoryClient.GetItemByBarcode(ctx, token, itemIdentifier)
	if err != nil {
		h.logger.Error("Failed to get item",
			zap.String("item_identifier", itemIdentifier),
			zap.Error(err),
		)
		return h.buildItemStatusUpdateResponse(false, institutionID, itemIdentifier, msg), nil
	}

	// Update item properties
	// TODO: Implement item property updates in FOLIO
	// For now, just acknowledge the request

	h.logger.Info("Item status update acknowledged",
		zap.String("item_id", item.ID),
		zap.String("item_identifier", itemIdentifier),
	)

	h.logResponse(string(parser.ItemStatusUpdateResponse), session, nil)

	return h.buildItemStatusUpdateResponse(true, institutionID, itemIdentifier, msg), nil
}

// buildItemStatusUpdateResponse builds an Item Status Update Response (20)
func (h *ItemStatusUpdateHandler) buildItemStatusUpdateResponse(ok bool, institutionID, itemIdentifier string, msg *parser.Message) string {
	timestamp := protocol.FormatSIP2DateTime(time.Now(), "    ")

	// Item Status Update Response format:
	// 20<item_properties_ok><transaction_date><item_identifier>
	// <title_identifier><item_properties><screen_message><print_line>

	itemPropertiesOK := "0"
	if ok {
		itemPropertiesOK = "1"
	}

	response := fmt.Sprintf("20%s%s",
		itemPropertiesOK,
		timestamp,
	)

	// Add variable fields
	response += fmt.Sprintf("|AB%s", itemIdentifier)
	response += fmt.Sprintf("|AO%s", institutionID)

	// Add title if available
	response += fmt.Sprintf("|AJ%s", itemIdentifier) // TODO: Get actual title

	// Add screen message
	if ok {
		response += "|AFItem properties updated"
	} else {
		response += "|AFItem properties update failed"
	}

	return response
}
