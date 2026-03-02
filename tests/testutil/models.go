package testutil

import (
	"fmt"

	"github.com/spokanepubliclibrary/fsip2/internal/folio/models"
	"github.com/spokanepubliclibrary/fsip2/tests/fixtures"
)

// LoadUser loads a user fixture by name (e.g. "valid_user", "expired_user", "blocked_manual").
func LoadUser(name string) (*models.User, error) {
	var u models.User
	return &u, fixtures.LoadFixtureAs("users/"+name+".json", &u)
}

func MustLoadUser(name string) *models.User {
	u, err := LoadUser(name)
	if err != nil {
		panic("testutil.MustLoadUser(" + name + "): " + err.Error())
	}
	return u
}

// DefaultUser returns an active patron from valid_user.json.
func DefaultUser() *models.User { return MustLoadUser("valid_user") }

// LoadItem loads an item fixture by name (e.g. "available_item", "checked_out_item").
func LoadItem(name string) (*models.Item, error) {
	var i models.Item
	return &i, fixtures.LoadFixtureAs("items/"+name+".json", &i)
}

func MustLoadItem(name string) *models.Item {
	i, err := LoadItem(name)
	if err != nil {
		panic("testutil.MustLoadItem(" + name + "): " + err.Error())
	}
	return i
}

// DefaultItem returns an available item from available_item.json.
func DefaultItem() *models.Item { return MustLoadItem("available_item") }

// LoadLoan loads a loan fixture by name (e.g. "active_loan", "overdue_loan").
func LoadLoan(name string) (*models.Loan, error) {
	var l models.Loan
	return &l, fixtures.LoadFixtureAs("loans/"+name+".json", &l)
}

func MustLoadLoan(name string) *models.Loan {
	l, err := LoadLoan(name)
	if err != nil {
		panic("testutil.MustLoadLoan(" + name + "): " + err.Error())
	}
	return l
}

// DefaultLoan returns an open loan from active_loan.json.
func DefaultLoan() *models.Loan { return MustLoadLoan("active_loan") }

// LoadAccount loads an account fixture collection by name and returns the first account.
// Fixture files use AccountCollection format (e.g. "single_fee", "multiple_fees").
func LoadAccount(name string) (*models.Account, error) {
	var col models.AccountCollection
	if err := fixtures.LoadFixtureAs("accounts/"+name+".json", &col); err != nil {
		return nil, err
	}
	if len(col.Accounts) == 0 {
		return nil, fmt.Errorf("testutil.LoadAccount(%s): fixture contains no accounts", name)
	}
	return &col.Accounts[0], nil
}

func MustLoadAccount(name string) *models.Account {
	a, err := LoadAccount(name)
	if err != nil {
		panic("testutil.MustLoadAccount(" + name + "): " + err.Error())
	}
	return a
}

// DefaultAccount returns a single open account from single_fee.json.
func DefaultAccount() *models.Account { return MustLoadAccount("single_fee") }

// LoadRequest loads a request fixture by name (e.g. "hold_request", "recall_request").
func LoadRequest(name string) (*models.Request, error) {
	var r models.Request
	return &r, fixtures.LoadFixtureAs("requests/"+name+".json", &r)
}

func MustLoadRequest(name string) *models.Request {
	r, err := LoadRequest(name)
	if err != nil {
		panic("testutil.MustLoadRequest(" + name + "): " + err.Error())
	}
	return r
}
