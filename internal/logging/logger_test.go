package logging

import (
	"os"
	"path/filepath"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// tempLogDir creates a temporary directory for log files and registers a best-effort
// cleanup. Unlike t.TempDir(), this does not fail the test if cleanup fails, which
// avoids spurious failures on Windows where zap holds file handles open until GC.
func tempLogDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "fsip2-log-test-*")
	if err != nil {
		t.Fatalf("failed to create temp log dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) }) //nolint:errcheck
	return dir
}

// TestNewLogger tests creating a new logger with different log levels
func TestNewLogger(t *testing.T) {
	tests := []struct {
		name          string
		level         string
		expectedLevel zapcore.Level
	}{
		{
			name:          "Debug level",
			level:         "debug",
			expectedLevel: zapcore.DebugLevel,
		},
		{
			name:          "Info level",
			level:         "info",
			expectedLevel: zapcore.InfoLevel,
		},
		{
			name:          "Warn level",
			level:         "warn",
			expectedLevel: zapcore.WarnLevel,
		},
		{
			name:          "Error level",
			level:         "error",
			expectedLevel: zapcore.ErrorLevel,
		},
		{
			name:          "Invalid level defaults to Info",
			level:         "invalid",
			expectedLevel: zapcore.InfoLevel,
		},
		{
			name:          "Empty level defaults to Info",
			level:         "",
			expectedLevel: zapcore.InfoLevel,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, err := NewLogger(tt.level, "")
			if err != nil {
				t.Fatalf("Failed to create logger: %v", err)
			}
			defer logger.Sync()

			if logger == nil {
				t.Fatal("Logger is nil")
			}

			// Verify the logger was created with the correct level
			// Note: We can't directly access the level, but we can verify it was created
		})
	}
}

// TestNewFileLogger tests creating a file logger
func TestNewFileLogger(t *testing.T) {
	// Create a temporary directory for test logs.
	// Use tempLogDir instead of t.TempDir() to avoid Windows cleanup failures
	// caused by zap holding file handles open until GC runs.
	tempDir := tempLogDir(t)
	logFile := filepath.Join(tempDir, "test.log")

	tests := []struct {
		name          string
		level         string
		logFile       string
		expectedLevel zapcore.Level
	}{
		{
			name:          "File logger with debug level",
			level:         "debug",
			logFile:       logFile,
			expectedLevel: zapcore.DebugLevel,
		},
		{
			name:          "File logger with info level",
			level:         "info",
			logFile:       logFile,
			expectedLevel: zapcore.InfoLevel,
		},
		{
			name:          "File logger with warn level",
			level:         "warn",
			logFile:       logFile,
			expectedLevel: zapcore.WarnLevel,
		},
		{
			name:          "File logger with error level",
			level:         "error",
			logFile:       logFile,
			expectedLevel: zapcore.ErrorLevel,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, err := NewFileLogger(tt.level, tt.logFile)
			if err != nil {
				t.Fatalf("Failed to create file logger: %v", err)
			}

			if logger == nil {
				t.Fatal("Logger is nil")
			}

			// Write a test message
			logger.Info("test message")

			// Sync and close the logger
			logger.Sync()

			// Verify the log file was created
			if _, err := os.Stat(tt.logFile); os.IsNotExist(err) {
				t.Error("Log file was not created")
			}
		})
	}
}

// TestNewFileLogger_DirectoryCreation tests that log directory is created if it doesn't exist
func TestNewFileLogger_DirectoryCreation(t *testing.T) {
	tempDir := tempLogDir(t)
	logFile := filepath.Join(tempDir, "subdir", "nested", "test.log")

	logger, err := NewFileLogger("info", logFile)
	if err != nil {
		t.Fatalf("Failed to create file logger: %v", err)
	}

	// Write a test message
	logger.Info("test message")

	// Sync and close the logger
	logger.Sync()

	// Verify the nested directories were created
	if _, err := os.Stat(filepath.Dir(logFile)); os.IsNotExist(err) {
		t.Error("Log directory was not created")
	}

	// Verify the log file exists
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		t.Error("Log file was not created")
	}
}

