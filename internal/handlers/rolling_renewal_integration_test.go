package handlers

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/spokanepubliclibrary/fsip2/internal/config"
	"github.com/spokanepubliclibrary/fsip2/internal/folio"
	"github.com/spokanepubliclibrary/fsip2/internal/folio/models"
	"github.com/spokanepubliclibrary/fsip2/tests/testutil"
)

// TestAttemptRollingRenewal_Integration tests rolling renewal eligibility windows,
// dry-run mode, patron group filtering, and disabled state.
func TestAttemptRollingRenewal_Integration(t *testing.T) {
	patronGroupUndergradID := "undergrad-group-123"
	patronGroupFacultyID := "faculty-group-456"
	patronGroupGuestID := "guest-group-789"

	tests := []struct {
		name             string
		renewalConfig    *config.RollingRenewalConfig
		user             *models.User
		today            time.Time
		expectUpdate     bool
		expectLogMessage string
		expectLogLevel   zapcore.Level
	}{
		{
			name: "Eligible for renewal - within 6 months window",
			renewalConfig: &config.RollingRenewalConfig{
				Enabled: true, RenewWithin: "6M", ExtendFor: "1Y",
				ExtendExpired: true, DryRun: false, SelectPatrons: false,
			},
			user: &models.User{
				ID: "user-1", Username: "student1", Barcode: "1234567",
				PatronGroup:    patronGroupUndergradID,
				Personal:       models.PersonalInfo{LastName: "Student", FirstName: "Test"},
				ExpirationDate: timePtr(time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)),
			},
			today:            time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC),
			expectUpdate:     true,
			expectLogMessage: "Rolling renewal successful",
			expectLogLevel:   zapcore.InfoLevel,
		},
		{
			name: "Not eligible - outside renewal window",
			renewalConfig: &config.RollingRenewalConfig{
				Enabled: true, RenewWithin: "6M", ExtendFor: "1Y",
				ExtendExpired: true, DryRun: false, SelectPatrons: false,
			},
			user: &models.User{
				ID: "user-2", Username: "student2", Barcode: "2345678",
				PatronGroup:    patronGroupUndergradID,
				Personal:       models.PersonalInfo{LastName: "Student", FirstName: "Another"},
				ExpirationDate: timePtr(time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)),
			},
			today:            time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC),
			expectUpdate:     false,
			expectLogMessage: "Rolling renewal not eligible",
			expectLogLevel:   zapcore.DebugLevel,
		},
		{
			name: "Eligible - expired account with extendExpired=true",
			renewalConfig: &config.RollingRenewalConfig{
				Enabled: true, RenewWithin: "6M", ExtendFor: "1Y",
				ExtendExpired: true, DryRun: false, SelectPatrons: false,
			},
			user: &models.User{
				ID: "user-3", Username: "expired1", Barcode: "3456789",
				PatronGroup:    patronGroupFacultyID,
				Personal:       models.PersonalInfo{LastName: "Expired", FirstName: "Account"},
				ExpirationDate: timePtr(time.Date(2024, 12, 1, 0, 0, 0, 0, time.UTC)),
			},
			today:            time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC),
			expectUpdate:     true,
			expectLogMessage: "Rolling renewal successful",
			expectLogLevel:   zapcore.InfoLevel,
		},
		{
			name: "Not eligible - expired account with extendExpired=false",
			renewalConfig: &config.RollingRenewalConfig{
				Enabled: true, RenewWithin: "6M", ExtendFor: "1Y",
				ExtendExpired: false, DryRun: false, SelectPatrons: false,
			},
			user: &models.User{
				ID: "user-4", Username: "expired2", Barcode: "4567890",
				PatronGroup:    patronGroupFacultyID,
				Personal:       models.PersonalInfo{LastName: "Expired", FirstName: "NoRenew"},
				ExpirationDate: timePtr(time.Date(2024, 12, 1, 0, 0, 0, 0, time.UTC)),
			},
			today:            time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC),
			expectUpdate:     false,
			expectLogMessage: "Rolling renewal not eligible",
			expectLogLevel:   zapcore.DebugLevel,
		},
		{
			name: "Eligible - selectPatrons with patron in allowed list",
			renewalConfig: &config.RollingRenewalConfig{
				Enabled: true, RenewWithin: "6M", ExtendFor: "1Y",
				ExtendExpired: true, DryRun: false, SelectPatrons: true,
				AllowedPatrons: []string{patronGroupUndergradID, patronGroupFacultyID},
			},
			user: &models.User{
				ID: "user-5", Username: "faculty1", Barcode: "5678901",
				PatronGroup:    patronGroupFacultyID,
				Personal:       models.PersonalInfo{LastName: "Faculty", FirstName: "Member"},
				ExpirationDate: timePtr(time.Date(2025, 5, 1, 0, 0, 0, 0, time.UTC)),
			},
			today:            time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC),
			expectUpdate:     true,
			expectLogMessage: "Rolling renewal successful",
			expectLogLevel:   zapcore.InfoLevel,
		},
		{
			name: "Not eligible - selectPatrons with patron not in allowed list",
			renewalConfig: &config.RollingRenewalConfig{
				Enabled: true, RenewWithin: "6M", ExtendFor: "1Y",
				ExtendExpired: true, DryRun: false, SelectPatrons: true,
				AllowedPatrons: []string{patronGroupUndergradID, patronGroupFacultyID},
			},
			user: &models.User{
				ID: "user-6", Username: "guest1", Barcode: "6789012",
				PatronGroup:    patronGroupGuestID,
				Personal:       models.PersonalInfo{LastName: "Guest", FirstName: "User"},
				ExpirationDate: timePtr(time.Date(2025, 5, 1, 0, 0, 0, 0, time.UTC)),
			},
			today:            time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC),
			expectUpdate:     false,
			expectLogMessage: "Rolling renewal not eligible",
			expectLogLevel:   zapcore.DebugLevel,
		},
		{
			name: "Dry-run mode - eligible but should not update",
			renewalConfig: &config.RollingRenewalConfig{
				Enabled: true, RenewWithin: "6M", ExtendFor: "1Y",
				ExtendExpired: true, DryRun: true, SelectPatrons: false,
			},
			user: &models.User{
				ID: "user-7", Username: "dryrun1", Barcode: "7890123",
				PatronGroup:    patronGroupUndergradID,
				Personal:       models.PersonalInfo{LastName: "DryRun", FirstName: "Test"},
				ExpirationDate: timePtr(time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)),
			},
			today:            time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC),
			expectUpdate:     false,
			expectLogMessage: "would update expiration",
			expectLogLevel:   zapcore.InfoLevel,
		},
		{
			name: "Not eligible - rolling renewals disabled",
			renewalConfig: &config.RollingRenewalConfig{
				Enabled: false, RenewWithin: "6M", ExtendFor: "1Y",
				ExtendExpired: true, DryRun: false, SelectPatrons: false,
			},
			user: &models.User{
				ID: "user-8", Username: "disabled1", Barcode: "8901234",
				PatronGroup:    patronGroupUndergradID,
				Personal:       models.PersonalInfo{LastName: "Disabled", FirstName: "Test"},
				ExpirationDate: timePtr(time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)),
			},
			today:            time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC),
			expectUpdate:     false,
			expectLogMessage: "", // No log expected when disabled
		},
		{
			name: "Eligible - boundary case (exactly on renewal window boundary)",
			renewalConfig: &config.RollingRenewalConfig{
				Enabled: true, RenewWithin: "6M", ExtendFor: "1Y",
				ExtendExpired: true, DryRun: false, SelectPatrons: false,
			},
			user: &models.User{
				ID: "user-9", Username: "boundary1", Barcode: "9012345",
				PatronGroup:    patronGroupUndergradID,
				Personal:       models.PersonalInfo{LastName: "Boundary", FirstName: "Test"},
				ExpirationDate: timePtr(time.Date(2025, 7, 15, 0, 0, 0, 0, time.UTC)),
			},
			today:            time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC),
			expectUpdate:     true,
			expectLogMessage: "Rolling renewal successful",
			expectLogLevel:   zapcore.InfoLevel,
		},
		{
			name: "Eligible - using days period (30D window)",
			renewalConfig: &config.RollingRenewalConfig{
				Enabled: true, RenewWithin: "30D", ExtendFor: "1Y",
				ExtendExpired: true, DryRun: false, SelectPatrons: false,
			},
			user: &models.User{
				ID: "user-10", Username: "days1", Barcode: "0123456",
				PatronGroup:    patronGroupUndergradID,
				Personal:       models.PersonalInfo{LastName: "Days", FirstName: "Test"},
				ExpirationDate: timePtr(time.Date(2025, 2, 10, 0, 0, 0, 0, time.UTC)),
			},
			today:            time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC),
			expectUpdate:     true,
			expectLogMessage: "Rolling renewal successful",
			expectLogLevel:   zapcore.InfoLevel,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			core, recorded := observer.New(zap.DebugLevel)
			logger := zap.New(core)

			tc := testutil.NewTenantConfig(testutil.WithRollingRenewal(tt.renewalConfig))
			sess := testutil.NewAuthSession(tc)

			mockPatron := &MockPatronClient{}
			if tt.expectUpdate {
				mockPatron.On("UpdateUserExpiration",
					mock.Anything, mock.Anything,
					tt.user.ID, mock.Anything, tt.renewalConfig.ExtendExpired,
				).Return(nil)
			}

			h := NewBaseHandler(logger, tc)
			injectMocks(h, mockPatron, nil, nil, nil)

			h.attemptRollingRenewalWithTime(context.Background(), tt.user, "test-token", sess, tt.today)

			mockPatron.AssertExpectations(t)

			if tt.expectLogMessage != "" {
				found := false
				for _, log := range recorded.All() {
					if strings.Contains(log.Message, tt.expectLogMessage) {
						found = true
						assert.Equal(t, tt.expectLogLevel, log.Level,
							"unexpected log level for message %q", log.Message)
						assert.Equal(t, "application", log.ContextMap()["type"],
							"rolling renewal log must have type=application")
						break
					}
				}
				assert.True(t, found, "expected log message %q not found in:\n%s",
					tt.expectLogMessage, summariseLogs(recorded.All()))
			}
		})
	}
}

