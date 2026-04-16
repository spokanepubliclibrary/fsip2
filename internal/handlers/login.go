package handlers

import (
	"context"
	"fmt"
	"time"

	"github.com/spokanepubliclibrary/fsip2/internal/config"
	"github.com/spokanepubliclibrary/fsip2/internal/folio"
	"github.com/spokanepubliclibrary/fsip2/internal/logging"
	"github.com/spokanepubliclibrary/fsip2/internal/sip2/builder"
	"github.com/spokanepubliclibrary/fsip2/internal/sip2/parser"
	"github.com/spokanepubliclibrary/fsip2/internal/types"
	"go.uber.org/zap"
)

// LoginHandler handles SIP2 Login requests (93)
type LoginHandler struct {
	*BaseHandler
	logger *zap.Logger
}

// NewLoginHandler creates a new login handler
func NewLoginHandler(logger *zap.Logger, tenantConfig *config.TenantConfig) *LoginHandler {
	return &LoginHandler{
		BaseHandler: NewBaseHandler(logger, tenantConfig),
		logger:      logger.With(logging.TypeField(logging.TypeApplication)),
	}
}

// Handle processes a Login request (93) and returns a Login response (94)
func (h *LoginHandler) Handle(ctx context.Context, msg *parser.Message, session *types.Session) (string, error) {
	h.logRequest(msg, session)

	// Validate required fields
	if err := h.validateRequiredFields(msg, map[parser.FieldCode]string{
		parser.LoginUserID:   "Login User ID",
		parser.LoginPassword: "Login Password",
	}); err != nil {
		h.logger.Error("Login validation failed", zap.Error(err))
		return h.buildLoginResponse(false, msg.SequenceNumber, session), nil
	}

	// Extract login credentials
	username := msg.GetField(parser.LoginUserID)
	password := msg.GetField(parser.LoginPassword)
	locationCode := msg.GetField(parser.LocationCode)

	h.logger.Info("Login attempt",
		zap.String("username", username),
		zap.String("location_code", locationCode),
	)

	// Store location code in session if provided
	if locationCode != "" {
		session.SetLocationCode(locationCode)
	}

	// Short-circuit: if the session already holds a valid (non-expired) token for
	// this user, skip the FOLIO round-trip entirely.  The per-request AuthClient
	// created below discards its TokenCache on return, so calling it again when the
	// token is still good wastes a network call and loses any cache benefit.
	if session.IsAuth() && !session.IsTokenExpired() {
		h.logger.Info("Login skipped — session token still valid",
			zap.String("username", username),
			zap.String("session_id", session.ID),
			zap.Time("token_expires_at", session.GetTokenExpiresAt()),
		)
		h.logResponse(string(parser.LoginResponse), session, nil)
		return h.buildLoginResponse(true, msg.SequenceNumber, session), nil
	}

	// Create auth client with token cache capacity of 100
	authClient := folio.NewAuthClient(session.TenantConfig.OkapiURL, session.TenantConfig.OkapiTenant, 100)

	// Authenticate with FOLIO
	authResp, err := authClient.Login(ctx, username, password)
	if err != nil {
		h.logger.Error("Authentication failed",
			zap.String("username", username),
			zap.Error(err),
		)
		return h.buildLoginResponse(false, msg.SequenceNumber, session), nil
	}

	// Determine token to use (prefer OkapiToken, fallback to AccessToken)
	token := authResp.OkapiToken
	if token == "" {
		token = authResp.AccessToken
	}

	if token == "" {
		h.logger.Error("No token received from FOLIO login",
			zap.String("username", username),
			zap.String("session_id", session.ID),
		)
		return h.buildLoginResponse(false, msg.SequenceNumber, session), nil
	}

	// Store authentication in session (username, patronID, patronBarcode, token, expiresAt)
	// For login handler, we don't have patron info yet, so use username for all
	session.SetAuthenticated(username, "", "", token, authResp.ExpiresAt)

	// Store credentials for automatic token refresh (Option A - Phase 3)
	// This enables getAuthenticatedFolioClient() to re-authenticate when token expires
	session.SetAuthCredentials(password)

	// Calculate time until expiration for logging
	timeUntilExpiry := time.Until(authResp.ExpiresAt)
	// 90s buffer is used in IsTokenExpired() check
	effectiveTimeRemaining := timeUntilExpiry - (90 * time.Second)

	h.logger.Info("Login successful",
		zap.String("username", username),
		zap.String("session_id", session.ID),
		zap.Bool("token_stored", session.GetAuthToken() != ""),
		zap.Int("token_length", len(token)),
		zap.Time("token_expires_at", authResp.ExpiresAt),
		zap.Duration("time_until_expiry", timeUntilExpiry),
		zap.Duration("effective_time_remaining_with_90s_buffer", effectiveTimeRemaining),
	)

	h.logResponse(string(parser.LoginResponse), session, nil)

	return h.buildLoginResponse(true, msg.SequenceNumber, session), nil
}

// buildLoginResponse builds a Login Response (94)
func (h *LoginHandler) buildLoginResponse(ok bool, sequenceNumber string, session *types.Session) string {
	// Login Response format: 94<ok><UIDAlgorithm><PWDAlgorithm>
	// ok: 0 = login failed, 1 = login successful
	// UIDAlgorithm: 0 = not encrypted
	// PWDAlgorithm: 0 = not encrypted

	okValue := "0"
	if ok {
		okValue = "1"
	}

	content := fmt.Sprintf("%s00", okValue)

	// Create builder with session's tenant config for proper error detection settings
	sessionBuilder := h.builder
	if session != nil && session.TenantConfig != nil {
		sessionBuilder = builder.NewResponseBuilder(session.TenantConfig)
	}

	// Use builder to add sequence number, checksum, and delimiter
	response, err := sessionBuilder.Build(parser.LoginResponse, content, sequenceNumber)
	if err != nil {
		h.logger.Error("Failed to build login response", zap.Error(err))
		// Fallback to simple response
		return "94" + content
	}

	return response
}
