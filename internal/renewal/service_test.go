package renewal

import (
	"testing"
	"time"

	"github.com/spokanepubliclibrary/fsip2/internal/config"
	"github.com/spokanepubliclibrary/fsip2/internal/folio/models"
)

func TestShouldRenew(t *testing.T) {
	service := NewRollingRenewalService()
	today := time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name                 string
		user                 *models.User
		cfg                  *config.RollingRenewalConfig
		today                time.Time
		expectRenew          bool
		expectReasonContains string
	}{
		{
			name: "rolling renewals disabled",
			user: &models.User{
				ID:             "user-1",
				PatronGroup:    "group-1",
				ExpirationDate: timePtr(time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC)),
			},
			cfg: &config.RollingRenewalConfig{
				Enabled:     false,
				RenewWithin: "6M",
				ExtendFor:   "1Y",
			},
			today:                today,
			expectRenew:          false,
			expectReasonContains: "disabled",
		},
		{
			name: "user has no expiration date",
			user: &models.User{
				ID:             "user-1",
				PatronGroup:    "group-1",
				ExpirationDate: nil,
			},
			cfg: &config.RollingRenewalConfig{
				Enabled:     true,
				RenewWithin: "6M",
				ExtendFor:   "1Y",
			},
			today:                today,
			expectRenew:          false,
			expectReasonContains: "no expiration date",
		},
		{
			name: "patron group not in allowed list",
			user: &models.User{
				ID:             "user-1",
				PatronGroup:    "group-other",
				ExpirationDate: timePtr(time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC)),
			},
			cfg: &config.RollingRenewalConfig{
				Enabled:        true,
				RenewWithin:    "6M",
				ExtendFor:      "1Y",
				SelectPatrons:  true,
				AllowedPatrons: []string{"group-1", "group-2"},
			},
			today:                today,
			expectRenew:          false,
			expectReasonContains: "not in allowed list",
		},
		{
			name: "patron group in allowed list",
			user: &models.User{
				ID:             "user-1",
				PatronGroup:    "group-1",
				ExpirationDate: timePtr(time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC)),
			},
			cfg: &config.RollingRenewalConfig{
				Enabled:        true,
				RenewWithin:    "6M",
				ExtendFor:      "1Y",
				SelectPatrons:  true,
				AllowedPatrons: []string{"group-1", "group-2"},
			},
			today:                today,
			expectRenew:          true,
			expectReasonContains: "eligible",
		},
		{
			name: "account expired and extendExpired is false",
			user: &models.User{
				ID:             "user-1",
				PatronGroup:    "group-1",
				ExpirationDate: timePtr(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)), // Expired 14 days ago
			},
			cfg: &config.RollingRenewalConfig{
				Enabled:       true,
				RenewWithin:   "6M",
				ExtendFor:     "1Y",
				ExtendExpired: false,
			},
			today:                today,
			expectRenew:          false,
			expectReasonContains: "expired",
		},
		{
			name: "account expired but extendExpired is true",
			user: &models.User{
				ID:             "user-1",
				PatronGroup:    "group-1",
				ExpirationDate: timePtr(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)), // Expired 14 days ago
			},
			cfg: &config.RollingRenewalConfig{
				Enabled:       true,
				RenewWithin:   "6M",
				ExtendFor:     "1Y",
				ExtendExpired: true,
			},
			today:                today,
			expectRenew:          true,
			expectReasonContains: "eligible",
		},
		{
			name: "expiration within renewal window",
			user: &models.User{
				ID:             "user-1",
				PatronGroup:    "group-1",
				ExpirationDate: timePtr(time.Date(2025, 5, 1, 0, 0, 0, 0, time.UTC)), // ~3.5 months away
			},
			cfg: &config.RollingRenewalConfig{
				Enabled:     true,
				RenewWithin: "6M", // Within 6 months
				ExtendFor:   "1Y",
			},
			today:                today,
			expectRenew:          true,
			expectReasonContains: "eligible",
		},
		{
			name: "expiration outside renewal window",
			user: &models.User{
				ID:             "user-1",
				PatronGroup:    "group-1",
				ExpirationDate: timePtr(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)), // ~12 months away
			},
			cfg: &config.RollingRenewalConfig{
				Enabled:     true,
				RenewWithin: "6M", // Within 6 months
				ExtendFor:   "1Y",
			},
			today:                today,
			expectRenew:          false,
			expectReasonContains: "not within renewal window",
		},
		{
			name: "expiration on boundary of renewal window",
			user: &models.User{
				ID:             "user-1",
				PatronGroup:    "group-1",
				ExpirationDate: timePtr(time.Date(2025, 4, 15, 0, 0, 0, 0, time.UTC)), // Exactly 3M away
			},
			cfg: &config.RollingRenewalConfig{
				Enabled:     true,
				RenewWithin: "3M",
				ExtendFor:   "1Y",
			},
			today:                today,
			expectRenew:          true,
			expectReasonContains: "eligible",
		},
		{
			name: "expiration in past - same day as today",
			user: &models.User{
				ID:             "user-1",
				PatronGroup:    "group-1",
				ExpirationDate: timePtr(time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC)), // Today
			},
			cfg: &config.RollingRenewalConfig{
				Enabled:       true,
				RenewWithin:   "6M",
				ExtendFor:     "1Y",
				ExtendExpired: true,
			},
			today:                today,
			expectRenew:          true,
			expectReasonContains: "eligible",
		},
		{
			name: "selectPatrons false - all patrons allowed",
			user: &models.User{
				ID:             "user-1",
				PatronGroup:    "any-group",
				ExpirationDate: timePtr(time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC)),
			},
			cfg: &config.RollingRenewalConfig{
				Enabled:       true,
				RenewWithin:   "6M",
				ExtendFor:     "1Y",
				SelectPatrons: false, // Don't filter by patron group
			},
			today:                today,
			expectRenew:          true,
			expectReasonContains: "eligible",
		},
		{
			name: "renewWithin using days - within window",
			user: &models.User{
				ID:             "user-1",
				PatronGroup:    "group-1",
				ExpirationDate: timePtr(time.Date(2025, 1, 25, 0, 0, 0, 0, time.UTC)), // 10 days away
			},
			cfg: &config.RollingRenewalConfig{
				Enabled:     true,
				RenewWithin: "30D", // Within 30 days
				ExtendFor:   "1Y",
			},
			today:                today,
			expectRenew:          true,
			expectReasonContains: "eligible",
		},
		{
			name: "renewWithin using days - outside window",
			user: &models.User{
				ID:             "user-1",
				PatronGroup:    "group-1",
				ExpirationDate: timePtr(time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC)), // 45 days away
			},
			cfg: &config.RollingRenewalConfig{
				Enabled:     true,
				RenewWithin: "30D", // Within 30 days
				ExtendFor:   "1Y",
			},
			today:                today,
			expectRenew:          false,
			expectReasonContains: "not within renewal window",
		},
		{
			name: "renewWithin using years",
			user: &models.User{
				ID:             "user-1",
				PatronGroup:    "group-1",
				ExpirationDate: timePtr(time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)), // ~5 months away
			},
			cfg: &config.RollingRenewalConfig{
				Enabled:     true,
				RenewWithin: "1Y", // Within 1 year
				ExtendFor:   "2Y",
			},
			today:                today,
			expectRenew:          true,
			expectReasonContains: "eligible",
		},
		{
			name: "extendExpiredLimits - expired within limit (3 months ago, limit 6M)",
			user: &models.User{
				ID:             "user-1",
				PatronGroup:    "group-1",
				ExpirationDate: timePtr(time.Date(2024, 10, 15, 0, 0, 0, 0, time.UTC)), // 3 months ago
			},
			cfg: &config.RollingRenewalConfig{
				Enabled:             true,
				RenewWithin:         "6M",
				ExtendFor:           "1Y",
				ExtendExpired:       true,
				ExtendExpiredLimits: "6M", // Allow renewal if expired less than 6 months ago
			},
			today:                today,
			expectRenew:          true,
			expectReasonContains: "eligible",
		},
		{
			name: "extendExpiredLimits - expired beyond limit (8 months ago, limit 6M)",
			user: &models.User{
				ID:             "user-1",
				PatronGroup:    "group-1",
				ExpirationDate: timePtr(time.Date(2024, 5, 15, 0, 0, 0, 0, time.UTC)), // 8 months ago
			},
			cfg: &config.RollingRenewalConfig{
				Enabled:             true,
				RenewWithin:         "12M",
				ExtendFor:           "1Y",
				ExtendExpired:       true,
				ExtendExpiredLimits: "6M", // Only allow renewal if expired less than 6 months ago
			},
			today:                today,
			expectRenew:          false,
			expectReasonContains: "expired more than 6M ago",
		},
		{
			name: "extendExpiredLimits - exactly at limit boundary",
			user: &models.User{
				ID:             "user-1",
				PatronGroup:    "group-1",
				ExpirationDate: timePtr(time.Date(2024, 7, 15, 0, 0, 0, 0, time.UTC)), // Exactly 6 months ago
			},
			cfg: &config.RollingRenewalConfig{
				Enabled:             true,
				RenewWithin:         "12M",
				ExtendFor:           "1Y",
				ExtendExpired:       true,
				ExtendExpiredLimits: "6M",
			},
			today:                today,
			expectRenew:          true, // Exactly at boundary should be allowed
			expectReasonContains: "eligible",
		},
		{
			name: "extendExpiredLimits - not expired, limit ignored",
			user: &models.User{
				ID:             "user-1",
				PatronGroup:    "group-1",
				ExpirationDate: timePtr(time.Date(2025, 3, 15, 0, 0, 0, 0, time.UTC)), // 2 months in future
			},
			cfg: &config.RollingRenewalConfig{
				Enabled:             true,
				RenewWithin:         "6M",
				ExtendFor:           "1Y",
				ExtendExpired:       false,
				ExtendExpiredLimits: "6M", // Limit only applies to expired accounts
			},
			today:                today,
			expectRenew:          true,
			expectReasonContains: "eligible",
		},
		{
			name: "extendExpiredLimits - no limit set (blank)",
			user: &models.User{
				ID:             "user-1",
				PatronGroup:    "group-1",
				ExpirationDate: timePtr(time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)), // 2 years ago
			},
			cfg: &config.RollingRenewalConfig{
				Enabled:             true,
				RenewWithin:         "3Y",
				ExtendFor:           "1Y",
				ExtendExpired:       true,
				ExtendExpiredLimits: "", // No limit - allow any expired account
			},
			today:                today,
			expectRenew:          true,
			expectReasonContains: "eligible",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decision := service.ShouldRenew(tt.user, tt.cfg, tt.today)

			if decision.ShouldRenew != tt.expectRenew {
				t.Errorf("ShouldRenew() = %v, want %v (reason: %s)", decision.ShouldRenew, tt.expectRenew, decision.Reason)
			}

			if tt.expectReasonContains != "" {
				if !contains(decision.Reason, tt.expectReasonContains) {
					t.Errorf("Reason = %q, expected to contain %q", decision.Reason, tt.expectReasonContains)
				}
			}
		})
	}
}

