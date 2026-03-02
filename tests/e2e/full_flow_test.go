// +build e2e

package e2e

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/spokanepubliclibrary/fsip2/internal/folio/models"
	"github.com/spokanepubliclibrary/fsip2/tests/testutil"
)

// TestE2E_FullCheckoutFlow tests the complete checkout workflow
func TestE2E_FullCheckoutFlow(t *testing.T) {
	setup := NewE2ESetup(t)
	defer setup.Close(t)
	conn := setup.Connect(t)
	defer conn.Close()

	// Step 1: Send SC Status request
	t.Log("Step 1: SC Status")
	resp := setup.Exchange(t, conn, "990302.00\r")
	require.True(t, len(resp) >= 2 && resp[:2] == "98", "Expected ACS Status (98), got: %s", resp)

	// Step 2: Login
	t.Log("Step 2: Login")
	setup.Login(t, conn)

	// Step 3: Patron Status
	t.Log("Step 3: Patron Status")
	resp = setup.Exchange(t, conn, testutil.NewPatronStatusMessage("test-inst", "123456"))
	assert.True(t, len(resp) >= 2 && resp[:2] == "24", "Expected Patron Status Response (24), got: %s", resp)

	// Step 4: Checkout
	t.Log("Step 4: Checkout")
	resp = setup.Exchange(t, conn, testutil.NewCheckoutMessage("test-inst", "123456", "ITEM001"))
	assert.True(t, len(resp) >= 2 && resp[:2] == "12", "Expected Checkout Response (12), got: %s", resp)

	t.Log("Full checkout flow completed successfully")
}

// TestE2E_CheckinFlow tests the checkin workflow
func TestE2E_CheckinFlow(t *testing.T) {
	setup := NewE2ESetup(t)
	defer setup.Close(t)
	conn := setup.Connect(t)
	defer conn.Close()

	setup.Login(t, conn)

	t.Log("Testing Checkin")
	resp := setup.Exchange(t, conn, testutil.NewCheckinMessage("test-inst", "ITEM001"))
	assert.True(t, len(resp) >= 2 && resp[:2] == "10", "Expected Checkin Response (10), got: %s", resp)
}

// TestE2E_RenewalFlow tests the renewal workflow
func TestE2E_RenewalFlow(t *testing.T) {
	setup := NewE2ESetup(t)
	defer setup.Close(t)
	conn := setup.Connect(t)
	defer conn.Close()

	setup.Login(t, conn)

	t.Log("Testing Renewal")
	resp := setup.Exchange(t, conn, testutil.NewRenewalMessage("test-inst", "123456", "ITEM001"))
	assert.True(t, len(resp) >= 2 && resp[:2] == "30", "Expected Renew Response (30), got: %s", resp)
}

// TestE2E_MultipleConnections tests handling multiple simultaneous connections
func TestE2E_MultipleConnections(t *testing.T) {
	setup := NewE2ESetup(t)
	defer setup.Close(t)

	numConnections := 5
	connections := make([]net.Conn, numConnections)
	for i := 0; i < numConnections; i++ {
		connections[i] = setup.Connect(t)
		defer connections[i].Close()
	}

	for i, conn := range connections {
		t.Logf("Sending message from connection %d", i)
		resp := setup.Exchange(t, conn, "990302.00\r")
		assert.True(t, len(resp) >= 2 && resp[:2] == "98", "Connection %d: Expected ACS Status (98), got: %s", i, resp)
	}

	activeConns := setup.Server.GetActiveConnections()
	assert.Equal(t, int64(numConnections), activeConns, "Expected %d active connections", numConnections)
}

// TestE2E_PatronInformation tests patron information retrieval
func TestE2E_PatronInformation(t *testing.T) {
	setup := NewE2ESetup(t)
	defer setup.Close(t)

	setup.MockFolio.AddUser("PAT123", &models.User{
		ID:       "user-pat123",
		Username: "patron1",
		Barcode:  "PAT123",
		Active:   true,
		Personal: models.PersonalInfo{
			FirstName: "John",
			LastName:  "Doe",
			Email:     "john.doe@example.com",
		},
	})

	conn := setup.Connect(t)
	defer conn.Close()
	setup.Login(t, conn)

	t.Log("Testing Patron Information")
	resp := setup.Exchange(t, conn, testutil.NewPatronInformationMessage("test-inst", "PAT123"))
	assert.True(t, len(resp) >= 2 && resp[:2] == "64", "Expected Patron Information Response (64), got: %s", resp)
}

