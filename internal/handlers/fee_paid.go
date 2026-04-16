package handlers

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/spokanepubliclibrary/fsip2/internal/config"
	"github.com/spokanepubliclibrary/fsip2/internal/folio/models"
	"github.com/spokanepubliclibrary/fsip2/internal/logging"
	"github.com/spokanepubliclibrary/fsip2/internal/sip2/builder"
	"github.com/spokanepubliclibrary/fsip2/internal/sip2/parser"
	"github.com/spokanepubliclibrary/fsip2/internal/sip2/protocol"
	"github.com/spokanepubliclibrary/fsip2/internal/types"
	"go.uber.org/zap"
)

// FeePaidHandler handles SIP2 Fee Paid requests (37)
type FeePaidHandler struct {
	*BaseHandler
	logger *zap.Logger
}

// NewFeePaidHandler creates a new fee paid handler
func NewFeePaidHandler(logger *zap.Logger, tenantConfig *config.TenantConfig) *FeePaidHandler {
	return &FeePaidHandler{
		BaseHandler: NewBaseHandler(logger, tenantConfig),
		logger:      logger.With(logging.TypeField(logging.TypeApplication)),
	}
}

// paymentResult tracks the result of a single account payment
type paymentResult struct {
	account         *models.Account         // Original account details (before payment)
	paymentResponse *models.PaymentResponse // Payment API response
	amountApplied   float64
	success         bool
	error           error
}

