package handlers

import (
	"context"
	"strings"
	"testing"

	"github.com/spokanepubliclibrary/fsip2/internal/config"
	"github.com/spokanepubliclibrary/fsip2/internal/folio/models"
	"github.com/spokanepubliclibrary/fsip2/internal/sip2/parser"
	"github.com/spokanepubliclibrary/fsip2/internal/types"
	"go.uber.org/zap"
)

func newFeePaidHandler(tc *config.TenantConfig) *FeePaidHandler {
	if tc == nil {
		tc = &config.TenantConfig{
			Tenant:                "test-tenant",
			MessageDelimiter:      "\r",
			FieldDelimiter:        "|",
			Charset:               "UTF-8",
			ErrorDetectionEnabled: true,
			OkapiURL:              "http://localhost:9130",
		}
	}
	return NewFeePaidHandler(zap.NewNop(), tc)
}

func newFeePaidSession(tc *config.TenantConfig) *types.Session {
	return types.NewSession("fee-test-session", tc)
}

func newFeePaidMsg(fields map[string]string) *parser.Message {
	return &parser.Message{
		Code:           parser.FeePaidRequest,
		Fields:         fields,
		SequenceNumber: "0",
	}
}

// TestBuildSuccessResponse_SinglePayment tests buildSuccessResponse for a single payment.
func TestBuildSuccessResponse_SinglePayment(t *testing.T) {
	tc := &config.TenantConfig{
		Tenant:                "test-tenant",
		MessageDelimiter:      "\r",
		FieldDelimiter:        "|",
		Charset:               "UTF-8",
		ErrorDetectionEnabled: true,
	}
	h := NewFeePaidHandler(zap.NewNop(), tc)
	session := types.NewSession("test", tc)

	msg := newFeePaidMsg(map[string]string{
		string(parser.InstitutionID):    "INST01",
		string(parser.PatronIdentifier): "P123",
		string(parser.FeeAmount):        "10.00",
	})

	results := []paymentResult{
		{
			account: &models.Account{
				ID:        "acc-123",
				FeeFineID: "ff-123",
			},
			paymentResponse: &models.PaymentResponse{
				RemainingAmount: "0.00",
			},
			amountApplied: 10.00,
			success:       true,
		},
	}

	resp := h.buildSuccessResponse("INST01", "P123", results, false, msg, session)

	if !strings.HasPrefix(resp, "38") {
		t.Errorf("Response should start with '38', got: %s", resp[:min(5, len(resp))])
	}
	if len(resp) < 3 || resp[2] != 'Y' {
		t.Errorf("Response should indicate success (Y), got: %q", resp[:min(5, len(resp))])
	}
	if !strings.Contains(resp, "INST01") {
		t.Error("Response should contain institution ID")
	}
	if !strings.Contains(resp, "P123") {
		t.Error("Response should contain patron identifier")
	}
	if !strings.Contains(resp, "|CGacc-123") {
		t.Error("Response should contain account ID (CG field)")
	}
	if !strings.Contains(resp, "Payment accepted") {
		t.Error("Response should contain 'Payment accepted' message")
	}
}

// TestBuildSuccessResponse_BulkPaymentAllSuccess tests buildSuccessResponse for bulk payment success.
func TestBuildSuccessResponse_BulkPaymentAllSuccess(t *testing.T) {
	tc := &config.TenantConfig{
		Tenant:           "test-tenant",
		MessageDelimiter: "\r",
		FieldDelimiter:   "|",
		Charset:          "UTF-8",
	}
	h := NewFeePaidHandler(zap.NewNop(), tc)
	session := types.NewSession("test", tc)

	msg := newFeePaidMsg(map[string]string{
		string(parser.InstitutionID):    "INST01",
		string(parser.PatronIdentifier): "P456",
		string(parser.FeeAmount):        "25.00",
	})

	results := []paymentResult{
		{
			account:         &models.Account{ID: "acc-1", FeeFineID: "ff-1"},
			paymentResponse: &models.PaymentResponse{RemainingAmount: "0.00"},
			amountApplied:   12.50,
			success:         true,
		},
		{
			account:         &models.Account{ID: "acc-2", FeeFineID: "ff-2"},
			paymentResponse: &models.PaymentResponse{RemainingAmount: "0.00"},
			amountApplied:   12.50,
			success:         true,
		},
	}

	resp := h.buildSuccessResponse("INST01", "P456", results, true, msg, session)

	if !strings.HasPrefix(resp, "38") {
		t.Errorf("Response should start with '38'")
	}
	if !strings.Contains(resp, "Bulk payment applied") {
		t.Error("Bulk payment response should contain 'Bulk payment applied'")
	}
}

