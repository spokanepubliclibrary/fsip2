package folio

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/spokanepubliclibrary/fsip2/internal/folio/models"
	"github.com/spokanepubliclibrary/fsip2/internal/logging"
	"go.uber.org/zap"
)

// Cookie names used by /authn/login-with-expiry, /authn/refresh and /authn/logout
// (mod-login-keycloak on Eureka, mod-login v7.10.0+ / Poppy on Okapi).
const (
	AccessTokenCookie  = "folioAccessToken"
	RefreshTokenCookie = "folioRefreshToken"
)

// ErrNoRefreshToken is returned when a refresh token is required but empty.
// No HTTP call is made in that case.
var ErrNoRefreshToken = errors.New("no refresh token")

// Package-level logger for auth operations (can be set via SetAuthLogger)
var authLogger *zap.Logger

// SetAuthLogger sets the logger for auth operations
func SetAuthLogger(logger *zap.Logger) {
	authLogger = logger
}

// AuthClient handles authentication with FOLIO.
// It holds no token state: tokens are owned per SIP connection by types.Session.
type AuthClient struct {
	client *Client
}

// NewAuthClient creates a new authentication client
func NewAuthClient(baseURL, tenant string) *AuthClient {
	return &AuthClient{
		client: NewClient(baseURL, tenant),
	}
}

// Login authenticates a user via /authn/login-with-expiry.
// The access and refresh tokens come from the response cookies; the expirations from the body.
func (ac *AuthClient) Login(ctx context.Context, username, password string) (*models.LoginResponse, error) {
	loginReq := models.LoginRequest{
		Username: username,
		Password: password,
	}

	var loginResp models.LoginResponse
	cookies, err := ac.client.PostWithCookies(ctx, "/authn/login-with-expiry", "", loginReq, &loginResp)
	if err != nil {
		return nil, fmt.Errorf("login failed: %w", err)
	}

	if err := fillTokens(&loginResp, cookies); err != nil {
		return nil, fmt.Errorf("login failed: %w", err)
	}

	logTokenExpirations("login", username, &loginResp)
	return &loginResp, nil
}

// Refresh exchanges a refresh token for a new access/refresh token pair via /authn/refresh.
// The refresh token is rotated: callers must replace both stored tokens with the result.
func (ac *AuthClient) Refresh(ctx context.Context, refreshToken string) (*models.LoginResponse, error) {
	if refreshToken == "" {
		return nil, ErrNoRefreshToken
	}

	var refreshResp models.LoginResponse
	cookies, err := ac.client.PostWithCookies(ctx, "/authn/refresh", "", nil, &refreshResp,
		&http.Cookie{Name: RefreshTokenCookie, Value: refreshToken})
	if err != nil {
		return nil, fmt.Errorf("token refresh failed: %w", err)
	}

	if err := fillTokens(&refreshResp, cookies); err != nil {
		return nil, fmt.Errorf("token refresh failed: %w", err)
	}

	logTokenExpirations("refresh", "", &refreshResp)
	return &refreshResp, nil
}

// fillTokens copies the tokens from the response cookies into resp and sets ExpiresAt.
func fillTokens(resp *models.LoginResponse, cookies []*http.Cookie) error {
	access, refresh, err := extractTokens(cookies)
	if err != nil {
		return err
	}
	if resp.AccessTokenExpiration.IsZero() {
		return fmt.Errorf("response has no accessTokenExpiration")
	}

	resp.OkapiToken = access
	resp.RefreshToken = refresh
	resp.ExpiresAt = resp.AccessTokenExpiration
	return nil
}

// extractTokens returns the access and refresh tokens from the response cookies.
// It returns an error naming any cookie that is missing or empty.
func extractTokens(cookies []*http.Cookie) (access, refresh string, err error) {
	for _, cookie := range cookies {
		switch cookie.Name {
		case AccessTokenCookie:
			if access == "" {
				access = cookie.Value
			}
		case RefreshTokenCookie:
			if refresh == "" {
				refresh = cookie.Value
			}
		}
	}

	if access == "" {
		return "", "", fmt.Errorf("response has no %s cookie", AccessTokenCookie)
	}
	if refresh == "" {
		return "", "", fmt.Errorf("response has no %s cookie", RefreshTokenCookie)
	}
	return access, refresh, nil
}

// logTokenExpirations logs both expirations at debug level. Token values are never logged.
func logTokenExpirations(operation, username string, resp *models.LoginResponse) {
	if authLogger == nil {
		return
	}
	authLogger.Debug("FOLIO token expiration details",
		logging.TypeField(logging.TypeApplication),
		zap.String("operation", operation),
		zap.String("username", username),
		zap.Time("access_expires_at", resp.AccessTokenExpiration),
		zap.Time("refresh_expires_at", resp.RefreshTokenExpiration),
	)
}
