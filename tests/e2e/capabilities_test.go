// +build e2e

package e2e

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/spokanepubliclibrary/fsip2/internal/config"
	"github.com/spokanepubliclibrary/fsip2/internal/folio"
	"github.com/spokanepubliclibrary/fsip2/internal/folio/models"
	"github.com/spokanepubliclibrary/fsip2/internal/logging"
	"github.com/spokanepubliclibrary/fsip2/internal/server"
	"github.com/spokanepubliclibrary/fsip2/tests/mocks"
	"github.com/spokanepubliclibrary/fsip2/tests/testutil"
)

// These tests run fsip2 against the mock FOLIO server in capability-enforcement mode: the
// kiosk user ("testuser", see Login) holds only the capabilities of the roles under test,
// and any other endpoint gets Eureka's 403. They check that the roles documented in
// documentation/eureka_capabilities.md are sufficient for every SIP flow.

const (
	capPatronBarcode = "CAP_PATRON"
	capPatronID      = "cap-user"
	capPatronPIN     = "1234"
	capItemBarcode   = "ITEM001" // default mock item (fixtures/items/available_item.json)
	capItemID        = "item-1111-2222-3333-4444"
	capFeeID         = "cap-fee"
)

// newCapabilitySetup starts fsip2 with PIN verification on (so 63/23 call /patron-pin/verify)
// and the mock enforcing the given capability sets for the kiosk user.
func newCapabilitySetup(t *testing.T, acceptBulkPayment bool, rolling *config.RollingRenewalConfig, roles ...[]string) *E2ESetup {
	t.Helper()
	mockFolio := mocks.NewFolioMockServer()
	cfg := buildE2EConfigWithOptions(t, mockFolio.GetURL(), acceptBulkPayment)
	tc := cfg.Tenants["test-inst"]
	tc.PatronPasswordVerificationRequired = true
	tc.UsePinForPatronVerification = true
	tc.RollingRenewals = rolling

	mockFolio.EnableCapabilityEnforcement("testuser", roles...)

	logger, _ := logging.NewLogger("warn", "")
	srv, err := server.NewServer(cfg, logger)
	require.NoError(t, err)
	srv.RegisterAllHandlers()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	go srv.Start(ctx)
	waitForServerReady(t, cfg.Port)

	return &E2ESetup{MockFolio: mockFolio, Server: srv, Config: cfg, Port: cfg.Port, cancel: cancel}
}

// seedCapabilityPatron adds a patron with a loan, an awaiting-pickup hold and an open fee,
// so 63, 23, 17, 65 and 37 exercise their loan, request and account lookups.
func seedCapabilityPatron(m *mocks.FolioMockServer, expiration time.Time) {
	m.AddUser(capPatronBarcode, &models.User{
		ID:             capPatronID,
		Username:       "cap_patron",
		Barcode:        capPatronBarcode,
		Active:         true,
		PatronGroup:    "adults",
		ExpirationDate: &expiration,
		Personal:       models.PersonalInfo{FirstName: "Cap", LastName: "Patron", Email: "cap@example.com"},
	})
	m.SetPatronPIN(capPatronID, capPatronPIN)

	loanDate := time.Now().Add(-24 * time.Hour)
	dueDate := time.Now().Add(13 * 24 * time.Hour)
	m.Loans["cap-loan"] = &models.Loan{
		ID: "cap-loan", ItemID: capItemID, UserID: capPatronID,
		LoanDate: &loanDate, DueDate: &dueDate, Status: models.LoanStatus{Name: "Open"},
	}

	requestDate := time.Now().Add(-48 * time.Hour)
	m.AddRequest(&models.Request{
		ID: "cap-request", RequestType: "Hold", RequestDate: &requestDate,
		RequesterID: capPatronID, ItemID: capItemID, Status: "Open - Awaiting pickup",
	})

	m.AddAccount(&models.Account{
		ID: capFeeID, UserID: capPatronID,
		Amount: models.FlexibleFloat(5.00), Remaining: models.FlexibleFloat(5.00),
		Status: models.AccountStatus{Name: "Open"}, PaymentStatus: models.PaymentStatus{Name: "Outstanding"},
		FeeFineType: "Overdue fine",
	})
}