// TestAttemptRollingRenewal_PermissionError verifies that a permission error
// from UpdateUserExpiration is logged at WARN or ERROR level.
func TestAttemptRollingRenewal_PermissionError(t *testing.T) {
	core, recorded := observer.New(zap.DebugLevel)
	logger := zap.New(core)

	tc := testutil.NewTenantConfig(testutil.WithRollingRenewal(&config.RollingRenewalConfig{
		Enabled: true, RenewWithin: "6M", ExtendFor: "1Y",
		ExtendExpired: true, DryRun: false, SelectPatrons: false,
	}))
	sess := testutil.NewAuthSession(tc)

	user := &models.User{
		ID:             "user-1",
		Username:       "test",
		Barcode:        "123",
		PatronGroup:    "group-1",
		ExpirationDate: timePtr(time.Date(2025, 5, 1, 0, 0, 0, 0, time.UTC)),
		Personal:       models.PersonalInfo{LastName: "Test", FirstName: "User"},
	}

	permErr := &folio.PermissionError{
		Operation: "UpdateUserExpiration",
		UserID:    user.ID,
	}
	mockPatron := &MockPatronClient{}
	mockPatron.On("UpdateUserExpiration",
		mock.Anything, mock.Anything, user.ID, mock.Anything, true,
	).Return(permErr)

	h := NewBaseHandler(logger, tc)
	injectMocks(h, mockPatron, nil, nil, nil)

	h.attemptRollingRenewalWithTime(context.Background(), user, "test-token", sess,
		time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC))

	mockPatron.AssertExpectations(t)

	found := false
	for _, log := range recorded.All() {
		if strings.Contains(log.Message, "permission denied") || strings.Contains(log.Message, "failed") {
			found = true
			assert.True(t, log.Level >= zapcore.WarnLevel,
				"permission error should be WARN or ERROR, got %v", log.Level)
			assert.Equal(t, "application", log.ContextMap()["type"],
				"rolling renewal permission-error log must have type=application")
			break
		}
	}
	assert.True(t, found, "expected permission-error log not found:\n%s",
		summariseLogs(recorded.All()))
}

