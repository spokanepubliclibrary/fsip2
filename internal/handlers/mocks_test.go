package handlers

import (
	"context"

	"github.com/stretchr/testify/mock"

	"github.com/spokanepubliclibrary/fsip2/internal/folio"
	"github.com/spokanepubliclibrary/fsip2/internal/folio/models"
	"github.com/spokanepubliclibrary/fsip2/internal/sip2/parser"
	"github.com/spokanepubliclibrary/fsip2/internal/types"
)

// ─── MockPatronClient ─────────────────────────────────────────────────────────

// MockPatronClient implements PatronLookup for use in handler tests.
type MockPatronClient struct{ mock.Mock }

func (m *MockPatronClient) GetUserByBarcode(ctx context.Context, token, barcode string) (*models.User, error) {
	args := m.Called(ctx, token, barcode)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

func (m *MockPatronClient) GetUserByID(ctx context.Context, token, userID string) (*models.User, error) {
	args := m.Called(ctx, token, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

func (m *MockPatronClient) GetManualBlocks(ctx context.Context, token, userID string) (*models.ManualBlockCollection, error) {
	args := m.Called(ctx, token, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.ManualBlockCollection), args.Error(1)
}

func (m *MockPatronClient) GetAutomatedPatronBlocks(ctx context.Context, token, userID string) (*models.AutomatedPatronBlock, error) {
	args := m.Called(ctx, token, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.AutomatedPatronBlock), args.Error(1)
}

func (m *MockPatronClient) GetPatronGroupByID(ctx context.Context, token, groupID string) (*models.PatronGroup, error) {
	args := m.Called(ctx, token, groupID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.PatronGroup), args.Error(1)
}

func (m *MockPatronClient) VerifyPatronPin(ctx context.Context, token, userID, pin string) (bool, error) {
	args := m.Called(ctx, token, userID, pin)
	return args.Bool(0), args.Error(1)
}

func (m *MockPatronClient) VerifyPatronPasswordWithLogin(ctx context.Context, username, password string) (bool, error) {
	args := m.Called(ctx, username, password)
	return args.Bool(0), args.Error(1)
}

func (m *MockPatronClient) UpdateUserExpiration(ctx context.Context, token, userID, newExpiration string, reactivate bool) error {
	args := m.Called(ctx, token, userID, newExpiration, reactivate)
	return args.Error(0)
}

// ─── MockCirculationClient ────────────────────────────────────────────────────

// MockCirculationClient implements CirculationLookup for use in handler tests.
type MockCirculationClient struct{ mock.Mock }

func (m *MockCirculationClient) Checkout(ctx context.Context, token string, req folio.CheckoutRequest) (*models.Loan, error) {
	args := m.Called(ctx, token, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Loan), args.Error(1)
}

func (m *MockCirculationClient) Checkin(ctx context.Context, token string, req folio.CheckinRequest) (*models.Loan, error) {
	args := m.Called(ctx, token, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Loan), args.Error(1)
}

func (m *MockCirculationClient) Renew(ctx context.Context, token string, req folio.RenewRequest) (*models.Loan, error) {
	args := m.Called(ctx, token, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Loan), args.Error(1)
}

func (m *MockCirculationClient) RenewByID(ctx context.Context, token string, req folio.RenewByIDRequest) (*models.Loan, error) {
	args := m.Called(ctx, token, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Loan), args.Error(1)
}

func (m *MockCirculationClient) GetLoansByUser(ctx context.Context, token, userID string) (*models.LoanCollection, error) {
	args := m.Called(ctx, token, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.LoanCollection), args.Error(1)
}

func (m *MockCirculationClient) GetOpenLoansByUser(ctx context.Context, token, userID string) (*models.LoanCollection, error) {
	args := m.Called(ctx, token, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.LoanCollection), args.Error(1)
}

func (m *MockCirculationClient) GetOpenRequestsByUser(ctx context.Context, token, userID string) (*models.RequestCollection, error) {
	args := m.Called(ctx, token, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.RequestCollection), args.Error(1)
}

func (m *MockCirculationClient) GetAvailableHolds(ctx context.Context, token, userID string) (*models.RequestCollection, error) {
	args := m.Called(ctx, token, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.RequestCollection), args.Error(1)
}

func (m *MockCirculationClient) GetUnavailableHolds(ctx context.Context, token, userID string) (*models.RequestCollection, error) {
	args := m.Called(ctx, token, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.RequestCollection), args.Error(1)
}

func (m *MockCirculationClient) GetLoansByItem(ctx context.Context, token, itemID string) (*models.LoanCollection, error) {
	args := m.Called(ctx, token, itemID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.LoanCollection), args.Error(1)
}

func (m *MockCirculationClient) GetRequestsByItem(ctx context.Context, token, itemID string) (*models.RequestCollection, error) {
	args := m.Called(ctx, token, itemID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.RequestCollection), args.Error(1)
}

// ─── MockInventoryClient ──────────────────────────────────────────────────────

// MockInventoryClient implements InventoryLookup for use in handler tests.
type MockInventoryClient struct{ mock.Mock }

func (m *MockInventoryClient) GetItemByBarcode(ctx context.Context, token, barcode string) (*models.Item, error) {
	args := m.Called(ctx, token, barcode)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Item), args.Error(1)
}

func (m *MockInventoryClient) GetItemByID(ctx context.Context, token, itemID string) (*models.Item, error) {
	args := m.Called(ctx, token, itemID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Item), args.Error(1)
}

func (m *MockInventoryClient) GetInstanceByID(ctx context.Context, token, instanceID string) (*models.Instance, error) {
	args := m.Called(ctx, token, instanceID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Instance), args.Error(1)
}

func (m *MockInventoryClient) GetHoldingsByID(ctx context.Context, token, holdingsID string) (*models.Holdings, error) {
	args := m.Called(ctx, token, holdingsID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Holdings), args.Error(1)
}

func (m *MockInventoryClient) GetLocationByID(ctx context.Context, token, locationID string) (*models.Location, error) {
	args := m.Called(ctx, token, locationID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Location), args.Error(1)
}

func (m *MockInventoryClient) GetMaterialTypeByID(ctx context.Context, token, materialTypeID string) (*models.MaterialType, error) {
	args := m.Called(ctx, token, materialTypeID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.MaterialType), args.Error(1)
}

func (m *MockInventoryClient) GetServicePointByID(ctx context.Context, token, servicePointID string) (*models.ServicePoint, error) {
	args := m.Called(ctx, token, servicePointID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.ServicePoint), args.Error(1)
}

// ─── MockFeesClient ───────────────────────────────────────────────────────────

// MockFeesClient implements FeesOps for use in handler tests.
type MockFeesClient struct{ mock.Mock }

func (m *MockFeesClient) GetOpenAccountsExcludingSuspended(ctx context.Context, token, userID string) (*models.AccountCollection, error) {
	args := m.Called(ctx, token, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.AccountCollection), args.Error(1)
}

func (m *MockFeesClient) GetEligibleAccountByID(ctx context.Context, token, accountID string) (*models.Account, error) {
	args := m.Called(ctx, token, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Account), args.Error(1)
}

func (m *MockFeesClient) PayAccount(ctx context.Context, token, accountID string, payment *models.PaymentRequest) (*models.PaymentResponse, error) {
	args := m.Called(ctx, token, accountID, payment)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.PaymentResponse), args.Error(1)
}

func (m *MockFeesClient) PayBulkAccounts(ctx context.Context, token string, payment *models.Payment) error {
	args := m.Called(ctx, token, payment)
	return args.Error(0)
}

// ─── Injection helper ─────────────────────────────────────────────────────────

// injectMocks overrides factory functions on h so that the given mocks are
// returned for all client calls within a test. Pass nil for any interface you
// do not need to override.
func injectMocks(h *BaseHandler, patron PatronLookup, circ CirculationLookup, inv InventoryLookup, fees FeesOps) {
	if patron != nil {
		h.newPatronClient = func(_ *types.Session) PatronLookup { return patron }
	}
	if circ != nil {
		h.newCirculationClient = func(_ *types.Session) CirculationLookup { return circ }
	}
	if inv != nil {
		h.newInventoryClient = func(_ *types.Session) InventoryLookup { return inv }
	}
	if fees != nil {
		h.newFeesClient = func(_ *types.Session) FeesOps { return fees }
	}
}

// ─── Message builder helper ───────────────────────────────────────────────────

// buildTestMsg constructs a *parser.Message for use in handler unit tests.
// Replaces the newMsg() helper in handle_extended_test.go.
func buildTestMsg(code parser.MessageCode, fields map[parser.FieldCode]string) *parser.Message {
	m := &parser.Message{
		Code:   code,
		Fields: make(map[string]string, len(fields)),
	}
	for fc, v := range fields {
		m.Fields[string(fc)] = v
	}
	return m
}