// Handle processes a Fee Paid request (37) and returns a Fee Paid response (38)
func (h *FeePaidHandler) Handle(ctx context.Context, msg *parser.Message, session *types.Session) (string, error) {
	h.logRequest(msg, session)

	// Validate required fields
	// Note: Currency type is in the fixed-length section, not a variable field (BY)
	if err := h.validateRequiredFields(msg, map[parser.FieldCode]string{
		parser.PatronIdentifier: "Patron Identifier",
		parser.FeeAmount:        "Fee Amount",
	}); err != nil {
		h.logger.Error("Fee paid validation failed", zap.Error(err))
		return h.buildErrorResponse(msg.GetField(parser.InstitutionID), msg.GetField(parser.PatronIdentifier), "Validation failed", msg, session), nil
	}

	// Extract fields from message
	institutionID := msg.GetField(parser.InstitutionID)
	patronIdentifier := msg.GetField(parser.PatronIdentifier)
	patronPassword := msg.GetField(parser.PatronPassword)
	feeAmountStr := msg.GetField(parser.FeeAmount)
	accountID := msg.GetField(parser.FeeIdentifier) // CG field - account ID

	// Get configuration values
	servicePointID := session.GetLocationCode() // CP field contains service point UUID
	username := session.GetUsername()           // CN field from login
	paymentMethod := session.TenantConfig.GetPaymentMethod()
	notifyPatron := session.TenantConfig.GetNotifyPatron()
	acceptBulkPayment := session.TenantConfig.GetAcceptBulkPayment()

	h.logger.Info("Fee paid request",
		zap.String("institution_id", institutionID),
		zap.String("patron_identifier", patronIdentifier),
		zap.String("fee_amount", feeAmountStr),
		zap.String("account_id", accountID),
		zap.String("service_point_id", servicePointID),
		zap.String("payment_method", paymentMethod),
		zap.Bool("notify_patron", notifyPatron),
		zap.Bool("accept_bulk_payment", acceptBulkPayment),
	)

	// Parse fee amount
	feeAmount, err := strconv.ParseFloat(feeAmountStr, 64)
	if err != nil {
		h.logger.Error("Invalid fee amount",
			zap.String("fee_amount", feeAmountStr),
			zap.Error(err),
		)
		return h.buildErrorResponse(institutionID, patronIdentifier, "Invalid fee amount", msg, session), nil
	}

	if feeAmount <= 0 {
		h.logger.Error("Fee amount must be positive", zap.Float64("fee_amount", feeAmount))
		return h.buildErrorResponse(institutionID, patronIdentifier, "Invalid fee amount", msg, session), nil
	}

	// Get authenticated FOLIO client
	_, token, err := h.getAuthenticatedFolioClient(ctx, session)
	if err != nil {
		h.logger.Error("Failed to get authenticated client", zap.Error(err))
		return h.buildErrorResponse(institutionID, patronIdentifier, "Authentication failed", msg, session), nil
	}

	// Get patron ID and user info (from session or lookup)
	patronID := session.GetPatronID()
	if patronID != "" && session.GetPatronBarcode() != patronIdentifier {
		h.logger.Info("Patron barcode mismatch — invalidating cached patron ID",
			zap.String("cached_barcode", session.GetPatronBarcode()),
			zap.String("request_barcode", patronIdentifier),
		)
		patronID = ""
	}
	var userID string

	// Create patron client for lookup or verification
	patronClient := h.getPatronClient(session)

	if patronID == "" {
		// Look up patron by barcode
		user, err := patronClient.GetUserByBarcode(ctx, token, patronIdentifier)
		if err != nil {
			h.logger.Error("Failed to get patron",
				zap.String("patron_identifier", patronIdentifier),
				zap.Error(err),
			)
			return h.buildErrorResponse(institutionID, patronIdentifier, GetVerificationErrorMessage(), msg, session), nil
		}
		patronID = user.ID
		userID = user.ID
		session.SetPatronBarcode(patronIdentifier)
		session.SetPatronID(user.ID)
	} else {
		userID = patronID
	}

	// Verify patron password/PIN if required
	verifyResult := VerifyPatronCredentials(
		ctx,
		h.logger,
		session,
		patronClient,
		token,
		userID,
		patronIdentifier,
		patronPassword,
	)

	if verifyResult.Required && !verifyResult.Verified {
		h.logger.Info("Payment blocked due to failed patron verification",
			zap.String("patron_identifier", patronIdentifier),
		)
		return h.buildErrorResponse(institutionID, patronIdentifier, GetVerificationErrorMessage(), msg, session), nil
	}

	// Create fees/fines client
	feesClient := h.getFeesClient(session)

	// Try single account payment if account ID (CG) is provided
	var results []paymentResult
	var isBulkPayment bool

	if accountID != "" {
		// Attempt single account payment
		result, shouldFallback := h.paySingleAccount(ctx, feesClient, token, accountID, feeAmount, servicePointID, username, paymentMethod, notifyPatron)

		if result.success {
			// Single payment succeeded
			results = append(results, result)
			h.logger.Info("Single account payment successful",
				zap.String("account_id", accountID),
				zap.Float64("amount", feeAmount),
			)
		} else if shouldFallback && acceptBulkPayment {
			// Account not found or not eligible - fall back to bulk payment
			h.logger.Info("Account not eligible, falling back to bulk payment",
				zap.String("account_id", accountID),
				zap.Error(result.error),
			)
			results = h.payBulkAccounts(ctx, feesClient, token, patronID, feeAmount, servicePointID, username, paymentMethod, notifyPatron)
			isBulkPayment = true
		} else {
			// Single payment failed and no fallback
			h.logger.Error("Single account payment failed",
				zap.String("account_id", accountID),
				zap.Error(result.error),
			)
			return h.buildErrorResponse(institutionID, patronIdentifier, "No fee/fine could be found by ID", msg, session), nil
		}
	} else {
		// No account ID provided - use bulk payment if enabled
		if acceptBulkPayment {
			h.logger.Info("No account ID provided, using bulk payment", zap.String("patron_id", patronID))
			results = h.payBulkAccounts(ctx, feesClient, token, patronID, feeAmount, servicePointID, username, paymentMethod, notifyPatron)
			isBulkPayment = true
		} else {
			h.logger.Error("No account ID provided and bulk payment disabled")
			return h.buildErrorResponse(institutionID, patronIdentifier, "Account ID required", msg, session), nil
		}
	}

	// Check if we have any successful payments
	successCount := 0
	for _, r := range results {
		if r.success {
			successCount++
		}
	}

	if successCount == 0 {
		h.logger.Error("All payment attempts failed", zap.Int("attempted", len(results)))
		return h.buildErrorResponse(institutionID, patronIdentifier, "Payment failed - see staff for details", msg, session), nil
	}

	h.logResponse(string(parser.FeePaidResponse), session, nil)

	// Build success response with payment details
	return h.buildSuccessResponse(institutionID, patronIdentifier, results, isBulkPayment, msg, session), nil
}

