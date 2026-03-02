package handlers

import (
	"context"
	"errors"

	"github.com/spokanepubliclibrary/fsip2/internal/types"
	"go.uber.org/zap"
)

var (
	// ErrVerificationRequired indicates that patron verification is required but password was not provided
	ErrVerificationRequired = errors.New("patron verification required but password not provided")

	// ErrVerificationFailed indicates that patron credentials failed verification
	ErrVerificationFailed = errors.New("patron verification failed")
)

// VerificationResult contains the result of patron credential verification
type VerificationResult struct {
	// Verified indicates whether the credentials were successfully verified
	Verified bool

	// Required indicates whether verification was required by config
	Required bool

	// Error contains any error that occurred during verification
	Error error
}

// VerifyPatronCredentials performs patron password/PIN verification based on session configuration.
// It handles both PIN verification (using patron-pin/verify endpoint) and password verification
// (using authn/login-with-expiry endpoint) based on the UsePinForPatronVerification config flag.
//
// Parameters:
//   - ctx: Context for the request
//   - logger: Logger for recording verification attempts and results
//   - session: Session containing tenant configuration
//   - patronClient: Client for making patron-related API calls
//   - token: Authentication token for API calls
//   - userID: FOLIO user ID (required for PIN verification)
//   - patronIdentifier: Patron identifier (barcode, required for password verification)
//   - patronPassword: The password or PIN to verify
//
// Returns:
//   - VerificationResult containing verification status and any errors
//
// Configuration behavior:
//   - If PatronPasswordVerificationRequired is false, verification is skipped (returns Verified=true, Required=false)
//   - If PatronPasswordVerificationRequired is true but patronPassword is empty, returns error (Required=true, Verified=false)
//   - If PatronPasswordVerificationRequired is true and password provided:
//   - If UsePinForPatronVerification is true: uses PIN verification endpoint
//   - If UsePinForPatronVerification is false: uses password login endpoint
func VerifyPatronCredentials(
	ctx context.Context,
	logger *zap.Logger,
	session *types.Session,
	patronClient PatronLookup,
	token string,
	userID string,
	patronIdentifier string,
	patronPassword string,
) VerificationResult {
	// If verification is not required, skip it
	if !session.TenantConfig.PatronPasswordVerificationRequired {
		logger.Debug("Patron password verification not required by config")
		return VerificationResult{
			Verified: true,
			Required: false,
			Error:    nil,
		}
	}

	// If verification is required but no password provided, fail
	if patronPassword == "" {
		logger.Warn("Patron password verification required but no password provided",
			zap.String("patron_identifier", patronIdentifier),
		)
		return VerificationResult{
			Verified: false,
			Required: true,
			Error:    ErrVerificationRequired,
		}
	}

	// Perform verification based on configuration
	if session.TenantConfig.UsePinForPatronVerification {
		// Use PIN verification (patron-pin/verify endpoint)
		logger.Info("Verifying patron PIN",
			zap.String("patron_id", userID),
			zap.String("patron_identifier", patronIdentifier),
		)

		verified, err := patronClient.VerifyPatronPin(ctx, token, userID, patronPassword)
		if err != nil {
			logger.Error("Failed to verify patron PIN",
				zap.String("patron_id", userID),
				zap.String("patron_identifier", patronIdentifier),
				zap.Error(err),
			)
			return VerificationResult{
				Verified: false,
				Required: true,
				Error:    err,
			}
		}

		if !verified {
			logger.Warn("Invalid patron PIN",
				zap.String("patron_id", userID),
				zap.String("patron_identifier", patronIdentifier),
			)
			return VerificationResult{
				Verified: false,
				Required: true,
				Error:    ErrVerificationFailed,
			}
		}

		logger.Info("Patron PIN verified successfully",
			zap.String("patron_id", userID),
			zap.String("patron_identifier", patronIdentifier),
		)
		return VerificationResult{
			Verified: true,
			Required: true,
			Error:    nil,
		}
	} else {
		// Use password verification (authn/login-with-expiry endpoint)
		logger.Info("Verifying patron credentials with login",
			zap.String("patron_identifier", patronIdentifier),
		)

		verified, err := patronClient.VerifyPatronPasswordWithLogin(ctx, patronIdentifier, patronPassword)
		if err != nil {
			logger.Error("Failed to verify patron credentials",
				zap.String("patron_identifier", patronIdentifier),
				zap.Error(err),
			)
			return VerificationResult{
				Verified: false,
				Required: true,
				Error:    err,
			}
		}

		if !verified {
			logger.Warn("Invalid patron credentials",
				zap.String("patron_identifier", patronIdentifier),
			)
			return VerificationResult{
				Verified: false,
				Required: true,
				Error:    ErrVerificationFailed,
			}
		}

		logger.Info("Patron credentials verified successfully",
			zap.String("patron_identifier", patronIdentifier),
		)
		return VerificationResult{
			Verified: true,
			Required: true,
			Error:    nil,
		}
	}
}

// GetVerificationErrorMessage returns the standard error message to display
// when patron verification fails. This message is used in the AF (screen message)
// field of SIP2 responses.
func GetVerificationErrorMessage() string {
	return "Your library card number cannot be located. Please see a staff member for assistance."
}