// TestBuildSuccessResponse_BulkPaymentPartialFailure tests buildSuccessResponse with mixed results.
func TestBuildSuccessResponse_BulkPaymentPartialFailure(t *testing.T) {
	tc := &config.TenantConfig{
		Tenant:           "test-tenant",
		MessageDelimiter: "\r",
		FieldDelimiter:   "|",
		Charset:          "UTF-8",
	}
	h := NewFeePaidHandler(zap.NewNop(), tc)
	session := types.NewSession("test", tc)

	msg := newFeePaidMsg(map[string]string{
		string(parser.InstitutionID):    "INST01",
		string(parser.PatronIdentifier): "P789",
		string(parser.FeeAmount):        "20.00",
	})

	results := []paymentResult{
		{
			account:         &models.Account{ID: "acc-1", FeeFineID: "ff-1"},
			paymentResponse: &models.PaymentResponse{RemainingAmount: "0.00"},
			amountApplied:   10.00,
			success:         true,
		},
		{
			account:       &models.Account{ID: "acc-2"},
			amountApplied: 10.00,
			success:       false,
		},
	}

	resp := h.buildSuccessResponse("INST01", "P789", results, true, msg, session)

	if !strings.HasPrefix(resp, "38") {
		t.Errorf("Response should start with '38'")
	}
	if !strings.Contains(resp, "see staff for details") {
		t.Error("Partial failure response should contain 'see staff for details'")
	}
}

// TestBuildSuccessResponse_WithTransactionID tests that transaction ID is included if present.
func TestBuildSuccessResponse_WithTransactionID(t *testing.T) {
	tc := &config.TenantConfig{
		Tenant:           "test-tenant",
		MessageDelimiter: "\r",
		FieldDelimiter:   "|",
		Charset:          "UTF-8",
	}
	h := NewFeePaidHandler(zap.NewNop(), tc)
	session := types.NewSession("test", tc)

	msg := &parser.Message{
		Code:           parser.FeePaidRequest,
		SequenceNumber: "0",
		Fields: map[string]string{
			string(parser.InstitutionID):    "INST01",
			string(parser.PatronIdentifier): "P999",
			string(parser.FeeAmount):        "5.00",
			string(parser.TransactionID):    "TXN-42",
		},
	}

	results := []paymentResult{
		{
			account:         &models.Account{ID: "acc-x", FeeFineID: "ff-x"},
			paymentResponse: &models.PaymentResponse{RemainingAmount: "0.00"},
			amountApplied:   5.00,
			success:         true,
		},
	}

	resp := h.buildSuccessResponse("INST01", "P999", results, false, msg, session)
	if !strings.Contains(resp, "TXN-42") {
		t.Error("Response should contain transaction ID (BK field)")
	}
}

// TestFeePaidHandle_InvalidFeeAmountString tests Handle with non-numeric fee amount.
func TestFeePaidHandle_InvalidFeeAmountString(t *testing.T) {
	tc := &config.TenantConfig{
		Tenant:           "test-tenant",
		MessageDelimiter: "\r",
		FieldDelimiter:   "|",
		Charset:          "UTF-8",
	}
	h := NewFeePaidHandler(zap.NewNop(), tc)
	session := newFeePaidSession(tc)

	msg := newFeePaidMsg(map[string]string{
		string(parser.InstitutionID):    "INST01",
		string(parser.PatronIdentifier): "P123",
		string(parser.FeeAmount):        "not-a-number",
	})

	resp, err := h.Handle(context.Background(), msg, session)
	if err != nil {
		t.Errorf("Handle() returned unexpected error: %v", err)
	}
	if !strings.HasPrefix(resp, "38") {
		t.Errorf("Expected fee paid response (38), got: %s", resp[:min(5, len(resp))])
	}
	if resp[2] != 'N' {
		t.Errorf("Expected failure response (N), got: %c", resp[2])
	}
}

