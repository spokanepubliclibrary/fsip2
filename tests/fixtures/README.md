# Test Fixtures

This directory contains JSON fixtures for testing the FSIP2 server. Fixtures provide consistent, reusable test data that simulates FOLIO API responses and database records.

## Directory Structure

```
tests/fixtures/
├── users/          # User/patron records
├── items/          # Item/bibliographic records
├── loans/          # Loan/circulation records
├── accounts/       # Fee/fine records
├── requests/       # Hold/recall/page requests
├── responses/      # FOLIO API response examples
├── loader.go       # Fixture loading utilities
└── README.md       # This file
```

## Available Fixtures

### Users (`users/`)

- **valid_user.json** - Active patron with full profile information
- **expired_user.json** - Patron with expired account (expirationDate in the past)
- **blocked_manual.json** - Patron with manual block on account
- **user_with_fees.json** - Patron with outstanding fees/fines
- **user_with_holds.json** - Patron with active hold requests
- **user_with_loans.json** - Patron with checked-out items

### Items (`items/`)

- **available_item.json** - Item available for checkout (status: Available)
- **checked_out_item.json** - Item currently checked out (status: Checked out)
- **on_hold_item.json** - Item awaiting pickup (status: Awaiting pickup)
- **missing_item.json** - Item marked as missing (status: Missing)

### Loans (`loans/`)

- **active_loan.json** - Current loan with future due date
- **overdue_loan.json** - Loan past its due date
- **renewed_loan.json** - Loan that has been renewed once
- **due_today_loan.json** - Loan due today
- **recalled_loan.json** - Loan that has been recalled by another patron

### Accounts (`accounts/`)

- **no_fees.json** - Empty fee/fine collection (patron owes nothing)
- **single_fee.json** - Single outstanding fee (e.g., overdue fine)
- **multiple_fees.json** - Multiple outstanding fees of different types
- **paid_fee.json** - Fee that has been fully paid

### Requests (`requests/`)

- **hold_request.json** - Active hold request (Open - Not yet filled)
- **recall_request.json** - Recall request on checked-out item
- **page_request.json** - Page request for non-circulating item
- **expired_request.json** - Expired/unfilled request (Closed - Unfilled)

### Responses (`responses/`)

- **login_success.json** - Successful authentication response with JWT token
- **login_failure.json** - Failed authentication (invalid credentials)
- **checkout_success.json** - Successful checkout response with loan details
- **checkout_item_unavailable.json** - Checkout failure (item not available)
- **checkin_success.json** - Successful checkin response
- **patron_info_full.json** - Complete patron information with loans, holds, and fees
- **item_info_response.json** - Item details with instance and holdings information

## Usage

### Loading Fixtures in Tests

Use the fixture loader utilities to load JSON data in your tests:

```go
import (
    "testing"
    "github.com/spokanepubliclibrary/fsip2/tests/fixtures"
    "github.com/spokanepubliclibrary/fsip2/internal/folio/models"
)

func TestSomething(t *testing.T) {
    // Load raw JSON
    data, err := fixtures.LoadFixture("users/valid_user.json")
    if err != nil {
        t.Fatal(err)
    }

    // Load and unmarshal into struct
    var user models.User
    err = fixtures.LoadFixtureAs("users/valid_user.json", &user)
    if err != nil {
        t.Fatal(err)
    }

    // Use user in test...
}
```

### Using Fixtures with Mock FOLIO Server

The mock FOLIO server can be configured to return fixture data:

```go
import (
    "testing"
    "github.com/spokanepubliclibrary/fsip2/tests/mocks"
    "github.com/spokanepubliclibrary/fsip2/tests/fixtures"
    "github.com/spokanepubliclibrary/fsip2/internal/folio/models"
)

func TestWithMockServer(t *testing.T) {
    // Create mock server
    mockServer := mocks.NewFolioMockServer()
    defer mockServer.Close()

    // Load fixture
    var user models.User
    fixtures.LoadFixtureAs("users/valid_user.json", &user)

    // Add to mock server
    mockServer.Users[user.Barcode] = &user

    // Now the mock server will return this user data
}
```

## Adding New Fixtures

### Guidelines

1. **Use realistic data** - Base fixtures on actual FOLIO API responses when possible
2. **Keep IDs unique** - Use UUIDs or unique identifiers across all fixtures
3. **Include metadata** - Add `createdDate`, `updatedDate` when applicable
4. **Document purpose** - Update this README when adding new fixtures
5. **Follow naming conventions** - Use descriptive names (e.g., `expired_user.json`, not `user2.json`)

### Creating a New Fixture

1. Create the JSON file in the appropriate subdirectory
2. Validate the JSON structure matches the corresponding FOLIO model
3. Add a description to this README
4. Test that the fixture loads correctly

Example:

```json
{
  "id": "unique-uuid-here",
  "barcode": "123456789",
  "active": true,
  "personal": {
    "lastName": "Smith",
    "firstName": "Jane"
  },
  "expirationDate": "2026-12-31T23:59:59.000Z"
}
```

## Testing Best Practices

### Use Fixtures For

- **Unit tests** - Test individual functions with known data
- **Integration tests** - Simulate FOLIO API responses
- **E2E tests** - Create realistic test scenarios

### Don't Use Fixtures For

- **Performance testing** - Generate data programmatically instead
- **Security testing** - Use dedicated security test data
- **Production data** - Never commit real patron/item data

## Fixture Maintenance

### Updating Fixtures

When FOLIO API models change:

1. Update affected fixture files to match new schema
2. Update this README if new fields are significant
3. Run tests to verify fixtures still load correctly
4. Update the fixture version/date in git commit message

### Versioning

Fixtures are versioned with the FSIP2 codebase. Breaking changes to FOLIO models should be documented in the main CHANGELOG.

## Reference

### FOLIO API Documentation

- [Users API](https://github.com/folio-org/mod-users)
- [Circulation API](https://github.com/folio-org/mod-circulation)
- [Inventory API](https://github.com/folio-org/mod-inventory)
- [Fees/Fines API](https://github.com/folio-org/mod-feesfines)

### SIP2 Protocol

- [3M SIP2 Specification](http://multimedia.3m.com/mws/media/355361O/sip2-protocol.pdf)

---

**Last Updated:** December 5, 2025
**Total Fixtures:** 25+ files
**Maintained By:** FSIP2 Development Team