// capPatronInfo builds a 63 with every summary flag set and the patron's PIN.
func capPatronInfo() string {
	return "63001" + "20250110    081500" + "YYYYYY    " + "|AOtest-inst|AA" + capPatronBarcode + "|AD" + capPatronPIN + "\r"
}

// withPIN appends the patron's PIN (AD), plus any extra fields, to a testutil message.
// The tenant requires patron verification, so every patron message needs it.
func withPIN(message string, extra ...string) string {
	return strings.TrimSuffix(message, "\r") + "|AD" + capPatronPIN + strings.Join(extra, "") + "\r"
}

// capPatronStatus builds a 23 with the patron's PIN.
func capPatronStatus() string {
	return withPIN(testutil.NewPatronStatusMessage("test-inst", capPatronBarcode))
}

// TestE2E_Capabilities_RoleAOnly_AllMessagesSucceed: with SIP2 Service alone, every
// supported SIP message succeeds and FOLIO denies nothing.
func TestE2E_Capabilities_RoleAOnly_AllMessagesSucceed(t *testing.T) {
	setup := newCapabilitySetup(t, false, nil, folio.RoleCapabilities(folio.RoleSIP2Service))
	defer setup.Close(t)
	seedCapabilityPatron(setup.MockFolio, time.Now().AddDate(5, 0, 0))

	conn := setup.Connect(t)
	defer conn.Close()
	setup.Login(t, conn)

	steps := []struct {
		name, message, prefix string
		check                 func(t *testing.T, resp string)
	}{
		{"99 SC status", "9900302.00\r", "98", nil},
		{"23 patron status", capPatronStatus(), "24", func(t *testing.T, resp string) {
			assertSIP2Field(t, resp, "BL", "Y")
			assertSIP2Field(t, resp, "CQ", "Y")
		}},
		{"63 patron information", capPatronInfo(), "64", func(t *testing.T, resp string) {
			assertSIP2Field(t, resp, "BL", "Y")
			assertSIP2Field(t, resp, "CQ", "Y")
			assertSIP2Field(t, resp, "AU", capItemBarcode) // loan -> GET /inventory/items/{id}
		}},
		{"17 item information", testutil.NewItemInformationMessage("test-inst", capItemBarcode), "18", nil},
		{"11 checkout", withPIN(testutil.NewCheckoutMessage("test-inst", capPatronBarcode, capItemBarcode)), "121", nil},
		{"29 renew", withPIN(testutil.NewRenewalMessage("test-inst", capPatronBarcode, capItemBarcode)), "301", nil},
		{"65 renew all", withPIN(testutil.NewRenewAllMessage("test-inst", capPatronBarcode)), "66", func(t *testing.T, resp string) {
			assert.Equal(t, "0001", resp[3:7], "one loan renewed via POST /circulation/renew-by-id")
		}},
		{"09 checkin", testutil.NewCheckinMessage("test-inst", capItemBarcode), "101", nil},
		{"37 fee paid (single)", withPIN(testutil.NewFeePaidMessage("test-inst", capPatronBarcode, 5.00), "|CG"+capFeeID), "38Y", nil},
		{"35 end patron session", "3520250110    081500|AOtest-inst|AA" + capPatronBarcode + "\r", "36", nil},
	}

	for _, step := range steps {
		resp := setup.Exchange(t, conn, step.message)
		assert.True(t, strings.HasPrefix(resp, step.prefix), "%s: expected %q prefix, got %q", step.name, step.prefix, resp)
		if step.check != nil {
			step.check(t, resp)
		}
	}

	assert.Empty(t, setup.MockFolio.GetCapabilityDenials(), "SIP2 Service must cover every SIP flow")
}

// TestE2E_Capabilities_RoleAOnly_BulkFeePaid covers POST /accounts-bulk/pay.
func TestE2E_Capabilities_RoleAOnly_BulkFeePaid(t *testing.T) {
	setup := newCapabilitySetup(t, true, nil, folio.RoleCapabilities(folio.RoleSIP2Service))
	defer setup.Close(t)
	seedCapabilityPatron(setup.MockFolio, time.Now().AddDate(5, 0, 0))

	conn := setup.Connect(t)
	defer conn.Close()
	setup.Login(t, conn)

	resp := setup.Exchange(t, conn, withPIN(testutil.NewFeePaidMessage("test-inst", capPatronBarcode, 5.00)))
	assert.True(t, strings.HasPrefix(resp, "38Y"), "expected 38Y, got %q", resp)
	assert.Equal(t, 1, setup.MockFolio.BulkPayCallCount)
	assert.Empty(t, setup.MockFolio.GetCapabilityDenials())
}