// TestFeePaidHandle_ZeroFeeAmount tests Handle with zero fee amount.
func TestFeePaidHandle_ZeroFeeAmount(t *testing.T) {
	tc := &config.TenantConfig{
		Tenant:           "test-tenant",
		MessageDelimiter: "\r",
		FieldDelimiter:   "|",
		Charset:          "UTF-8",
	}
	h := NewFeePaidHandler(zap.NewNop(), tc)
	session := newFeePaidSession(tc)

	msg := newFeePaidMsg(map[string]string{
		string(parser.InstitutionID):    "INST01",
		string(parser.PatronIdentifier): "P123",
		string(parser.FeeAmount):        "0.00",
	})

	resp, err := h.Handle(context.Background(), msg, session)
	if err != nil {
		t.Errorf("Handle() returned unexpected error: %v", err)
	}
	if !strings.HasPrefix(resp, "38") {
		t.Errorf("Expected fee paid response (38), got: %s", resp[:min(5, len(resp))])
	}
	if resp[2] != 'N' {
		t.Errorf("Expected failure response (N) for zero amount, got: %c", resp[2])
	}
}

// TestFeePaidHandle_NegativeFeeAmount tests Handle with negative fee amount.
func TestFeePaidHandle_NegativeFeeAmount(t *testing.T) {
	tc := &config.TenantConfig{
		Tenant:           "test-tenant",
		MessageDelimiter: "\r",
		FieldDelimiter:   "|",
		Charset:          "UTF-8",
	}
	h := NewFeePaidHandler(zap.NewNop(), tc)
	session := newFeePaidSession(tc)

	msg := newFeePaidMsg(map[string]string{
		string(parser.InstitutionID):    "INST01",
		string(parser.PatronIdentifier): "P123",
		string(parser.FeeAmount):        "-5.00",
	})

	resp, err := h.Handle(context.Background(), msg, session)
	if err != nil {
		t.Errorf("Handle() returned unexpected error: %v", err)
	}
	if !strings.HasPrefix(resp, "38") {
		t.Errorf("Expected fee paid response (38), got: %s", resp[:min(5, len(resp))])
	}
	if resp[2] != 'N' {
		t.Errorf("Expected failure response (N) for negative amount, got: %c", resp[2])
	}
}

// TestFeePaidHandle_NoAuthToken tests Handle when no auth token is available.
func TestFeePaidHandle_NoAuthToken(t *testing.T) {
	tc := &config.TenantConfig{
		Tenant:           "test-tenant",
		MessageDelimiter: "\r",
		FieldDelimiter:   "|",
		Charset:          "UTF-8",
		OkapiURL:         "http://localhost:9130",
	}
	h := NewFeePaidHandler(zap.NewNop(), tc)
	session := newFeePaidSession(tc) // No auth token

	msg := newFeePaidMsg(map[string]string{
		string(parser.InstitutionID):    "INST01",
		string(parser.PatronIdentifier): "P123",
		string(parser.FeeAmount):        "10.00",
	})

	resp, err := h.Handle(context.Background(), msg, session)
	if err != nil {
		t.Errorf("Handle() returned unexpected error: %v", err)
	}
	if !strings.HasPrefix(resp, "38") {
		t.Errorf("Expected fee paid response (38), got: %s", resp[:min(5, len(resp))])
	}
	if resp[2] != 'N' {
		t.Errorf("Expected failure response (N) when no auth token, got: %c", resp[2])
	}
}

// TestFeePaidHandle_MissingFeeAmount tests Handle with missing fee amount field.
func TestFeePaidHandle_MissingFeeAmount(t *testing.T) {
	tc := &config.TenantConfig{
		Tenant:           "test-tenant",
		MessageDelimiter: "\r",
		FieldDelimiter:   "|",
		Charset:          "UTF-8",
	}
	h := NewFeePaidHandler(zap.NewNop(), tc)
	session := newFeePaidSession(tc)

	msg := newFeePaidMsg(map[string]string{
		string(parser.InstitutionID):    "INST01",
		string(parser.PatronIdentifier): "P123",
		// FeeAmount missing
	})

	resp, err := h.Handle(context.Background(), msg, session)
	if err != nil {
		t.Errorf("Handle() returned unexpected error: %v", err)
	}
	if !strings.HasPrefix(resp, "38") {
		t.Errorf("Expected fee paid response (38), got: %s", resp[:min(5, len(resp))])
	}
}
