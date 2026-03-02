package handlers

import (
	"context"
	"strings"
	"testing"

	"github.com/spokanepubliclibrary/fsip2/internal/sip2/parser"
	"github.com/spokanepubliclibrary/fsip2/tests/testutil"
	"go.uber.org/zap"
)

// ─── PatronStatusHandler.Handle ──────────────────────────────────────────────

func TestPatronStatusHandle_MissingInstitutionID(t *testing.T) {
	tc := testutil.NewTenantConfig()
	h := NewPatronStatusHandler(zap.NewNop(), tc)
	msg := buildTestMsg(parser.PatronStatusRequest, map[parser.FieldCode]string{
		parser.PatronIdentifier: "P123",
	})
	resp, err := h.Handle(context.Background(), msg, testutil.NewSession(tc))
	if err == nil {
		t.Error("Expected validation error when institution ID is missing")
	}
	if resp != "96" {
		t.Errorf("Expected '96' error response, got: %q", resp)
	}
}

func TestPatronStatusHandle_MissingPatronIdentifier(t *testing.T) {
	tc := testutil.NewTenantConfig()
	h := NewPatronStatusHandler(zap.NewNop(), tc)
	msg := buildTestMsg(parser.PatronStatusRequest, map[parser.FieldCode]string{
		parser.InstitutionID: "INST01",
	})
	resp, err := h.Handle(context.Background(), msg, testutil.NewSession(tc))
	if err == nil {
		t.Error("Expected validation error when patron identifier is missing")
	}
	if resp != "96" {
		t.Errorf("Expected '96' error response, got: %q", resp)
	}
}

func TestPatronStatusHandle_NoAuthToken(t *testing.T) {
	tc := testutil.NewTenantConfig()
	h := NewPatronStatusHandler(zap.NewNop(), tc)
	msg := buildTestMsg(parser.PatronStatusRequest, map[parser.FieldCode]string{
		parser.InstitutionID:    "INST01",
		parser.PatronIdentifier: "P123",
	})
	resp, err := h.Handle(context.Background(), msg, testutil.NewSession(tc))
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !strings.HasPrefix(resp, "24") {
		t.Errorf("Expected patron status response (24...), got: %q", resp[:min(5, len(resp))])
	}
}

// ─── PatronInformationHandler.Handle ─────────────────────────────────────────

func TestPatronInformationHandle_MissingInstitutionID(t *testing.T) {
	tc := testutil.NewTenantConfig()
	h := NewPatronInformationHandler(zap.NewNop(), tc)
	msg := buildTestMsg(parser.PatronInformationRequest, map[parser.FieldCode]string{
		parser.PatronIdentifier: "P123",
	})
	resp, err := h.Handle(context.Background(), msg, testutil.NewSession(tc))
	if err == nil {
		t.Error("Expected validation error when institution ID is missing")
	}
	if resp != "96" {
		t.Errorf("Expected '96' error response, got: %q", resp)
	}
}

func TestPatronInformationHandle_MissingPatronIdentifier(t *testing.T) {
	tc := testutil.NewTenantConfig()
	h := NewPatronInformationHandler(zap.NewNop(), tc)
	msg := buildTestMsg(parser.PatronInformationRequest, map[parser.FieldCode]string{
		parser.InstitutionID: "INST01",
	})
	resp, err := h.Handle(context.Background(), msg, testutil.NewSession(tc))
	if err == nil {
		t.Error("Expected validation error when patron identifier is missing")
	}
	if resp != "96" {
		t.Errorf("Expected '96' error response, got: %q", resp)
	}
}

func TestPatronInformationHandle_NoAuthToken(t *testing.T) {
	tc := testutil.NewTenantConfig()
	h := NewPatronInformationHandler(zap.NewNop(), tc)
	msg := buildTestMsg(parser.PatronInformationRequest, map[parser.FieldCode]string{
		parser.InstitutionID:    "INST01",
		parser.PatronIdentifier: "P123",
	})
	resp, err := h.Handle(context.Background(), msg, testutil.NewSession(tc))
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !strings.HasPrefix(resp, "64") {
		t.Errorf("Expected patron information response (64...), got: %q", resp[:min(5, len(resp))])
	}
}

// ─── CheckinHandler.Handle ────────────────────────────────────────────────────

func TestCheckinHandle_MissingInstitutionID(t *testing.T) {
	tc := testutil.NewTenantConfig()
	h := NewCheckinHandler(zap.NewNop(), tc)
	msg := buildTestMsg(parser.CheckinRequest, map[parser.FieldCode]string{
		parser.ItemIdentifier: "ITEM01",
	})
	resp, err := h.Handle(context.Background(), msg, testutil.NewSession(tc))
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !strings.HasPrefix(resp, "10") {
		t.Errorf("Expected checkin response (10...), got: %q", resp[:min(5, len(resp))])
	}
	if len(resp) >= 3 && resp[2] != '0' {
		t.Errorf("Expected failure response (0), got: %c", resp[2])
	}
}

func TestCheckinHandle_MissingItemIdentifier(t *testing.T) {
	tc := testutil.NewTenantConfig()
	h := NewCheckinHandler(zap.NewNop(), tc)
	msg := buildTestMsg(parser.CheckinRequest, map[parser.FieldCode]string{
		parser.InstitutionID: "INST01",
	})
	resp, err := h.Handle(context.Background(), msg, testutil.NewSession(tc))
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !strings.HasPrefix(resp, "10") {
		t.Errorf("Expected checkin response (10...), got: %q", resp[:min(5, len(resp))])
	}
}

