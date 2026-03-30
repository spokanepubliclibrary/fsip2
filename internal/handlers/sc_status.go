package handlers

import (
	"context"
	"fmt"
	"time"

	"github.com/spokanepubliclibrary/fsip2/internal/config"
	"github.com/spokanepubliclibrary/fsip2/internal/logging"
	"github.com/spokanepubliclibrary/fsip2/internal/sip2/builder"
	"github.com/spokanepubliclibrary/fsip2/internal/sip2/parser"
	"github.com/spokanepubliclibrary/fsip2/internal/sip2/protocol"
	"github.com/spokanepubliclibrary/fsip2/internal/types"
	"go.uber.org/zap"
)

// SCStatusHandler handles SIP2 SC Status requests (99)
type SCStatusHandler struct {
	*BaseHandler
	logger *zap.Logger
}

// NewSCStatusHandler creates a new SC status handler
func NewSCStatusHandler(logger *zap.Logger, tenantConfig *config.TenantConfig) *SCStatusHandler {
	return &SCStatusHandler{
		BaseHandler: NewBaseHandler(logger, tenantConfig),
		logger:      logger.With(logging.TypeField(logging.TypeApplication)),
	}
}

// Handle processes an SC Status request (99) and returns an ACS Status response (98)
func (h *SCStatusHandler) Handle(ctx context.Context, msg *parser.Message, session *types.Session) (string, error) {
	h.logRequest(msg, session)

	h.logger.Info("SC Status request",
		zap.String("session_id", session.ID),
	)

	// Build ACS Status Response
	response := h.buildACSStatusResponse(session, msg.SequenceNumber)

	h.logResponse("ACS Status Response", session, nil)

	return response, nil
}

// buildACSStatusResponse builds an ACS Status Response (98)
func (h *SCStatusHandler) buildACSStatusResponse(session *types.Session, sequenceNumber string) string {
	// ACS Status Response format:
	// 98<online_status><checkin_ok><checkout_ok><acs_renewal_policy>
	// <status_update_ok><offline_ok><timeout_period><retries_allowed>
	// <date_time_sync><protocol_version><institution_id><library_name>
	// <supported_messages><terminal_location><screen_message><print_line>

	timestamp := protocol.FormatSIP2DateTime(time.Now(), "    ")

	// Status flags derived from configuration
	onlineStatus := "Y" // Always online (service is running)

	// Checkin OK: Check if message 09 (Checkin) is supported
	checkinOK := "N"
	if session.TenantConfig.IsMessageSupported("09") {
		checkinOK = "Y"
	}

	// Checkout OK: Check if message 11 (Checkout) is supported
	checkoutOK := "N"
	if session.TenantConfig.IsMessageSupported("11") {
		checkoutOK = "Y"
	}

	// ACS Renewal Policy: Always N (FOLIO handles renewal authentication)
	acsRenewalPolicy := "N"

	// Status Update OK: From tenant configuration
	statusUpdateOK := "N"
	if session.TenantConfig.StatusUpdateOk {
		statusUpdateOK = "Y"
	}

	// Offline OK: From tenant configuration
	offlineOK := "N"
	if session.TenantConfig.OfflineOk {
		offlineOK = "Y"
	}

	// Timeout and retries from configuration (with defaults)
	timeoutPeriod := session.TenantConfig.GetTimeoutPeriod()
	retriesAllowed := session.TenantConfig.GetRetriesAllowed()

	// Protocol version
	protocolVersion := "2.00"

	// Institution details — SC Status (msg 99) carries no AO field; use configured tenant name
	institutionID := session.TenantConfig.Tenant

	libraryName := session.TenantConfig.Tenant

	// Supported messages (BX field) - built from configuration
	supportedMessages := session.TenantConfig.BuildSupportedMessages()

	// Terminal location from session (CP field from login)
	terminalLocation := session.GetLocationCode()

	// Build content (everything after message code)
	content := fmt.Sprintf("%s%s%s%s%s%s%s%s%s%s",
		onlineStatus,
		checkinOK,
		checkoutOK,
		acsRenewalPolicy,
		statusUpdateOK,
		offlineOK,
		timeoutPeriod,
		retriesAllowed,
		timestamp,
		protocolVersion,
	)

	// Add variable fields
	content += fmt.Sprintf("|AO%s", institutionID)
	content += fmt.Sprintf("|AM%s", libraryName)
	content += fmt.Sprintf("|BX%s", supportedMessages)

	if terminalLocation != "" {
		content += fmt.Sprintf("|AN%s", terminalLocation)
	}

	// Create builder with session's tenant config for proper error detection settings
	sessionBuilder := h.builder
	if session != nil && session.TenantConfig != nil {
		sessionBuilder = builder.NewResponseBuilder(session.TenantConfig)
	}

	// Use builder to add sequence number, checksum, and delimiter
	response, err := sessionBuilder.Build(parser.ACSStatus, content, sequenceNumber)
	if err != nil {
		h.logger.Error("Failed to build ACS status response", zap.Error(err))
		// Fallback to simple response
		return "98" + content
	}

	return response
}
