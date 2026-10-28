package folio

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

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

// ErrInvalidRefreshToken is returned by Logout when the refresh token is not JWT-shaped.
// No HTTP call is made in that case.
var ErrInvalidRefreshToken = errors.New("refresh token is not JWT-shaped")

// ErrNoAccessToken is returned by Logout when the access token is empty.
// No HTTP call is made in that case.
var ErrNoAccessToken = errors.New("no access token")

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

// Logout ends the Keycloak session that refreshToken belongs to via /authn/logout.
//
// TRAP: on Eureka (mod-login-keycloak), a logout that carries an access token (header or
// cookie) with a missing, blank or whitespace refresh token ends EVERY session of the user,
// i.e. every SIP device sharing the kiosk username. The guards below therefore run before
// any request is built, and the access token is only ever sent from here, as a cookie,
// next to a validated refresh token. The X-Okapi-Token header is never sent.
//
// The access token must be current: the gateway rejects an expired one (401) and nothing
// is ended. Callers refresh first when it has expired.
func (ac *AuthClient) Logout(ctx context.Context, accessToken, refreshToken string) error {
	if strings.TrimSpace(refreshToken) == "" {
		return ErrNoRefreshToken
	}
	if !isJWTShaped(refreshToken) {
		return ErrInvalidRefreshToken
	}
	if strings.TrimSpace(accessToken) == "" {
		return ErrNoAccessToken
	}

	_, err := ac.client.PostWithCookies(ctx, "/authn/logout", "", nil, nil,
		&http.Cookie{Name: RefreshTokenCookie, Value: refreshToken},
		&http.Cookie{Name: AccessTokenCookie, Value: accessToken})
	if err != nil {
		return fmt.Errorf("logout failed: %w", err)
	}
	return nil
}

// isJWTShaped reports whether token is in JWT compact form: 3 (signed) or 5 (encrypted)
// non-empty, dot-separated segments. It catches corrupted values such as "null" or a
// truncated token that are not blank but that the server might treat as blank.
func isJWTShaped(token string) bool {
	if strings.ContainsAny(token, " \t\r\n") {
		return false
	}
	segments := strings.Split(token, ".")
	if len(segments) != 3 && len(segments) != 5 {
		return false
	}
	for _, segment := range segments {
		if segment == "" {
			return false
		}
	}
	return true
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
