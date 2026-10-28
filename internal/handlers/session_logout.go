package handlers

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/spokanepubliclibrary/fsip2/internal/folio"
	"github.com/spokanepubliclibrary/fsip2/internal/logging"
	"github.com/spokanepubliclibrary/fsip2/internal/types"
	"go.uber.org/zap"
)

// logoutTimeout bounds one logout, including a refresh-first, so a slow gateway can never
// hold up a connection close or leave a goroutine running for long.
const logoutTimeout = 3 * time.Second

// Logout reasons, logged with every logout line.
const (
	LogoutReasonClientClosed     = "client_closed"     // the device closed the connection
	LogoutReasonReadTimeout      = "read_timeout"      // the idle read deadline expired
	LogoutReasonContextCancelled = "context_cancelled" // the server is stopping
	LogoutReasonError            = "error"             // any other read or write failure
	LogoutReasonRelogin93        = "relogin_93"        // a 93 Login replaced the pair
	LogoutReasonTenantChanged    = "tenant_changed"    // a 93 moved the session to another tenant and failed
	LogoutReasonRenewalFallback  = "renewal_fallback"  // a transient refresh failure forced a re-login
)

// LogoutSession takes the session's token pair and ends its Keycloak session.
// Used when the SIP connection closes; the session is left with no tokens in every case.
func LogoutSession(session *types.Session, logger *zap.Logger, reason string) {
	LogoutTokens(session.TakeTokens(), logger, session.ID, session.GetUsername(), reason)
}

// LogoutTokens ends the Keycloak session a token pair belongs to, best-effort.
//
// The request goes to the tenant that issued the pair, never the session's current one.
// If the access token is missing or expired, the pair is refreshed first: the gateway
// rejects a logout with an expired access token (401) and nothing is ended. A 401 on a
// token fsip2 believed current (clock drift) gets one refresh and retry. A refresh that is
// rejected means the session is already gone, so no logout is sent.
//
// AuthClient.Logout enforces the guards against the blank-refresh-token trap; this function
// never sends a request itself. Token values are never logged.
func LogoutTokens(pair types.TokenPair, logger *zap.Logger, sessionID, username, reason string) {
	if logger == nil {
		logger = zap.NewNop()
	}
	fields := []zap.Field{
		logging.TypeField(logging.TypeApplication),
		zap.String("reason", reason),
		zap.String("session_id", sessionID),
		zap.String("username", username),
	}

	if strings.TrimSpace(pair.RefreshToken) == "" {
		logger.Debug("Keycloak logout skipped", append(fields, zap.String("skip_reason", "no_refresh_token"))...)
		return
	}
	if !pair.RefreshExpiresAt.IsZero() && time.Now().After(pair.RefreshExpiresAt) {
		logger.Debug("Keycloak logout skipped", append(fields, zap.String("skip_reason", "refresh_token_expired"))...)
		return
	}
	if pair.OkapiURL == "" {
		logger.Debug("Keycloak logout skipped", append(fields, zap.String("skip_reason", "no_issuer"))...)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), logoutTimeout)
	defer cancel()
	authClient := folio.NewAuthClient(pair.OkapiURL, pair.OkapiTenant)

	accessToken, refreshToken := pair.AccessToken, pair.RefreshToken
	refreshedFirst := false

	// refreshPair swaps in a fresh pair from the same Keycloak session. It returns false
	// (after logging) when the session is already gone or the refresh failed.
	refreshPair := func() bool {
		resp, err := authClient.Refresh(ctx, refreshToken)
		if err != nil {
			if isClientError(err) {
				logger.Debug("Keycloak logout skipped",
					append(fields, zap.String("skip_reason", "refresh_rejected"), statusOrClass(err))...)
			} else {
				logger.Warn("Keycloak logout failed",
					append(fields, zap.String("stage", "refresh"), statusOrClass(err))...)
			}
			return false
		}
		accessToken, refreshToken = resp.OkapiToken, resp.RefreshToken
		refreshedFirst = true
		return true
	}

	if accessToken == "" || !time.Now().Before(pair.AccessExpiresAt) {
		if !refreshPair() {
			return
		}
	}

	err := authClient.Logout(ctx, accessToken, refreshToken)
	if isUnauthorized(err) && !refreshedFirst {
		// The gateway saw the access token as expired (clock drift): refresh once and retry.
		if !refreshPair() {
			return
		}
		err = authClient.Logout(ctx, accessToken, refreshToken)
	}
	fields = append(fields, zap.Bool("refreshed_first", refreshedFirst))

	switch {
	case err == nil:
		// A 204 is not proof that a live session was just ended: the gateway also answers
		// 204 for a session that has already ended.
		logger.Info("Keycloak logout accepted", fields...)
	case errors.Is(err, folio.ErrNoRefreshToken), errors.Is(err, folio.ErrInvalidRefreshToken), errors.Is(err, folio.ErrNoAccessToken):
		logger.Warn("Keycloak logout refused by guard", append(fields, zap.String("error_class", err.Error()))...)
	case isClientError(err):
		logger.Debug("Keycloak logout rejected", append(fields, statusOrClass(err))...)
	default:
		logger.Warn("Keycloak logout failed", append(fields, zap.String("stage", "logout"), statusOrClass(err))...)
	}
}

// isClientError reports whether err is an HTTP 4xx from FOLIO.
func isClientError(err error) bool {
	var httpErr *folio.HTTPError
	return errors.As(err, &httpErr) && httpErr.StatusCode >= 400 && httpErr.StatusCode < 500
}

// isUnauthorized reports whether err is an HTTP 401 from FOLIO.
func isUnauthorized(err error) bool {
	var httpErr *folio.HTTPError
	return errors.As(err, &httpErr) && httpErr.StatusCode == 401
}

// statusOrClass returns the HTTP status of err, or a short body-free error class.
func statusOrClass(err error) zap.Field {
	var httpErr *folio.HTTPError
	if errors.As(err, &httpErr) {
		return zap.Int("status_code", httpErr.StatusCode)
	}
	return zap.String("error_class", renewalErrorClass(err))
}