// paySingleAccount attempts to pay a single account
// Returns the payment result and a boolean indicating if fallback to bulk payment should be attempted
func (h *FeePaidHandler) paySingleAccount(
	ctx context.Context,
	feesClient FeesOps,
	token string,
	accountID string,
	amount float64,
	servicePointID string,
	username string,
	paymentMethod string,
	notifyPatron bool,
) (paymentResult, bool) {
	// Check if account exists and is eligible for payment
	account, err := feesClient.GetEligibleAccountByID(ctx, token, accountID)
	if err != nil {
		h.logger.Error("Error checking account eligibility",
			zap.String("account_id", accountID),
			zap.Error(err),
		)
		return paymentResult{success: false, error: err}, false
	}

	if account == nil {
		// Account not found or not eligible - allow fallback to bulk payment
		h.logger.Warn("Account not found or not eligible",
			zap.String("account_id", accountID),
		)
		return paymentResult{success: false, error: fmt.Errorf("account not eligible")}, true
	}

	// Build payment request
	paymentReq := &models.PaymentRequest{
		Amount:         amount,
		NotifyPatron:   notifyPatron,
		ServicePointID: servicePointID,
		UserName:       username,
		PaymentMethod:  paymentMethod,
	}

	// Execute payment
	paymentResp, err := feesClient.PayAccount(ctx, token, accountID, paymentReq)
	if err != nil {
		h.logger.Error("Failed to pay account",
			zap.String("account_id", accountID),
			zap.Float64("amount", amount),
			zap.Error(err),
		)
		return paymentResult{success: false, error: err}, false
	}

	return paymentResult{
		account:         account,
		paymentResponse: paymentResp,
		amountApplied:   amount,
		success:         true,
	}, false
}

// payBulkAccounts distributes payment across all eligible open accounts
func (h *FeePaidHandler) payBulkAccounts(
	ctx context.Context,
	feesClient FeesOps,
	token string,
	userID string,
	totalAmount float64,
	servicePointID string,
	username string,
	paymentMethod string,
	notifyPatron bool,
) []paymentResult {
	var results []paymentResult

	// Get all eligible open accounts excluding suspended claim returned
	accounts, err := feesClient.GetOpenAccountsExcludingSuspended(ctx, token, userID)
	if err != nil {
		h.logger.Error("Failed to get open accounts",
			zap.String("user_id", userID),
			zap.Error(err),
		)
		return results
	}

	if len(accounts.Accounts) == 0 {
		h.logger.Warn("No eligible open accounts found", zap.String("user_id", userID))
		return results
	}

	// Calculate payment distribution
	// Split evenly with remainder going to last account
	accountCount := len(accounts.Accounts)
	baseAmount := math.Floor(totalAmount/float64(accountCount)*100) / 100
	totalDistributed := baseAmount * float64(accountCount)
	remainder := totalAmount - totalDistributed

	h.logger.Info("Distributing bulk payment",
		zap.Int("account_count", accountCount),
		zap.Float64("total_amount", totalAmount),
		zap.Float64("base_amount", baseAmount),
		zap.Float64("remainder", remainder),
	)

	// Process payments for each account
	// Continue processing even if some fail (partial success handling)
	for i, account := range accounts.Accounts {
		// Calculate amount for this account (last account gets remainder)
		amountForAccount := baseAmount
		if i == accountCount-1 {
			amountForAccount += remainder
		}

		// Build payment request
		paymentReq := &models.PaymentRequest{
			Amount:         amountForAccount,
			NotifyPatron:   notifyPatron,
			ServicePointID: servicePointID,
			UserName:       username,
			PaymentMethod:  paymentMethod,
		}

		// Execute payment
		paymentResp, err := feesClient.PayAccount(ctx, token, account.ID, paymentReq)
		if err != nil {
			// Log error but continue processing other accounts
			h.logger.Error("Failed to pay account in bulk payment",
				zap.String("account_id", account.ID),
				zap.Float64("amount", amountForAccount),
				zap.Error(err),
			)
			results = append(results, paymentResult{
				account:       &account,
				amountApplied: amountForAccount,
				success:       false,
				error:         err,
			})
			continue
		}

		h.logger.Info("Bulk payment applied to account",
			zap.String("account_id", account.ID),
			zap.Float64("amount", amountForAccount),
			zap.String("remaining", paymentResp.RemainingAmount),
		)

		results = append(results, paymentResult{
			account:         &account,
			paymentResponse: paymentResp,
			amountApplied:   amountForAccount,
			success:         true,
		})
	}

	return results
}

