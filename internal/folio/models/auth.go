package models

import "time"

// LoginRequest represents a login request to FOLIO
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// LoginResponse represents the response from /authn/login-with-expiry and /authn/refresh.
//
// The body carries only the two expirations (ISO-8601 UTC). The tokens themselves arrive
// as folioAccessToken / folioRefreshToken Set-Cookie headers; the auth client copies
// them into OkapiToken and RefreshToken, which are never read from or written to JSON.
type LoginResponse struct {
	AccessTokenExpiration  time.Time `json:"accessTokenExpiration"`
	RefreshTokenExpiration time.Time `json:"refreshTokenExpiration"`
	OkapiToken             string    `json:"-"` // From the folioAccessToken cookie; sent as X-Okapi-Token
	RefreshToken           string    `json:"-"` // From the folioRefreshToken cookie
	ExpiresAt              time.Time `json:"-"` // Same as AccessTokenExpiration
	User                   *User     `json:"user,omitempty"`
}
