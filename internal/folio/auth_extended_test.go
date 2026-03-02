package folio

import (
	"testing"

	"go.uber.org/zap"
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
