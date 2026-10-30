package types

import (
	"sync"
	"time"

	"github.com/spokanepubliclibrary/fsip2/internal/config"
)

// TokenExpiryBuffer is how long before a token's expiry it is treated as expired.
// It covers request latency and normal host/server clock drift.
const TokenExpiryBuffer = 90 * time.Second

// TokenPair is a session's access/refresh token pair together with the tenant that issued it.
// It holds credentials: never log it.
type TokenPair struct {
	AccessToken      string
	AccessExpiresAt  time.Time
	RefreshToken     string
	RefreshExpiresAt time.Time
	OkapiURL         string // Gateway that issued the pair
	OkapiTenant      string // Tenant that issued the pair
}

// IsEmpty reports whether the pair holds no refresh token (nothing to log out).
func (p TokenPair) IsEmpty() bool {
	return p.RefreshToken == ""
}

// Session represents a SIP2 client session
type Session struct {
	ID              string
	TenantConfig    *config.TenantConfig
	AuthToken       string
	TokenExpiresAt  time.Time // Token expiration time
	PatronID        string
	PatronBarcode   string
	Username     string
	password     string // Stored for automatic token refresh (Option A)
	LocationCode string
	IsAuthenticated bool
	CreatedAt       time.Time
	LastActivity    time.Time
	mu              sync.RWMutex

	// Refresh token for this connection's Keycloak session (from login-with-expiry or refresh).
	// It is a credential: never logged, never copied by Clone.
	refreshToken          string
	refreshTokenExpiresAt time.Time

	// Issuer of the stored token pair, recorded when the pair is stored. A 93 Login can move
	// the session to another tenant, so a logout must go to the tenant that issued the pair.
	tokenOkapiURL string
	tokenTenant   string

	// renewMu serialises token renewal so concurrent callers renew at most once.
	renewMu sync.Mutex
}

// NewSession creates a new session
func NewSession(id string, tenantConfig *config.TenantConfig) *Session {
	now := time.Now()
	return &Session{
		ID:              id,
		TenantConfig:    tenantConfig,
		IsAuthenticated: false,
		CreatedAt:       now,
		LastActivity:    now,
	}
}

// SetAuthenticated marks the session as authenticated
func (s *Session) SetAuthenticated(username, patronID, patronBarcode, authToken string, expiresAt time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.IsAuthenticated = true
	s.Username = username
	s.PatronID = patronID
	s.PatronBarcode = patronBarcode
	s.AuthToken = authToken
	s.TokenExpiresAt = expiresAt
	s.LastActivity = time.Now()
}

// SetAuthCredentials stores credentials for automatic token refresh (Option A)
// This should be called during login to enable token refresh when expired
func (s *Session) SetAuthCredentials(password string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.password = password
}

// GetAuthCredentials returns stored credentials for token refresh
// Returns username and password; password may be empty if not stored
func (s *Session) GetAuthCredentials() (username, password string) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Username, s.password
}

// HasAuthCredentials returns true if credentials are stored for token refresh
func (s *Session) HasAuthCredentials() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Username != "" && s.password != ""
}

// UpdateTokens replaces the access and refresh tokens and their expirations together.
// Refresh tokens rotate, so both must always be replaced as a pair. The session's current
// tenant is recorded as the pair's issuer.
func (s *Session) UpdateTokens(accessToken string, accessExpiresAt time.Time, refreshToken string, refreshExpiresAt time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.storeTokensLocked(accessToken, accessExpiresAt, refreshToken, refreshExpiresAt)
}

// SwapTokens stores a new token pair (as UpdateTokens) and returns the pair it replaced.
// Taking the old pair and storing the new one happen atomically, so the old pair can be
// logged out exactly once.
func (s *Session) SwapTokens(accessToken string, accessExpiresAt time.Time, refreshToken string, refreshExpiresAt time.Time) TokenPair {
	s.mu.Lock()
	defer s.mu.Unlock()
	old := s.tokenPairLocked()
	s.storeTokensLocked(accessToken, accessExpiresAt, refreshToken, refreshExpiresAt)
	return old
}

// TakeTokens returns the stored token pair and clears it (both tokens, both expirations and
// the issuer). A second caller gets an empty pair, so a pair is logged out at most once and a
// leftover access token can never be paired with a blank refresh token.
// Authentication state and stored credentials are left untouched.
func (s *Session) TakeTokens() TokenPair {
	s.mu.Lock()
	defer s.mu.Unlock()
	pair := s.tokenPairLocked()
	s.clearTokensLocked()
	return pair
}

// HasTokensFromOtherTenant reports whether a stored token pair was issued by a gateway or
// tenant other than the session's current one (a 93 Login moved the session to another tenant).
func (s *Session) HasTokensFromOtherTenant() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.refreshToken == "" || s.TenantConfig == nil {
		return false
	}
	return s.tokenOkapiURL != s.TenantConfig.OkapiURL || s.tokenTenant != s.TenantConfig.OkapiTenant
}

func (s *Session) storeTokensLocked(accessToken string, accessExpiresAt time.Time, refreshToken string, refreshExpiresAt time.Time) {
	s.AuthToken = accessToken
	s.TokenExpiresAt = accessExpiresAt
	s.refreshToken = refreshToken
	s.refreshTokenExpiresAt = refreshExpiresAt
	s.tokenOkapiURL, s.tokenTenant = "", ""
	if s.TenantConfig != nil {
		s.tokenOkapiURL = s.TenantConfig.OkapiURL
		s.tokenTenant = s.TenantConfig.OkapiTenant
	}
	s.LastActivity = time.Now()
}