// buildErrorResponse builds an error Fee Paid Response (38)
func (h *FeePaidHandler) buildErrorResponse(institutionID, patronIdentifier, message string, msg *parser.Message, session *types.Session) string {
	timestamp := protocol.FormatSIP2DateTime(time.Now(), "    ")

	response := fmt.Sprintf("38N%s", timestamp)
	response += fmt.Sprintf("|AO%s", institutionID)
	response += fmt.Sprintf("|AA%s", patronIdentifier)

	// Add transaction ID if provided
	transactionID := msg.GetField(parser.TransactionID)
	if transactionID != "" {
		response += fmt.Sprintf("|BK%s", transactionID)
	}

	response += fmt.Sprintf("|AF%s", message)

	// Use ResponseBuilder to add AY (sequence number) and AZ (checksum) if error detection is enabled
	sessionBuilder := h.builder
	if session != nil && session.TenantConfig != nil {
		sessionBuilder = builder.NewResponseBuilder(session.TenantConfig)
	}

	// Remove the "38" prefix from response as the builder will add it
	content := response[2:]

	// Use builder to add sequence number, checksum, and delimiter
	finalResponse, err := sessionBuilder.Build(parser.FeePaidResponse, content, msg.SequenceNumber)
	if err != nil {
		h.logger.Error("Failed to build fee paid error response", zap.Error(err))
		// Fallback to simple response without checksum
		return response
	}

	return finalResponse
}

// buildSuccessResponse builds a successful Fee Paid Response (38) with payment details
func (h *FeePaidHandler) buildSuccessResponse(
	institutionID string,
	patronIdentifier string,
	results []paymentResult,
	isBulkPayment bool,
	msg *parser.Message,
	session *types.Session,
) string {
	timestamp := protocol.FormatSIP2DateTime(time.Now(), "    ")

	// Fee Paid Response format:
	// 38<payment_accepted><transaction_date>|AO|AA|CG|FA|FC|FE|FG|AF|AY|AZ
	response := fmt.Sprintf("38Y%s", timestamp)
	response += fmt.Sprintf("|AO%s", institutionID)
	response += fmt.Sprintf("|AA%s", patronIdentifier)

	// Add repeatable fields for each successful payment
	successCount := 0
	failureCount := 0
	for _, result := range results {
		if result.success && result.account != nil && result.paymentResponse != nil {
			// CG = Account ID payment was applied to
			response += fmt.Sprintf("|CG%s", result.account.ID)

			// FA = Remaining balance of the account (from payment response)
			response += fmt.Sprintf("|FA%s", result.paymentResponse.RemainingAmount)

			// FC = Payment date in format YYYYMMDD    HHMMSS (4 spaces)
			paymentDate := protocol.FormatSIP2DateTime(time.Now(), "    ")
			response += fmt.Sprintf("|FC%s", paymentDate)

			// FE = Fee/fine identifier (feeFineId)
			response += fmt.Sprintf("|FE%s", result.account.FeeFineID)

			// FG = Amount applied to account
			response += fmt.Sprintf("|FG%.2f", result.amountApplied)

			successCount++
		} else {
			failureCount++
		}
	}

	// Add transaction ID if provided
	transactionID := msg.GetField(parser.TransactionID)
	if transactionID != "" {
		response += fmt.Sprintf("|BK%s", transactionID)
	}

	// Add screen message (AF field)
	if isBulkPayment {
		if failureCount > 0 {
			// Partial failure in bulk payment
			response += "|AFBulk payment applied - see staff for details"
		} else {
			// All bulk payments succeeded
			response += "|AFBulk payment applied"
		}
	} else {
		// Single payment
		response += "|AFPayment accepted"
	}

	// Use ResponseBuilder to add AY (sequence number) and AZ (checksum) if error detection is enabled
	sessionBuilder := h.builder
	if session != nil && session.TenantConfig != nil {
		sessionBuilder = builder.NewResponseBuilder(session.TenantConfig)
	}

	// Remove the "38" prefix from response as the builder will add it
	content := response[2:]

	// Use builder to add sequence number, checksum, and delimiter
	finalResponse, err := sessionBuilder.Build(parser.FeePaidResponse, content, msg.SequenceNumber)
	if err != nil {
		h.logger.Error("Failed to build fee paid success response", zap.Error(err))
		// Fallback to simple response without checksum
		return response
	}

	return finalResponse
}