func TestCheckinHandle_NoAuthToken(t *testing.T) {
	tc := testutil.NewTenantConfig()
	h := NewCheckinHandler(zap.NewNop(), tc)
	msg := buildTestMsg(parser.CheckinRequest, map[parser.FieldCode]string{
		parser.InstitutionID:  "INST01",
		parser.ItemIdentifier: "ITEM01",
	})
	resp, err := h.Handle(context.Background(), msg, testutil.NewSession(tc))
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !strings.HasPrefix(resp, "10") {
		t.Errorf("Expected checkin response (10...), got: %q", resp[:min(5, len(resp))])
	}
}

// ─── CheckoutHandler.Handle ───────────────────────────────────────────────────

func TestCheckoutHandle_MissingInstitutionID(t *testing.T) {
	tc := testutil.NewTenantConfig()
	h := NewCheckoutHandler(zap.NewNop(), tc)
	msg := buildTestMsg(parser.CheckoutRequest, map[parser.FieldCode]string{
		parser.PatronIdentifier: "P123",
		parser.ItemIdentifier:   "ITEM01",
	})
	resp, err := h.Handle(context.Background(), msg, testutil.NewSession(tc))
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !strings.HasPrefix(resp, "12") {
		t.Errorf("Expected checkout response (12...), got: %q", resp[:min(5, len(resp))])
	}
}

func TestCheckoutHandle_MissingPatronIdentifier(t *testing.T) {
	tc := testutil.NewTenantConfig()
	h := NewCheckoutHandler(zap.NewNop(), tc)
	msg := buildTestMsg(parser.CheckoutRequest, map[parser.FieldCode]string{
		parser.InstitutionID:  "INST01",
		parser.ItemIdentifier: "ITEM01",
	})
	resp, err := h.Handle(context.Background(), msg, testutil.NewSession(tc))
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !strings.HasPrefix(resp, "12") {
		t.Errorf("Expected checkout response (12...), got: %q", resp[:min(5, len(resp))])
	}
}

func TestCheckoutHandle_MissingItemIdentifier(t *testing.T) {
	tc := testutil.NewTenantConfig()
	h := NewCheckoutHandler(zap.NewNop(), tc)
	msg := buildTestMsg(parser.CheckoutRequest, map[parser.FieldCode]string{
		parser.InstitutionID:    "INST01",
		parser.PatronIdentifier: "P123",
	})
	resp, err := h.Handle(context.Background(), msg, testutil.NewSession(tc))
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !strings.HasPrefix(resp, "12") {
		t.Errorf("Expected checkout response (12...), got: %q", resp[:min(5, len(resp))])
	}
}

func TestCheckoutHandle_NoAuthToken(t *testing.T) {
	tc := testutil.NewTenantConfig()
	h := NewCheckoutHandler(zap.NewNop(), tc)
	msg := buildTestMsg(parser.CheckoutRequest, map[parser.FieldCode]string{
		parser.InstitutionID:    "INST01",
		parser.PatronIdentifier: "P123",
		parser.ItemIdentifier:   "ITEM01",
	})
	resp, err := h.Handle(context.Background(), msg, testutil.NewSession(tc))
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !strings.HasPrefix(resp, "12") {
		t.Errorf("Expected checkout response (12...), got: %q", resp[:min(5, len(resp))])
	}
}

// ─── ItemStatusUpdateHandler.Handle ──────────────────────────────────────────

func TestItemStatusUpdateHandle_NoAuthToken(t *testing.T) {
	tc := testutil.NewTenantConfig()
	h := NewItemStatusUpdateHandler(zap.NewNop(), tc)
	msg := buildTestMsg(parser.ItemStatusUpdateRequest, map[parser.FieldCode]string{
		parser.InstitutionID:  "INST01",
		parser.ItemIdentifier: "ITEM01",
	})
	resp, err := h.Handle(context.Background(), msg, testutil.NewSession(tc))
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !strings.HasPrefix(resp, "20") {
		t.Errorf("Expected item status update response (20...), got: %q", resp[:min(5, len(resp))])
	}
}

// ─── LoginHandler additional paths ────────────────────────────────────────────

func TestLoginHandle_MissingPassword(t *testing.T) {
	tc := testutil.NewTenantConfig()
	h := NewLoginHandler(zap.NewNop(), tc)
	msg := buildTestMsg(parser.LoginRequest, map[parser.FieldCode]string{
		parser.LoginUserID: "admin",
	})
	resp, err := h.Handle(context.Background(), msg, testutil.NewSession(tc))
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !strings.HasPrefix(resp, "94") {
		t.Errorf("Expected login response (94...), got: %q", resp[:min(5, len(resp))])
	}
	if len(resp) >= 3 && resp[2] != '0' {
		t.Errorf("Expected login failure (0), got: %c", resp[2])
	}
}

func TestLoginHandle_NetworkFailure(t *testing.T) {
	tc := testutil.NewTenantConfig() // OkapiURL points to 127.0.0.1:9999 which will fail
	h := NewLoginHandler(zap.NewNop(), tc)
	msg := buildTestMsg(parser.LoginRequest, map[parser.FieldCode]string{
		parser.LoginUserID:   "admin",
		parser.LoginPassword: "wrong",
		parser.LocationCode:  "CIRC",
	})
	resp, err := h.Handle(context.Background(), msg, testutil.NewSession(tc))
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !strings.HasPrefix(resp, "94") {
		t.Errorf("Expected login response (94...), got: %q", resp[:min(5, len(resp))])
	}
}