func (s *Session) tokenPairLocked() TokenPair {
	return TokenPair{
		AccessToken:      s.AuthToken,
		AccessExpiresAt:  s.TokenExpiresAt,
		RefreshToken:     s.refreshToken,
		RefreshExpiresAt: s.refreshTokenExpiresAt,
		OkapiURL:         s.tokenOkapiURL,
		OkapiTenant:      s.tokenTenant,
	}
}

func (s *Session) clearTokensLocked() {
	s.AuthToken = ""
	s.TokenExpiresAt = time.Time{}
	s.refreshToken = ""
	s.refreshTokenExpiresAt = time.Time{}
	s.tokenOkapiURL, s.tokenTenant = "", ""
}

// GetRefreshToken returns the refresh token and its expiration
func (s *Session) GetRefreshToken() (string, time.Time) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.refreshToken, s.refreshTokenExpiresAt
}

// HasValidRefreshToken reports whether a refresh token is stored and will not expire
// within buffer. A refresh token with no known expiration is treated as not valid.
func (s *Session) HasValidRefreshToken(buffer time.Duration) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.refreshToken == "" || s.refreshTokenExpiresAt.IsZero() {
		return false
	}
	return time.Now().Add(buffer).Before(s.refreshTokenExpiresAt)
}

// LockRenewal acquires the per-session token renewal lock
func (s *Session) LockRenewal() {
	s.renewMu.Lock()
}

// UnlockRenewal releases the per-session token renewal lock
func (s *Session) UnlockRenewal() {
	s.renewMu.Unlock()
}

// UpdateActivity updates the last activity time
func (s *Session) UpdateActivity() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.LastActivity = time.Now()
}

// SetLocationCode sets the location code
func (s *Session) SetLocationCode(locationCode string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.LocationCode = locationCode
}

// GetAuthToken returns the authentication token
func (s *Session) GetAuthToken() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.AuthToken
}

// GetTokenExpiresAt returns the token expiration time
func (s *Session) GetTokenExpiresAt() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.TokenExpiresAt
}

// GetPatronID returns the patron ID
func (s *Session) GetPatronID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.PatronID
}

// SetPatronID sets the patron ID
func (s *Session) SetPatronID(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.PatronID = id
}

// SetPatronBarcode sets the patron barcode
func (s *Session) SetPatronBarcode(barcode string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.PatronBarcode = barcode
}

// GetPatronBarcode returns the patron barcode
func (s *Session) GetPatronBarcode() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.PatronBarcode
}

// GetUsername returns the username
func (s *Session) GetUsername() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Username
}

// GetLocationCode returns the location code
func (s *Session) GetLocationCode() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.LocationCode
}

// IsAuth checks if the session is authenticated
func (s *Session) IsAuth() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.IsAuthenticated
}

// IsTokenExpired checks if the authentication token is expired
func (s *Session) IsTokenExpired() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.TokenExpiresAt.IsZero() {
		return true // No expiration set, consider expired
	}
	return time.Now().Add(TokenExpiryBuffer).After(s.TokenExpiresAt)
}

// Clear resets all session state including ACS credentials.
// Used when the TCP connection is torn down or fully re-established.
// To reset between patrons while keeping the ACS login intact, use ClearPatron instead.
func (s *Session) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.IsAuthenticated = false
	s.Username = ""
	s.password = "" // Clear stored credentials for security
	s.PatronID = ""
	s.PatronBarcode = ""
	s.clearTokensLocked()
	s.LastActivity = time.Now()
}

// ClearPatron clears the current patron context while preserving ACS system credentials.
// Called by End Patron Session (35) to reset between patrons without dropping the
// system-level login established by Login (93).
func (s *Session) ClearPatron() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.PatronID = ""
	s.PatronBarcode = ""
	s.LastActivity = time.Now()
}

// GetDuration returns the session duration
func (s *Session) GetDuration() time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return time.Since(s.CreatedAt)
}

// GetIdleTime returns the time since last activity
func (s *Session) GetIdleTime() time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return time.Since(s.LastActivity)
}

// UpdateTenant updates the tenant configuration
func (s *Session) UpdateTenant(tenantConfig *config.TenantConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.TenantConfig = tenantConfig
}

// GetTenant returns the tenant configuration
func (s *Session) GetTenant() *config.TenantConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.TenantConfig
}

// Clone creates a copy of the session (for safe passing)
// Note: Does NOT copy stored credentials (password, refresh token) for security
func (s *Session) Clone() *Session {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return &Session{
		ID:              s.ID,
		TenantConfig:    s.TenantConfig,
		AuthToken:       s.AuthToken,
		TokenExpiresAt:  s.TokenExpiresAt,
		PatronID:        s.PatronID,
		PatronBarcode:   s.PatronBarcode,
		Username:     s.Username,
		// password and refresh token intentionally not copied for security
		LocationCode: s.LocationCode,
		IsAuthenticated: s.IsAuthenticated,
		CreatedAt:       s.CreatedAt,
		LastActivity:    s.LastActivity,
	}
}