// TestE2E_Capabilities_MissingCapabilityIsDenied checks that enforcement mode bites: without
// circulation_check-out-by-barcode.execute, checkout fails with FOLIO's 403.
func TestE2E_Capabilities_MissingCapabilityIsDenied(t *testing.T) {
	var caps []string
	for _, c := range folio.RoleCapabilities(folio.RoleSIP2Service) {
		if c != "circulation_check-out-by-barcode.execute" {
			caps = append(caps, c)
		}
	}
	setup := newCapabilitySetup(t, false, nil, caps)
	defer setup.Close(t)
	seedCapabilityPatron(setup.MockFolio, time.Now().AddDate(5, 0, 0))

	conn := setup.Connect(t)
	defer conn.Close()
	setup.Login(t, conn)

	resp := setup.Exchange(t, conn, withPIN(testutil.NewCheckoutMessage("test-inst", capPatronBarcode, capItemBarcode)))
	assert.True(t, strings.HasPrefix(resp, "120"), "checkout must fail without its capability, got %q", resp)
	assert.Equal(t,
		[]string{"POST /circulation/check-out-by-barcode -> circulation_check-out-by-barcode.execute"},
		setup.MockFolio.GetCapabilityDenials())
}

func capRollingRenewalConfig() *config.RollingRenewalConfig {
	return &config.RollingRenewalConfig{Enabled: true, RenewWithin: "1M", ExtendFor: "1Y"}
}

// TestE2E_Capabilities_RollingRenewal_RoleAOnly_FailsGracefully: without SIP2 Rolling
// Renewal the 64 is normal and the expiration date is unchanged.
func TestE2E_Capabilities_RollingRenewal_RoleAOnly_FailsGracefully(t *testing.T) {
	setup := newCapabilitySetup(t, false, capRollingRenewalConfig(), folio.RoleCapabilities(folio.RoleSIP2Service))
	defer setup.Close(t)
	expiration := time.Now().AddDate(0, 0, 10).UTC().Truncate(time.Second)
	seedCapabilityPatron(setup.MockFolio, expiration)

	conn := setup.Connect(t)
	defer conn.Close()
	setup.Login(t, conn)

	resp := setup.Exchange(t, conn, capPatronInfo())
	require.True(t, strings.HasPrefix(resp, "64"), "expected 64, got %q", resp)
	assertSIP2Field(t, resp, "BL", "Y")

	assert.Equal(t, 0, setup.MockFolio.UserPutCount)
	assert.True(t, setup.MockFolio.Users[capPatronBarcode].ExpirationDate.Equal(expiration), "expiration must be unchanged")
	assert.Equal(t, []string{"PUT /users/" + capPatronID + " -> users_item.edit"}, setup.MockFolio.GetCapabilityDenials())
}

// TestE2E_Capabilities_RollingRenewal_RoleAAndB_Extends: with both roles the expiration is
// extended and nothing is denied.
func TestE2E_Capabilities_RollingRenewal_RoleAAndB_Extends(t *testing.T) {
	setup := newCapabilitySetup(t, false, capRollingRenewalConfig(),
		folio.RoleCapabilities(folio.RoleSIP2Service), folio.RoleCapabilities(folio.RoleSIP2RollingRenewal))
	defer setup.Close(t)
	expiration := time.Now().AddDate(0, 0, 10).UTC().Truncate(time.Second)
	seedCapabilityPatron(setup.MockFolio, expiration)

	conn := setup.Connect(t)
	defer conn.Close()
	setup.Login(t, conn)

	resp := setup.Exchange(t, conn, capPatronInfo())
	require.True(t, strings.HasPrefix(resp, "64"), "expected 64, got %q", resp)

	assert.Equal(t, 1, setup.MockFolio.UserPutCount)
	newExpiration := setup.MockFolio.Users[capPatronBarcode].ExpirationDate
	assert.True(t, newExpiration.After(expiration.AddDate(0, 6, 0)), "expiration must be extended by ~1Y, got %v", newExpiration)
	assert.Empty(t, setup.MockFolio.GetCapabilityDenials())
}
