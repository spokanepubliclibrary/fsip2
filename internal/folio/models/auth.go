package models

import "time"

// LoginRequest represents a login request to FOLIO
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// LoginResponse represents a login response from FOLIO
type LoginResponse struct {
	OkapiToken   string    `json:"okapiToken"`
	RefreshToken string    `json:"refreshToken,omitempty"`
	AccessToken  string    `json:"accessToken,omitempty"`
	TokenType    string    `json:"tokenType,omitempty"`
	ExpiresIn    int       `json:"expiresIn,omitempty"`
	ExpiresAt    time.Time `json:"-"` // Calculated expiration time (not from JSON)
	User         *User     `json:"user,omitempty"`
}

// TokenCache represents a cached authentication token
type TokenCache struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
	UserID       string
	Username     string
}

// IsExpired checks if the token is expired (with buffer)
func (tc *TokenCache) IsExpired() bool {
	// Consider expired 90 seconds before actual expiration (buffer)
	buffer := 90 * time.Second
	return time.Now().Add(buffer).After(tc.ExpiresAt)
}

// NeedsRefresh checks if the token needs to be refreshed
func (tc *TokenCache) NeedsRefresh() bool {
	return tc.IsExpired()
}