// TestE2E_CheckoutWithTitleFetching tests checkout with actual instance title retrieval
func TestE2E_CheckoutWithTitleFetching(t *testing.T) {
	setup := NewE2ESetup(t)
	defer setup.Close(t)

	setup.MockFolio.AddInstance("instance-test-title", &models.Instance{
		ID:    "instance-test-title",
		Title: "To Kill a Mockingbird",
	})
	setup.MockFolio.AddHoldings("holdings-test-title", &models.Holdings{
		ID:         "holdings-test-title",
		InstanceID: "instance-test-title",
	})
	setup.MockFolio.AddItem("BOOK123", &models.Item{
		ID:               "item-test-title",
		Barcode:          "BOOK123",
		HoldingsRecordID: "holdings-test-title",
		Status:           models.ItemStatus{Name: "Available"},
	})
	setup.MockFolio.AddUser("PATRON123", &models.User{
		ID:       "user-test-checkout",
		Username: "testpatron",
		Barcode:  "PATRON123",
		Active:   true,
		Personal: models.PersonalInfo{FirstName: "Jane", LastName: "Doe", Email: "jane.doe@example.com"},
	})

	conn := setup.Connect(t)
	defer conn.Close()
	setup.Login(t, conn)

	t.Log("Testing Checkout with Title Fetching")
	resp := setup.Exchange(t, conn, testutil.NewCheckoutMessage("test-inst", "PATRON123", "BOOK123"))
	assert.True(t, len(resp) >= 2 && resp[:2] == "12", "Expected Checkout Response (12), got: %s", resp)
	assert.Contains(t, resp, "AJTo Kill a Mockingbird", "Expected response to contain actual title")
	assert.NotContains(t, resp, "AJBOOK123", "Expected response NOT to use barcode as title when instance title is available")

	t.Log("Checkout with title fetching completed successfully")
}

// TestE2E_CheckoutWithLongTitle tests checkout with title truncation
func TestE2E_CheckoutWithLongTitle(t *testing.T) {
	setup := NewE2ESetup(t)
	defer setup.Close(t)

	longTitle := "This is a very long book title that exceeds the sixty character limit and should be truncated according to the SIP2 specification requirements"
	setup.MockFolio.AddInstance("instance-long-title", &models.Instance{
		ID:    "instance-long-title",
		Title: longTitle,
	})
	setup.MockFolio.AddHoldings("holdings-long-title", &models.Holdings{
		ID:         "holdings-long-title",
		InstanceID: "instance-long-title",
	})
	setup.MockFolio.AddItem("LONGBOOK", &models.Item{
		ID:               "item-long-title",
		Barcode:          "LONGBOOK",
		HoldingsRecordID: "holdings-long-title",
		Status:           models.ItemStatus{Name: "Available"},
	})
	setup.MockFolio.AddUser("PATRON456", &models.User{
		ID:       "user-long-title",
		Username: "testpatron2",
		Barcode:  "PATRON456",
		Active:   true,
		Personal: models.PersonalInfo{FirstName: "John", LastName: "Smith"},
	})

	conn := setup.Connect(t)
	defer conn.Close()
	setup.Login(t, conn)

	t.Log("Testing Checkout with Long Title (Truncation)")
	resp := setup.Exchange(t, conn, testutil.NewCheckoutMessage("test-inst", "PATRON456", "LONGBOOK"))
	assert.True(t, len(resp) >= 2 && resp[:2] == "12", "Expected Checkout Response (12), got: %s", resp)

	ajIndex := findFieldIndex(resp, "AJ")
	if ajIndex == -1 {
		t.Errorf("Expected response to contain AJ field, got: %s", resp)
	} else {
		nextDelimIndex := findNextDelimiter(resp, ajIndex+2)
		titleInResponse := resp[ajIndex+2 : nextDelimIndex]
		assert.LessOrEqual(t, len([]rune(titleInResponse)), 60,
			"Expected title to be truncated to 60 characters, got %d: %s", len([]rune(titleInResponse)), titleInResponse)
		t.Logf("Title in response (length %d): %s", len([]rune(titleInResponse)), titleInResponse)
	}

	t.Log("Checkout with long title completed successfully")
}

// TestE2E_CheckoutWithMissingInstance tests checkout fallback when instance is missing
func TestE2E_CheckoutWithMissingInstance(t *testing.T) {
	setup := NewE2ESetup(t)
	defer setup.Close(t)

	// Add test data WITHOUT instance (should fallback to barcode)
	setup.MockFolio.AddItem("NOINSTANCE", &models.Item{
		ID:      "item-no-instance",
		Barcode: "NOINSTANCE",
		Status:  models.ItemStatus{Name: "Available"},
		// No HoldingsRecordID - simulates missing instance chain
	})
	setup.MockFolio.AddUser("PATRON789", &models.User{
		ID:       "user-no-instance",
		Username: "testpatron3",
		Barcode:  "PATRON789",
		Active:   true,
		Personal: models.PersonalInfo{FirstName: "Alice", LastName: "Johnson"},
	})

	conn := setup.Connect(t)
	defer conn.Close()
	setup.Login(t, conn)

	t.Log("Testing Checkout Fallback (Missing Instance)")
	resp := setup.Exchange(t, conn, testutil.NewCheckoutMessage("test-inst", "PATRON789", "NOINSTANCE"))
	assert.True(t, len(resp) >= 2 && resp[:2] == "12", "Expected Checkout Response (12), got: %s", resp)
	assert.Contains(t, resp, "AJNOINSTANCE", "Expected response to contain 'AJNOINSTANCE' as fallback")

	t.Log("Checkout with missing instance (fallback) completed successfully")
}
