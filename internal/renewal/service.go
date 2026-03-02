package renewal

import (
	"fmt"
	"time"

	"github.com/spokanepubliclibrary/fsip2/internal/config"
	"github.com/spokanepubliclibrary/fsip2/internal/folio/models"
)

// RenewalDecision represents the result of a renewal eligibility check
type RenewalDecision struct {
	ShouldRenew bool
	Reason      string
}

// RollingRenewalService handles automatic patron account expiration renewals
type RollingRenewalService struct{}

// NewRollingRenewalService creates a new rolling renewal service
func NewRollingRenewalService() *RollingRenewalService {
	return &RollingRenewalService{}
}

// ShouldRenew determines if a patron account should be renewed based on configuration
// Returns a RenewalDecision with the decision and reason
func (s *RollingRenewalService) ShouldRenew(user *models.User, cfg *config.RollingRenewalConfig, today time.Time) RenewalDecision {
	// Check if rolling renewals are enabled
	if !cfg.Enabled {
		return RenewalDecision{
			ShouldRenew: false,
			Reason:      "rolling renewals disabled",
		}
	}

	// Check if user has an expiration date
	if user.ExpirationDate == nil {
		return RenewalDecision{
			ShouldRenew: false,
			Reason:      "user has no expiration date",
		}
	}

	// Check selectPatrons filtering
	if cfg.SelectPatrons {
		allowed := false
		for _, patronGroupID := range cfg.AllowedPatrons {
			if user.PatronGroup == patronGroupID {
				allowed = true
				break
			}
		}
		if !allowed {
			return RenewalDecision{
				ShouldRenew: false,
				Reason:      "patron group not in allowed list",
			}
		}
	}

	// Check if account is expired
	isExpired := config.IsExpired(*user.ExpirationDate, today)
	if isExpired && !cfg.ExtendExpired {
		return RenewalDecision{
			ShouldRenew: false,
			Reason:      "account expired and extendExpired is false",
		}
	}

	// If account is expired and extendExpiredLimits is set, check if expiration is beyond the limit
	if isExpired && cfg.ExtendExpiredLimits != "" {
		// Parse the limit duration
		limitDuration, err := config.ParseDuration(cfg.ExtendExpiredLimits)
		if err != nil {
			return RenewalDecision{
				ShouldRenew: false,
				Reason:      fmt.Sprintf("error parsing extendExpiredLimits: %v", err),
			}
		}

		// Calculate the cutoff date (today - limit)
		cutoffDate, err := config.SubtractDuration(today, limitDuration.Value, limitDuration.Period)
		if err != nil {
			return RenewalDecision{
				ShouldRenew: false,
				Reason:      fmt.Sprintf("error calculating extendExpiredLimits cutoff: %v", err),
			}
		}

		// If expiration is before the cutoff date, it's been expired too long
		if user.ExpirationDate.Before(cutoffDate) {
			return RenewalDecision{
				ShouldRenew: false,
				Reason:      fmt.Sprintf("account expired more than %s ago (expired: %s, limit: %s ago)", cfg.ExtendExpiredLimits, user.ExpirationDate.Format("2006-01-02"), cutoffDate.Format("2006-01-02")),
			}
		}
	}

	// Check if expiration is within renewal window
	withinWindow, err := config.IsWithinPeriod(*user.ExpirationDate, today, cfg.RenewWithin)
	if err != nil {
		return RenewalDecision{
			ShouldRenew: false,
			Reason:      fmt.Sprintf("error checking renewal window: %v", err),
		}
	}

	if !withinWindow {
		return RenewalDecision{
			ShouldRenew: false,
			Reason:      "expiration date not within renewal window",
		}
	}

	// All checks passed - should renew
	return RenewalDecision{
		ShouldRenew: true,
		Reason:      "eligible for renewal",
	}
}

// CalculateNewExpiration calculates the new expiration date based on configuration
// Returns the new expiration date calculated from today + extendFor period
func (s *RollingRenewalService) CalculateNewExpiration(today time.Time, cfg *config.RollingRenewalConfig) (time.Time, error) {
	// Parse the extendFor duration
	duration, err := config.ParseDuration(cfg.ExtendFor)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to parse extendFor duration: %w", err)
	}

	// Add the duration to today's date
	newExpiration, err := config.AddDuration(today, duration.Value, duration.Period)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to calculate new expiration: %w", err)
	}

	return newExpiration, nil
}
