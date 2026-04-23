package handlers

import (
	"context"

	"github.com/spokanepubliclibrary/fsip2/internal/folio"
	"github.com/spokanepubliclibrary/fsip2/internal/folio/models"
)

// PatronLookup is the subset of folio.PatronClient methods used by handlers.
// Defined here (consumer side) so tests can inject mocks without depending on
// the folio package's concrete type.
type PatronLookup interface {
	GetUserByBarcode(ctx context.Context, token, barcode string) (*models.User, error)
	GetUserByID(ctx context.Context, token, userID string) (*models.User, error)
	GetManualBlocks(ctx context.Context, token, userID string) (*models.ManualBlockCollection, error)
	GetAutomatedPatronBlocks(ctx context.Context, token, userID string) (*models.AutomatedPatronBlock, error)
	GetPatronGroupByID(ctx context.Context, token, groupID string) (*models.PatronGroup, error)
	VerifyPatronPin(ctx context.Context, token, userID, pin string) (bool, error)
	VerifyPatronPasswordWithLogin(ctx context.Context, username, password string) (bool, error)
	UpdateUserExpiration(ctx context.Context, token, userID, newExpiration string, reactivate bool) error
}

// CirculationLookup is the subset of folio.CirculationClient methods used by handlers.
type CirculationLookup interface {
	Checkout(ctx context.Context, token string, req folio.CheckoutRequest) (*models.Loan, error)
	Checkin(ctx context.Context, token string, req folio.CheckinRequest) (*models.Loan, error)
	Renew(ctx context.Context, token string, req folio.RenewRequest) (*models.Loan, error)
	RenewByID(ctx context.Context, token string, req folio.RenewByIDRequest) (*models.Loan, error)
	GetLoansByUser(ctx context.Context, token, userID string) (*models.LoanCollection, error)
	GetOpenLoansByUser(ctx context.Context, token, userID string) (*models.LoanCollection, error)
	GetOpenRequestsByUser(ctx context.Context, token, userID string) (*models.RequestCollection, error)
	GetAvailableHolds(ctx context.Context, token, userID string) (*models.RequestCollection, error)
	GetUnavailableHolds(ctx context.Context, token, userID string) (*models.RequestCollection, error)
	GetLoansByItem(ctx context.Context, token, itemID string) (*models.LoanCollection, error)
	GetRequestsByItem(ctx context.Context, token, itemID string) (*models.RequestCollection, error)
}

// InventoryLookup is the subset of folio.InventoryClient methods used by handlers.
type InventoryLookup interface {
	GetItemByBarcode(ctx context.Context, token, barcode string) (*models.Item, error)
	GetItemByID(ctx context.Context, token, itemID string) (*models.Item, error)
	GetInstanceByID(ctx context.Context, token, instanceID string) (*models.Instance, error)
	GetHoldingsByID(ctx context.Context, token, holdingsID string) (*models.Holdings, error)
	GetLocationByID(ctx context.Context, token, locationID string) (*models.Location, error)
	GetMaterialTypeByID(ctx context.Context, token, materialTypeID string) (*models.MaterialType, error)
	GetServicePointByID(ctx context.Context, token, servicePointID string) (*models.ServicePoint, error)
}

// FeesOps is the subset of folio.FeesClient methods used by handlers.
type FeesOps interface {
	GetOpenAccountsExcludingSuspended(ctx context.Context, token, userID string) (*models.AccountCollection, error)
	GetEligibleAccountByID(ctx context.Context, token, accountID string) (*models.Account, error)
	PayAccount(ctx context.Context, token, accountID string, payment *models.PaymentRequest) (*models.PaymentResponse, error)
	PayBulkAccounts(ctx context.Context, token string, payment *models.Payment) error
}

// Compile-time assertions: these lines fail to compile if the concrete folio
// client types no longer satisfy the interfaces defined above.  Any method
// rename or signature change in the folio package will surface here
// immediately rather than at test time.
var _ PatronLookup      = (*folio.PatronClient)(nil)
var _ CirculationLookup = (*folio.CirculationClient)(nil)
var _ InventoryLookup   = (*folio.InventoryClient)(nil)
var _ FeesOps           = (*folio.FeesClient)(nil)
