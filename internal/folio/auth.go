package folio

import (
	"context"
	"fmt"
	"time"

	"github.com/spokanepubliclibrary/fsip2/internal/cache"
	"github.com/spokanepubliclibrary/fsip2/internal/folio/models"
	"go.uber.org/zap"
)

// Package-level logger for auth operations (can be set via SetAuthLogger)
var authLogger *zap.Logger

// SetAuthLogger sets the logger for auth operations
func SetAuthLogger(logger *zap.Logger) {
	authLogger = logger
}

// AuthClient handles authentication with FOLIO
type AuthClient struct {
	client     *Client
	tokenCache *cache.TokenCache
}

// NewAuthClient creates a new authentication client
func NewAuthClient(baseURL, tenant string, tokenCacheCapacity int) *AuthClient {
	return &AuthClient{
		client:     NewClient(baseURL, tenant),
		tokenCache: cache.NewTokenCache(tokenCacheCapacity),
	}
}

// Login authenticates a user and returns a token
func (ac *AuthClient) Login(ctx context.Context, username, password string) (*models.LoginResponse, error) {
	// Check cache first
	cacheKey := cache.BuildCacheKey(username, ac.client.tenant)
	if cachedToken, found := ac.tokenCache.Get(cacheKey); found {
		if !cachedToken.NeedsRefresh() {
			// Return cached token
			return &models.LoginResponse{
				OkapiToken:   cachedToken.AccessToken,
				RefreshToken: cachedToken.RefreshToken,
				AccessToken:  cachedToken.AccessToken,
				ExpiresAt:    cachedToken.ExpiresAt,
			}, nil
		}
	}

	// Perform login
	loginReq := models.LoginRequest{
		Username: username,
		Password: password,
	}

	var loginResp models.LoginResponse
	err := ac.client.Post(ctx, "/authn/login", "", loginReq, &loginResp)
	if err != nil {
		return nil, fmt.Errorf("login failed: %w", err)
	}

	// Determine token to use
	token := loginResp.OkapiToken
	if token == "" {
		token = loginResp.AccessToken
	}

	if token == "" {
		return nil, fmt.Errorf("no token received from login")
	}

	// Calculate expiration (default to 10 minutes if not provided)
	rawExpiresIn := loginResp.ExpiresIn
	expiresIn := rawExpiresIn
	usingDefault := false
	if expiresIn == 0 {
		expiresIn = 600 // 10 minutes default
		usingDefault = true
	}
	expiresAt := time.Now().Add(time.Duration(expiresIn) * time.Second)

	// Debug logging for token expiration troubleshooting (Phase 1.1/1.2)
	if authLogger != nil {
		authLogger.Info("FOLIO token expiration details",
			zap.String("username", username),
			zap.Int("raw_expires_in_from_folio", rawExpiresIn),
			zap.Int("effective_expires_in_seconds", expiresIn),
			zap.Bool("using_default_600s", usingDefault),
			zap.Time("calculated_expires_at", expiresAt),
			zap.Duration("token_lifetime", time.Duration(expiresIn)*time.Second),
		)
	}

	// Cache the token
	tokenCache := &models.TokenCache{
		AccessToken:  token,
		RefreshToken: loginResp.RefreshToken,
		Username:     username,
		ExpiresAt:    expiresAt,
	}

	// Get user ID if available
	if loginResp.User != nil {
		tokenCache.UserID = loginResp.User.ID
	}

	// Store in cache
	if err := ac.tokenCache.Set(cacheKey, tokenCache); err != nil {
		// Log warning but don't fail the request
		// Token is still valid, just not cached
	}

	// Set expiration in response
	loginResp.ExpiresAt = expiresAt

	return &loginResp, nil
}

// ValidateToken validates a token by attempting to use it
func (ac *AuthClient) ValidateToken(ctx context.Context, token string) (bool, error) {
	// Try to get current user with the token
	var user models.User
	err := ac.client.Get(ctx, "/users/_self", token, &user)
	if err != nil {
		if httpErr, ok := err.(*HTTPError); ok {
			if httpErr.IsUnauthorized() {
				return false, nil
			}
		}
		return false, err
	}

	return true, nil
}

// GetCachedToken retrieves a cached token
func (ac *AuthClient) GetCachedToken(username string) (*models.TokenCache, bool) {
	cacheKey := cache.BuildCacheKey(username, ac.client.tenant)
	return ac.tokenCache.Get(cacheKey)
}

// InvalidateToken removes a token from the cache
func (ac *AuthClient) InvalidateToken(username string) {
	cacheKey := cache.BuildCacheKey(username, ac.client.tenant)
	ac.tokenCache.Delete(cacheKey)
}

// ClearCache clears all cached tokens
func (ac *AuthClient) ClearCache() {
	ac.tokenCache.Clear()
}

// LoginAndCache performs login and ensures token is cached
func (ac *AuthClient) LoginAndCache(ctx context.Context, username, password string) (string, error) {
	loginResp, err := ac.Login(ctx, username, password)
	if err != nil {
		return "", err
	}

	token := loginResp.OkapiToken
	if token == "" {
		token = loginResp.AccessToken
	}

	return token, nil
}

// GetOrRefreshToken gets a cached token or refreshes it if needed
func (ac *AuthClient) GetOrRefreshToken(ctx context.Context, username, password string) (string, error) {
	// Try to get from cache
	cacheKey := cache.BuildCacheKey(username, ac.client.tenant)
	if cachedToken, found := ac.tokenCache.Get(cacheKey); found {
		if !cachedToken.NeedsRefresh() {
			return cachedToken.AccessToken, nil
		}
	}

	// Need to refresh - perform new login
	return ac.LoginAndCache(ctx, username, password)
}
