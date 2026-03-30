package folio

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/spokanepubliclibrary/fsip2/internal/folio/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestSetAuthLogger(t *testing.T) {
	// Initially nil
	if authLogger != nil {
		// Reset for test isolation
		authLogger = nil
	}

	// Set a no-op logger
	logger, err := zap.NewNop().Named("test"), error(nil)
	_ = err
	SetAuthLogger(logger)

	if authLogger == nil {
		t.Error("Expected authLogger to be set after SetAuthLogger")
	}

	// Set back to nil
	SetAuthLogger(nil)
	if authLogger != nil {
		t.Error("Expected authLogger to be nil after SetAuthLogger(nil)")
	}
}

func TestSetAuthLogger_WithDevelopmentLogger(t *testing.T) {
	// Verify a real logger can be set
	logger, err := zap.NewDevelopment()
	if err != nil {
		t.Fatalf("Failed to create development logger: %v", err)
	}
	defer logger.Sync()

	SetAuthLogger(logger)
	if authLogger == nil {
		t.Error("Expected authLogger to be set")
	}

	// Clean up
	SetAuthLogger(nil)
}

// TestLogin_TokenExpirationLog_IsDebugNotInfo verifies that the token expiration
// details log entry is emitted at debug level and NOT at info level.
func TestLogin_TokenExpirationLog_IsDebugNotInfo(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(models.LoginResponse{
			OkapiToken:  "tok",
			AccessToken: "tok",
			ExpiresIn:   600,
		})
	}))
	defer srv.Close()

	// Observer capturing everything from debug level up.
	core, recorded := observer.New(zap.DebugLevel)
	SetAuthLogger(zap.New(core))
	defer SetAuthLogger(nil)

	client := NewAuthClient(srv.URL, "test-tenant", 100)
	_, err := client.Login(context.Background(), "user", "pass")
	require.NoError(t, err)

	var tokenLog *observer.LoggedEntry
	for _, e := range recorded.All() {
		if e.Message == "FOLIO token expiration details" {
			entry := e
			tokenLog = &entry
			break
		}
	}
	require.NotNil(t, tokenLog, "expected 'FOLIO token expiration details' log entry")

	// Must be debug, not info.
	assert.Equal(t, zap.DebugLevel, tokenLog.Level, "token expiration log must be debug level")
	assert.Equal(t, "application", tokenLog.ContextMap()["type"])

	// Confirm nothing is logged at info level for token expiration.
	for _, e := range recorded.All() {
		if e.Message == "FOLIO token expiration details" {
			assert.NotEqual(t, zap.InfoLevel, e.Level,
				"token expiration details must not appear at info level")
		}
	}
}