func TestCalculateNewExpiration(t *testing.T) {
	service := NewRollingRenewalService()
	today := time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name        string
		today       time.Time
		cfg         *config.RollingRenewalConfig
		expectDate  time.Time
		expectError bool
	}{
		{
			name:  "extend by 1 year",
			today: today,
			cfg: &config.RollingRenewalConfig{
				ExtendFor: "1Y",
			},
			expectDate:  time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC),
			expectError: false,
		},
		{
			name:  "extend by 6 months",
			today: today,
			cfg: &config.RollingRenewalConfig{
				ExtendFor: "6M",
			},
			expectDate:  time.Date(2025, 7, 15, 0, 0, 0, 0, time.UTC),
			expectError: false,
		},
		{
			name:  "extend by 30 days",
			today: today,
			cfg: &config.RollingRenewalConfig{
				ExtendFor: "30D",
			},
			expectDate:  time.Date(2025, 2, 14, 0, 0, 0, 0, time.UTC),
			expectError: false,
		},
		{
			name:  "extend by 2 years",
			today: today,
			cfg: &config.RollingRenewalConfig{
				ExtendFor: "2Y",
			},
			expectDate:  time.Date(2027, 1, 15, 0, 0, 0, 0, time.UTC),
			expectError: false,
		},
		{
			name:  "invalid duration format",
			today: today,
			cfg: &config.RollingRenewalConfig{
				ExtendFor: "invalid",
			},
			expectError: true,
		},
		{
			name:  "empty duration",
			today: today,
			cfg: &config.RollingRenewalConfig{
				ExtendFor: "",
			},
			expectError: true,
		},
		{
			name:  "end of month overflow - Jan 31 + 1M",
			today: time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC),
			cfg: &config.RollingRenewalConfig{
				ExtendFor: "1M",
			},
			expectDate:  time.Date(2025, 3, 3, 0, 0, 0, 0, time.UTC), // Feb has 28 days in 2025
			expectError: false,
		},
		{
			name:  "leap year - Feb 29 + 1Y",
			today: time.Date(2024, 2, 29, 0, 0, 0, 0, time.UTC),
			cfg: &config.RollingRenewalConfig{
				ExtendFor: "1Y",
			},
			expectDate:  time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC), // Feb 29 doesn't exist in 2025
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := service.CalculateNewExpiration(tt.today, tt.cfg)

			if tt.expectError {
				if err == nil {
					t.Errorf("CalculateNewExpiration() expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("CalculateNewExpiration() unexpected error: %v", err)
				}

				if !result.Equal(tt.expectDate) {
					t.Errorf("CalculateNewExpiration() = %v, want %v", result, tt.expectDate)
				}
			}
		})
	}
}