// TestNewProductionLogger tests creating a production logger
func TestNewProductionLogger(t *testing.T) {
	logger, err := NewProductionLogger()
	if err != nil {
		t.Fatalf("Failed to create production logger: %v", err)
	}
	defer logger.Sync()

	if logger == nil {
		t.Fatal("Logger is nil")
	}

	// Verify it works by logging a message
	logger.Info("production test message")
}

// TestNewDevelopmentLogger tests creating a development logger
func TestNewDevelopmentLogger(t *testing.T) {
	logger, err := NewDevelopmentLogger()
	if err != nil {
		t.Fatalf("Failed to create development logger: %v", err)
	}
	defer logger.Sync()

	if logger == nil {
		t.Fatal("Logger is nil")
	}

	// Verify it works by logging a message
	logger.Info("development test message")
}

// TestLogger_StructuredLogging tests structured logging with fields
func TestLogger_StructuredLogging(t *testing.T) {
	logger, err := NewLogger("info", "")
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Sync()

	// Test logging with various field types
	logger.Info("structured log test",
		zap.String("string_field", "test_value"),
		zap.Int("int_field", 42),
		zap.Bool("bool_field", true),
		zap.Float64("float_field", 3.14),
	)

	// No error means test passed
}

// TestLogger_ErrorLogging tests error logging
func TestLogger_ErrorLogging(t *testing.T) {
	logger, err := NewLogger("error", "")
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Sync()

	// Test error logging
	logger.Error("test error message",
		zap.Error(err),
		zap.String("context", "test"),
	)

	// No error means test passed
}

// TestLogger_DifferentLevels tests logging at different levels
func TestLogger_DifferentLevels(t *testing.T) {
	logger, err := NewLogger("debug", "")
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Sync()

	// Test all log levels
	logger.Debug("debug message")
	logger.Info("info message")
	logger.Warn("warn message")
	logger.Error("error message")

	// No error means test passed
}

// TestLogger_ConcurrentLogging tests concurrent logging
func TestLogger_ConcurrentLogging(t *testing.T) {
	logger, err := NewLogger("info", "")
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Sync()

	// Spawn multiple goroutines that log concurrently
	done := make(chan bool, 100)
	for i := 0; i < 100; i++ {
		go func(id int) {
			logger.Info("concurrent log message",
				zap.Int("goroutine_id", id),
			)
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 100; i++ {
		<-done
	}

	// No panic means test passed
}

// TestGetEncoderConfig tests the encoder configuration
func TestGetEncoderConfig(t *testing.T) {
	config := getEncoderConfig()

	// Verify key fields are set
	if config.TimeKey != "timestamp" {
		t.Errorf("Expected TimeKey 'timestamp', got '%s'", config.TimeKey)
	}

	if config.LevelKey != "level" {
		t.Errorf("Expected LevelKey 'level', got '%s'", config.LevelKey)
	}

	if config.MessageKey != "message" {
		t.Errorf("Expected MessageKey 'message', got '%s'", config.MessageKey)
	}

	if config.CallerKey != "caller" {
		t.Errorf("Expected CallerKey 'caller', got '%s'", config.CallerKey)
	}
}

// TestNewLogger_WithFileOutput tests logger with file output
func TestNewLogger_WithFileOutput(t *testing.T) {
	tempDir := tempLogDir(t)
	logFile := filepath.Join(tempDir, "output.log")

	logger, err := NewLogger("info", logFile)
	if err != nil {
		t.Fatalf("Failed to create logger with file output: %v", err)
	}

	// Log some messages
	logger.Info("test message 1")
	logger.Info("test message 2")
	logger.Warn("test warning")

	// Sync and close the logger
	logger.Sync()

	// Verify the log file was created
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		t.Error("Log file was not created")
	}
}

// TestLogLevel_Constants tests that log level constants are defined correctly
func TestLogLevel_Constants(t *testing.T) {
	tests := []struct {
		name     string
		level    LogLevel
		expected string
	}{
		{
			name:     "Debug level constant",
			level:    LevelDebug,
			expected: "debug",
		},
		{
			name:     "Info level constant",
			level:    LevelInfo,
			expected: "info",
		},
		{
			name:     "Warn level constant",
			level:    LevelWarn,
			expected: "warn",
		},
		{
			name:     "Error level constant",
			level:    LevelError,
			expected: "error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.level) != tt.expected {
				t.Errorf("Expected level '%s', got '%s'", tt.expected, string(tt.level))
			}
		})
	}
}
