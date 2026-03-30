package handlers

import (
	"context"

	"github.com/spokanepubliclibrary/fsip2/internal/config"
	"github.com/spokanepubliclibrary/fsip2/internal/logging"
	"github.com/spokanepubliclibrary/fsip2/internal/sip2/parser"
	"github.com/spokanepubliclibrary/fsip2/internal/types"
	"go.uber.org/zap"
)

// ResendHandler handles SIP2 Resend requests (97)
type ResendHandler struct {
	*BaseHandler
	logger *zap.Logger
}

// NewResendHandler creates a new resend handler
func NewResendHandler(logger *zap.Logger, tenantConfig *config.TenantConfig) *ResendHandler {
	return &ResendHandler{
		BaseHandler: NewBaseHandler(logger, tenantConfig),
		logger:      logger.With(logging.TypeField(logging.TypeApplication)),
	}
}

// Handle processes a Resend request (97) and returns a Resend response (96)
func (h *ResendHandler) Handle(ctx context.Context, msg *parser.Message, session *types.Session) (string, error) {
	h.logRequest(msg, session)

	h.logger.Info("Resend request received",
		zap.String("session_id", session.ID),
	)

	// Resend requests are typically sent when a checksum fails or a message is corrupted
	// The proper response is to resend the last message
	// For now, we'll just acknowledge with a simple resend response

	h.logResponse(string(parser.RequestSCResend), session, nil)

	return h.buildResendResponse(), nil
}

// buildResendResponse builds a Resend Response (96)
func (h *ResendHandler) buildResendResponse() string {
	// Request SC/ACS Resend format: 96
	// This is the simplest SIP2 message - just the message code
	return "96"
}
