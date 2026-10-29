package logging

import (
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestShouldLogMessage(t *testing.T) {
	tests := []struct {
		name        string
		messageCode string
		logLevel    string
		want        bool
	}{
		// None level - no messages logged
		{"None - Login", "93", "None", false},
		{"None - Patron Info", "63", "None", false},
		{"None - Checkout", "11", "None", false},

		// Debugging level - all messages logged
		{"Debugging - Login", "93", "Debugging", true},
		{"Debugging - Patron Info", "63", "Debugging", true},
		{"Debugging - Checkout", "11", "Debugging", true},
		{"Debugging - SC Status", "99", "Debugging", true},
		{"Debugging - ACS Status", "98", "Debugging", true},

		// Full level - all except 93/94 (login) and 98/99 (client heartbeat)
		{"Full - Login Request", "93", "Full", false},
		{"Full - Login Response", "94", "Full", false},
		{"Full - SC Status", "99", "Full", false},
		{"Full - ACS Status", "98", "Full", false},
		{"Full - Patron Info Request", "63", "Full", true},
		{"Full - Patron Info Response", "64", "Full", true},
		{"Full - Checkout", "11", "Full", true},
		{"Full - Checkin", "09", "Full", true},

		// Patron level - only 63/64 (patron info)
		{"Patron - Login", "93", "Patron", false},
		{"Patron - Patron Info Request", "63", "Patron", true},
		{"Patron - Patron Info Response", "64", "Patron", true},
		{"Patron - Checkout", "11", "Patron", false},
		{"Patron - Checkin", "09", "Patron", false},
		{"Patron - SC Status", "99", "Patron", false},

		// Invalid/unknown log level - defaults to None
		{"Invalid - Login", "93", "Invalid", false},
		{"Empty - Login", "93", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ShouldLogMessage(tt.messageCode, tt.logLevel)
			if got != tt.want {
				t.Errorf("ShouldLogMessage(%q, %q) = %v, want %v",
					tt.messageCode, tt.logLevel, got, tt.want)
			}
		})
	}
}

func TestIsQuietMessage(t *testing.T) {
	tests := []struct {
		name        string
		messageCode string
		logLevel    string
		want        bool
	}{
		{"Debugging - SC Status", "99", "Debugging", false},
		{"Debugging - ACS Status", "98", "Debugging", false},
		{"Full - SC Status", "99", "Full", true},
		{"Full - ACS Status", "98", "Full", true},
		{"Patron - SC Status", "99", "Patron", true},
		{"None - SC Status", "99", "None", true},
		{"Empty - SC Status", "99", "", true},
		{"Full - Checkout", "11", "Full", false},
		{"Full - Login", "93", "Full", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsQuietMessage(tt.messageCode, tt.logLevel)
			if got != tt.want {
				t.Errorf("IsQuietMessage(%q, %q) = %v, want %v",
					tt.messageCode, tt.logLevel, got, tt.want)
			}
		})
	}
}

func TestScopedLogger(t *testing.T) {
	tests := []struct {
		name        string
		messageCode string
		logLevel    string
		wantLevels  []zapcore.Level
	}{
		{"Full heartbeat drops Info/Debug", "99", "Full", []zapcore.Level{zapcore.WarnLevel, zapcore.ErrorLevel}},
		{"Debugging heartbeat keeps all", "99", "Debugging", []zapcore.Level{zapcore.DebugLevel, zapcore.InfoLevel, zapcore.WarnLevel, zapcore.ErrorLevel}},
		{"Full checkout keeps all", "11", "Full", []zapcore.Level{zapcore.DebugLevel, zapcore.InfoLevel, zapcore.WarnLevel, zapcore.ErrorLevel}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			core, logs := observer.New(zapcore.DebugLevel)
			logger := ScopedLogger(zap.New(core), tt.messageCode, tt.logLevel)

			logger.Debug("debug")
			logger.Info("info")
			logger.Warn("warn")
			logger.Error("error")

			entries := logs.All()
			if len(entries) != len(tt.wantLevels) {
				t.Fatalf("got %d entries, want %d", len(entries), len(tt.wantLevels))
			}
			for i, want := range tt.wantLevels {
				if entries[i].Level != want {
					t.Errorf("entry %d level = %v, want %v", i, entries[i].Level, want)
				}
			}
		})
	}
}

func TestObfuscateMessage(t *testing.T) {
	tests := []struct {
		name        string
		message     string
		messageCode string
		logLevel    string
		want        string
	}{
		// Debugging level - no obfuscation
		{
			name:        "Debugging - No obfuscation",
			message:     "93001.00|ADmyPIN123|COmyPassword456|",
			messageCode: "93",
			logLevel:    "Debugging",
			want:        "93001.00|ADmyPIN123|COmyPassword456|",
		},

		// Full level - obfuscate PINs
		{
			name:        "Full - Obfuscate PIN",
			message:     "63001.00|AA123456|ADmyPIN123|AO001|",
			messageCode: "63",
			logLevel:    "Full",
			want:        "63001.00|AA123456|AD********|AO001|",
		},
		{
			name:        "Full - Obfuscate multiple passwords",
			message:     "93001.00|ADmyPIN|COmyPassword|",
			messageCode: "93",
			logLevel:    "Full",
			want:        "93001.00|AD*****|CO**********|",
		},

		// Patron level - obfuscate PINs
		{
			name:        "Patron - Obfuscate PIN",
			message:     "63001.00|AA123456|AD1234|AO001|",
			messageCode: "63",
			logLevel:    "Patron",
			want:        "63001.00|AA123456|AD****|AO001|",
		},

		// Empty PIN/password fields
		{
			name:        "Full - Empty PIN field",
			message:     "63001.00|AA123456|AD|AO001|",
			messageCode: "63",
			logLevel:    "Full",
			want:        "63001.00|AA123456|AD|AO001|",
		},

		// No PIN/password fields
		{
			name:        "Full - No PIN fields",
			message:     "23001.00|AA123456|AO001|",
			messageCode: "23",
			logLevel:    "Full",
			want:        "23001.00|AA123456|AO001|",
		},

		// None level - no obfuscation (message shouldn't be logged anyway)
		{
			name:        "None - No obfuscation",
			message:     "93001.00|ADmyPIN123|COmyPassword456|",
			messageCode: "93",
			logLevel:    "None",
			want:        "93001.00|ADmyPIN123|COmyPassword456|",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ObfuscateMessage(tt.message, tt.messageCode, tt.logLevel)
			if got != tt.want {
				t.Errorf("ObfuscateMessage() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExtractMessageCode(t *testing.T) {
	tests := []struct {
		name    string
		message string
		want    string
	}{
		{"Login request", "93001.00|CNusername|COpassword|", "93"},
		{"Patron info", "63001.00|AA123456|ADpin|AO001|", "63"},
		{"Checkout", "11YN20230101    120000|AOinst|AB123|AC|ADpin|", "11"},
		{"Empty message", "", ""},
		{"Single char", "9", ""},
		{"Two chars", "93", "93"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractMessageCode(tt.message)
			if got != tt.want {
				t.Errorf("ExtractMessageCode(%q) = %q, want %q", tt.message, got, tt.want)
			}
		})
	}
}
