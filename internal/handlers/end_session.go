package handlers

import (
	"context"
	"fmt"
	"time"

	"github.com/spokanepubliclibrary/fsip2/internal/config"
	"github.com/spokanepubliclibrary/fsip2/internal/logging"
	"github.com/spokanepubliclibrary/fsip2/internal/sip2/parser"
	"github.com/spokanepubliclibrary/fsip2/internal/sip2/protocol"
	"github.com/spokanepubliclibrary/fsip2/internal/types"
	"go.uber.org/zap"
)

// EndSessionHandler handles SIP2 End Patron Session requests (35)
type EndSessionHandler struct {
	*BaseHandler
	logger *zap.Logger
}

// NewEndSessionHandler creates a new end session handler
func NewEndSessionHandler(logger *zap.Logger, tenantConfig *config.TenantConfig) *EndSessionHandler {
	return &EndSessionHandler{
		BaseHandler: NewBaseHandler(logger, tenantConfig),
		logger:      logger.With(logging.TypeField(logging.TypeApplication)),
	}
}

// Handle processes an End Patron Session request (35) and returns an End Session response (36)
func (h *EndSessionHandler) Handle(ctx context.Context, msg *parser.Message, session *types.Session) (string, error) {
	h.logRequest(msg, session)

	// Validate required fields
	if err := h.validateRequiredFields(msg, map[parser.FieldCode]string{
		parser.PatronIdentifier: "Patron Identifier",
	}); err != nil {
		h.logger.Error("End session validation failed", zap.Error(err))
		return h.buildEndSessionResponse(false, "", "", msg), nil
	}

	// Extract fields
	institutionID := msg.GetField(parser.InstitutionID)
	patronIdentifier := msg.GetField(parser.PatronIdentifier)

	h.logger.Info("End patron session request",
		zap.String("institution_id", institutionID),
		zap.String("patron_identifier", patronIdentifier),
		zap.String("session_id", session.ID),
	)

	// Clear session data
	session.Clear()

	h.logger.Info("Patron session ended",
		zap.String("session_id", session.ID),
		zap.String("patron_identifier", patronIdentifier),
	)

	h.logResponse(string(parser.EndPatronSessionResponse), session, nil)

	return h.buildEndSessionResponse(true, institutionID, patronIdentifier, msg), nil
}

// buildEndSessionResponse builds an End Session Response (36)
func (h *EndSessionHandler) buildEndSessionResponse(ok bool, institutionID, patronIdentifier string, msg *parser.Message) string {
	timestamp := protocol.FormatSIP2DateTime(time.Now(), "    ")

	// End Session Response format:
	// 36<end_session><transaction_date><institution_id><patron_identifier>
	// <screen_message><print_line>

	endSession := "N"
	if ok {
		endSession = "Y"
	}

	response := fmt.Sprintf("36%s%s",
		endSession,
		timestamp,
	)

	// Add variable fields
	response += fmt.Sprintf("|AO%s", institutionID)
	response += fmt.Sprintf("|AA%s", patronIdentifier)

	// Add screen message
	if ok {
		response += "|AFSession ended successfully"
	} else {
		response += "|AFFailed to end session"
	}

	return response
}