// TestAttemptRollingRenewal_NoExpirationDate verifies that a user with no
// expiration date is logged as not-eligible with a "no expiration date" reason.
func TestAttemptRollingRenewal_NoExpirationDate(t *testing.T) {
	core, recorded := observer.New(zap.DebugLevel)
	logger := zap.New(core)

	tc := testutil.NewTenantConfig(testutil.WithRollingRenewal(&config.RollingRenewalConfig{
		Enabled: true, RenewWithin: "6M", ExtendFor: "1Y",
		ExtendExpired: true, DryRun: false, SelectPatrons: false,
	}))
	sess := testutil.NewAuthSession(tc)

	user := &models.User{
		ID:             "user-1",
		Username:       "test",
		Barcode:        "123",
		PatronGroup:    "group-1",
		ExpirationDate: nil,
		Personal:       models.PersonalInfo{LastName: "Test", FirstName: "User"},
	}

	// No mock expectations — UpdateUserExpiration must not be called.
	mockPatron := &MockPatronClient{}

	h := NewBaseHandler(logger, tc)
	injectMocks(h, mockPatron, nil, nil, nil)

	h.attemptRollingRenewal(context.Background(), user, "test-token", sess)

	mockPatron.AssertExpectations(t)

	found := false
	for _, log := range recorded.All() {
		if strings.Contains(log.Message, "not eligible") {
			found = true
			assert.Equal(t, zapcore.DebugLevel, log.Level)
			assert.Equal(t, "application", log.ContextMap()["type"],
				"rolling renewal not-eligible log must have type=application")

			hasReason := false
			for _, f := range log.Context {
				if f.Key == "reason" && strings.Contains(f.String, "no expiration date") {
					hasReason = true
					break
				}
			}
			assert.True(t, hasReason, "expected reason='no expiration date' in log context")
			break
		}
	}
	assert.True(t, found, "expected 'not eligible' log not found:\n%s",
		summariseLogs(recorded.All()))
}

// ─── helpers ──────────────────────────────────────────────────────────────────

// timePtr returns a pointer to t.
func timePtr(t time.Time) *time.Time { return &t }

// summariseLogs formats observed log entries for test failure output.
func summariseLogs(logs []observer.LoggedEntry) string {
	var b strings.Builder
	for _, l := range logs {
		b.WriteString("  [" + l.Level.String() + "] " + l.Message + "\n")
	}
	return b.String()
}
