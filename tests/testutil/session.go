package testutil

import (
	"time"

	"github.com/spokanepubliclibrary/fsip2/internal/config"
	"github.com/spokanepubliclibrary/fsip2/internal/types"
)

// SessionOption is a functional option for building a Session.
type SessionOption func(*sessionOpts)

type sessionOpts struct {
	username, userID, barcode, token string
	expiresIn                        time.Duration
	locationCode                     string
}

func WithSessionUser(username, userID, barcode string) SessionOption {
	return func(o *sessionOpts) { o.username = username; o.userID = userID; o.barcode = barcode }
}

func WithExpiredToken() SessionOption {
	return func(o *sessionOpts) { o.expiresIn = -10 * time.Minute }
}

func WithLocationCode(code string) SessionOption {
	return func(o *sessionOpts) { o.locationCode = code }
}

// NewSession returns an unauthenticated session.
func NewSession(tc *config.TenantConfig) *types.Session {
	return types.NewSession("test-session", tc)
}

// NewAuthSession returns a session with a valid cached token (expires in 10 minutes by default).
func NewAuthSession(tc *config.TenantConfig, opts ...SessionOption) *types.Session {
	s := NewSession(tc)
	o := &sessionOpts{
		username:  "testuser",
		userID:    "user-123",
		barcode:   "123456",
		token:     "test-token",
		expiresIn: 10 * time.Minute,
	}
	for _, opt := range opts {
		opt(o)
	}
	s.SetAuthenticated(o.username, o.userID, o.barcode, o.token, time.Now().Add(o.expiresIn))
	if o.locationCode != "" {
		s.SetLocationCode(o.locationCode)
	}
	return s
}