// Test various patron scenarios with complete renewal flow
func TestPatronScenarios(t *testing.T) {
	service := NewRollingRenewalService()
	today := time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC)

	scenarios := []struct {
		name            string
		user            *models.User
		cfg             *config.RollingRenewalConfig
		shouldRenew     bool
		expectedNewDate time.Time
	}{
		{
			name: "standard renewal - undergraduate within 6 months",
			user: &models.User{
				ID:             "user-1",
				Username:       "jdoe",
				PatronGroup:    "undergrad-group-id",
				ExpirationDate: timePtr(time.Date(2025, 5, 1, 0, 0, 0, 0, time.UTC)),
			},
			cfg: &config.RollingRenewalConfig{
				Enabled:       true,
				RenewWithin:   "6M",
				ExtendFor:     "1Y",
				ExtendExpired: false,
				SelectPatrons: false,
			},
			shouldRenew:     true,
			expectedNewDate: time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC),
		},
		{
			name: "expired faculty with extendExpired enabled",
			user: &models.User{
				ID:             "user-2",
				Username:       "professor",
				PatronGroup:    "faculty-group-id",
				ExpirationDate: timePtr(time.Date(2024, 12, 1, 0, 0, 0, 0, time.UTC)), // Expired
			},
			cfg: &config.RollingRenewalConfig{
				Enabled:       true,
				RenewWithin:   "6M",
				ExtendFor:     "2Y",
				ExtendExpired: true,
				SelectPatrons: false,
			},
			shouldRenew:     true,
			expectedNewDate: time.Date(2027, 1, 15, 0, 0, 0, 0, time.UTC), // 2 years from today
		},
		{
			name: "staff member - selective renewal",
			user: &models.User{
				ID:             "user-3",
				Username:       "staff001",
				PatronGroup:    "staff-group-id",
				ExpirationDate: timePtr(time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC)),
			},
			cfg: &config.RollingRenewalConfig{
				Enabled:        true,
				RenewWithin:    "6M",
				ExtendFor:      "1Y",
				ExtendExpired:  false,
				SelectPatrons:  true,
				AllowedPatrons: []string{"staff-group-id", "faculty-group-id"},
			},
			shouldRenew:     true,
			expectedNewDate: time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC),
		},
		{
			name: "guest - not in allowed patrons",
			user: &models.User{
				ID:             "user-4",
				Username:       "guest123",
				PatronGroup:    "guest-group-id",
				ExpirationDate: timePtr(time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC)),
			},
			cfg: &config.RollingRenewalConfig{
				Enabled:        true,
				RenewWithin:    "6M",
				ExtendFor:      "1Y",
				ExtendExpired:  false,
				SelectPatrons:  true,
				AllowedPatrons: []string{"staff-group-id", "faculty-group-id"},
			},
			shouldRenew: false,
		},
		{
			name: "community user - expiration too far in future",
			user: &models.User{
				ID:             "user-5",
				Username:       "community",
				PatronGroup:    "community-group-id",
				ExpirationDate: timePtr(time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)), // 17 months away
			},
			cfg: &config.RollingRenewalConfig{
				Enabled:       true,
				RenewWithin:   "6M",
				ExtendFor:     "1Y",
				ExtendExpired: false,
				SelectPatrons: false,
			},
			shouldRenew: false,
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			// Check if should renew
			decision := service.ShouldRenew(scenario.user, scenario.cfg, today)

			if decision.ShouldRenew != scenario.shouldRenew {
				t.Errorf("ShouldRenew() = %v, want %v (reason: %s)",
					decision.ShouldRenew, scenario.shouldRenew, decision.Reason)
			}

			// If should renew, calculate new expiration
			if scenario.shouldRenew {
				newDate, err := service.CalculateNewExpiration(today, scenario.cfg)
				if err != nil {
					t.Errorf("CalculateNewExpiration() unexpected error: %v", err)
				}

				if !newDate.Equal(scenario.expectedNewDate) {
					t.Errorf("CalculateNewExpiration() = %v, want %v", newDate, scenario.expectedNewDate)
				}
			}
		})
	}
}

// Helper functions

func timePtr(t time.Time) *time.Time {
	return &t
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
