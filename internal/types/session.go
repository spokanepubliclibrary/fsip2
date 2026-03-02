package types

import (
	"sync"
	"time"

	"github.com/spokanepubliclibrary/fsip2/internal/config"
)

// Session represents a SIP2 client session
type Session struct {
	ID              string
	TenantConfig    *config.TenantConfig
	AuthToken       string
	TokenExpiresAt  time.Time // Token expiration time
	PatronID        string
	PatronBarcode   string
	Username        string
	password        string // Stored for automatic token refresh (Option A)
	InstitutionID   string
	LocationCode    string
	IsAuthenticated bool
	CreatedAt       time.Time
	LastActivity    time.Time
	mu              sync.RWMutex
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

// UpdateToken updates the authentication token and expiration time
// Used for token refresh without changing other session state
func (s *Session) UpdateToken(authToken string, expiresAt time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.AuthToken = authToken
	s.TokenExpiresAt = expiresAt
	s.LastActivity = time.Now()
}

// UpdateActivity updates the last activity time
func (s *Session) UpdateActivity() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.LastActivity = time.Now()
}

// SetInstitutionID sets the institution ID
func (s *Session) SetInstitutionID(institutionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.InstitutionID = institutionID
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

// GetInstitutionID returns the institution ID
func (s *Session) GetInstitutionID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.InstitutionID
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
	// Add 90s buffer for safety (same as token cache)
	return time.Now().Add(90 * time.Second).After(s.TokenExpiresAt)
}

// Clear clears session data (for end session)
func (s *Session) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.IsAuthenticated = false
	s.Username = ""
	s.password = "" // Clear stored credentials for security
	s.PatronID = ""
	s.PatronBarcode = ""
	s.AuthToken = ""
	s.TokenExpiresAt = time.Time{} // Reset to zero value
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
// Note: Does NOT copy stored credentials (password) for security
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
		Username:        s.Username,
		// password intentionally not copied for security
		InstitutionID:   s.InstitutionID,
		LocationCode:    s.LocationCode,
		IsAuthenticated: s.IsAuthenticated,
		CreatedAt:       s.CreatedAt,
		LastActivity:    s.LastActivity,
	}
}
