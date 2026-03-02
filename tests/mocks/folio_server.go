package mocks

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"github.com/spokanepubliclibrary/fsip2/internal/folio/models"
	"github.com/spokanepubliclibrary/fsip2/tests/fixtures"
)

// tokenEntry tracks a token with its creation time for expiration-aware validation
type tokenEntry struct {
	Token     string
	CreatedAt time.Time
}

// FolioMockServer is a mock FOLIO API server for testing
type FolioMockServer struct {
	Server          *httptest.Server
	Users           map[string]*models.User
	Items           map[string]*models.Item
	Loans           map[string]*models.Loan
	Holdings        map[string]*models.Holdings         // id -> holdings
	Instances       map[string]*models.Instance         // id -> instance
	ManualBlocks    map[string][]*models.ManualBlock     // userID -> []block
	AutomatedBlocks map[string]*models.AutomatedPatronBlock // userID -> blocks
	Accounts        map[string]*models.Account          // accountID -> account
	PatronPINs      map[string]string                   // userID -> valid PIN ("" = reject all PINs)
	Tokens          map[string]string                   // username -> token (legacy)
	tokenEntries    map[string]*tokenEntry              // token -> entry with timestamp
	TokenLifetime   int                                 // ExpiresIn value to return (0 = use default 3600)
	RejectLogins    bool                                // When true, reject all login attempts (for testing refresh failure)
	LoginCount      int                                 // Number of successful logins (for verification)
}

// NewFolioMockServer creates a new mock FOLIO server
func NewFolioMockServer() *FolioMockServer {
	mock := &FolioMockServer{
		Users:           make(map[string]*models.User),
		Items:           make(map[string]*models.Item),
		Loans:           make(map[string]*models.Loan),
		Holdings:        make(map[string]*models.Holdings),
		Instances:       make(map[string]*models.Instance),
		ManualBlocks:    make(map[string][]*models.ManualBlock),
		AutomatedBlocks: make(map[string]*models.AutomatedPatronBlock),
		Accounts:        make(map[string]*models.Account),
		PatronPINs:      make(map[string]string),
		Tokens:          make(map[string]string),
		tokenEntries:    make(map[string]*tokenEntry),
	}

	// Create test server
	mock.Server = httptest.NewServer(http.HandlerFunc(mock.handler))

	// Add default test data
	mock.addDefaultTestData()

	return mock
}

// Close shuts down the mock server
func (m *FolioMockServer) Close() {
	m.Server.Close()
}

// GetURL returns the mock server URL
func (m *FolioMockServer) GetURL() string {
	return m.Server.URL
}

// handler routes requests to appropriate handlers
func (m *FolioMockServer) handler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Check authentication for protected endpoints
	if !strings.HasPrefix(r.URL.Path, "/authn/login") {
		token := r.Header.Get("X-Okapi-Token")
		if token == "" || !m.isValidToken(token) {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
			return
		}
	}

	// Route requests
	switch {
	case strings.HasPrefix(r.URL.Path, "/authn/login"):
		m.handleLogin(w, r)
	case strings.HasPrefix(r.URL.Path, "/users"):
		m.handleUsers(w, r)
	case strings.HasPrefix(r.URL.Path, "/circulation/check-out"):
		m.handleCheckout(w, r)
	case strings.HasPrefix(r.URL.Path, "/circulation/check-in"):
		m.handleCheckin(w, r)
	case strings.HasPrefix(r.URL.Path, "/circulation/renew"):
		m.handleRenew(w, r)
	case strings.HasPrefix(r.URL.Path, "/inventory/items"):
		m.handleItems(w, r)
	case strings.HasPrefix(r.URL.Path, "/holdings-storage/holdings/"):
		m.handleHoldings(w, r)
	case strings.HasPrefix(r.URL.Path, "/inventory/instances/"):
		m.handleInstances(w, r)
	case strings.HasPrefix(r.URL.Path, "/circulation/loans"):
		m.handleLoans(w, r)
	case strings.HasPrefix(r.URL.Path, "/manualblocks"):
		m.handleManualBlocks(w, r)
	case strings.HasPrefix(r.URL.Path, "/automated-patron-blocks/"):
		m.handleAutomatedBlocks(w, r)
	case strings.HasPrefix(r.URL.Path, "/accounts"):
		m.handleAccounts(w, r)
	case strings.HasPrefix(r.URL.Path, "/patron-pin/verify"):
		m.handlePatronPin(w, r)
	default:
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "not found"})
	}
}

// handleLogin handles authentication requests
func (m *FolioMockServer) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var loginReq struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&loginReq); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Reject logins if configured (for testing refresh failure)
	if m.RejectLogins {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{"error": "service unavailable"})
		return
	}

	// Check credentials
	if loginReq.Username == "testuser" && loginReq.Password == "testpass" {
		token := fmt.Sprintf("token-%s-%d", loginReq.Username, time.Now().UnixNano())
		m.Tokens[loginReq.Username] = token
		m.tokenEntries[token] = &tokenEntry{
			Token:     token,
			CreatedAt: time.Now(),
		}
		m.LoginCount++

		expiresIn := 3600
		if m.TokenLifetime > 0 {
			expiresIn = m.TokenLifetime
		}

		response := models.LoginResponse{
			AccessToken:  token,
			RefreshToken: "refresh-" + token,
			ExpiresIn:    expiresIn,
		}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(response)
	} else {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid credentials"})
	}
}

// handleUsers handles user queries
func (m *FolioMockServer) handleUsers(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("query")

	// Extract barcode from query
	var barcode string
	if strings.Contains(query, "barcode==") {
		parts := strings.Split(query, "barcode==")
		if len(parts) > 1 {
			barcode = strings.Trim(parts[1], "\"")
		}
	}

	if user, ok := m.Users[barcode]; ok {
		response := struct {
			Users        []*models.User `json:"users"`
			TotalRecords int            `json:"totalRecords"`
		}{
			Users:        []*models.User{user},
			TotalRecords: 1,
		}
		json.NewEncoder(w).Encode(response)
	} else {
		response := struct {
			Users        []*models.User `json:"users"`
			TotalRecords int            `json:"totalRecords"`
		}{
			Users:        []*models.User{},
			TotalRecords: 0,
		}
		json.NewEncoder(w).Encode(response)
	}
}

// handleCheckout handles checkout requests
func (m *FolioMockServer) handleCheckout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var checkoutReq struct {
		ItemBarcode  string `json:"itemBarcode"`
		UserBarcode  string `json:"userBarcode"`
		LoanDate     string `json:"loanDate"`
		ServicePoint string `json:"servicePointId"`
	}

	if err := json.NewDecoder(r.Body).Decode(&checkoutReq); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Validate that the item exists
	if _, ok := m.Items[checkoutReq.ItemBarcode]; !ok {
		w.WriteHeader(http.StatusUnprocessableEntity)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"errors": []map[string]string{
				{
					"message": fmt.Sprintf("Item with barcode %s not found", checkoutReq.ItemBarcode),
				},
			},
		})
		return
	}

	// Create loan
	now := time.Now()
	dueDate := now.Add(14 * 24 * time.Hour)
	loan := &models.Loan{
		ID:       fmt.Sprintf("loan-%d", time.Now().UnixNano()),
		ItemID:   checkoutReq.ItemBarcode,
		UserID:   checkoutReq.UserBarcode,
		LoanDate: &now,
		DueDate:  &dueDate,
		Status: models.LoanStatus{
			Name: "Open",
		},
	}

	m.Loans[loan.ID] = loan

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(loan)
}

// handleCheckin handles checkin requests
func (m *FolioMockServer) handleCheckin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var checkinReq struct {
		ItemBarcode  string `json:"itemBarcode"`
		CheckinDate  string `json:"checkinDate"`
		ServicePoint string `json:"servicePointId"`
	}

	if err := json.NewDecoder(r.Body).Decode(&checkinReq); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Return a closed loan
	now := time.Now()
	loan := &models.Loan{
		ID:      fmt.Sprintf("loan-%d", time.Now().UnixNano()),
		ItemID:  checkinReq.ItemBarcode,
		DueDate: &now,
		Status: models.LoanStatus{
			Name: "Closed",
		},
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(loan)
}

// handleRenew handles renewal requests
func (m *FolioMockServer) handleRenew(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var renewReq struct {
		ItemBarcode string `json:"itemBarcode"`
		UserBarcode string `json:"userBarcode"`
	}

	if err := json.NewDecoder(r.Body).Decode(&renewReq); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Return renewed loan
	now := time.Now()
	dueDate := now.Add(14 * 24 * time.Hour)
	loan := &models.Loan{
		ID:           fmt.Sprintf("loan-%d", time.Now().UnixNano()),
		ItemID:       renewReq.ItemBarcode,
		UserID:       renewReq.UserBarcode,
		DueDate:      &dueDate,
		RenewalCount: 1,
		Status: models.LoanStatus{
			Name: "Open",
		},
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(loan)
}

// handleItems handles item queries
func (m *FolioMockServer) handleItems(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("query")

	// Extract barcode from query
	var barcode string
	if strings.Contains(query, "barcode==") {
		parts := strings.Split(query, "barcode==")
		if len(parts) > 1 {
			barcode = strings.Trim(parts[1], "\"")
		}
	}

	if item, ok := m.Items[barcode]; ok {
		response := struct {
			Items        []*models.Item `json:"items"`
			TotalRecords int            `json:"totalRecords"`
		}{
			Items:        []*models.Item{item},
			TotalRecords: 1,
		}
		json.NewEncoder(w).Encode(response)
	} else {
		response := struct {
			Items        []*models.Item `json:"items"`
			TotalRecords int            `json:"totalRecords"`
		}{
			Items:        []*models.Item{},
			TotalRecords: 0,
		}
		json.NewEncoder(w).Encode(response)
	}
}

// handleLoans handles loan queries
func (m *FolioMockServer) handleLoans(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("query")

	var userID string
	if strings.Contains(query, "userId==") {
		parts := strings.Split(query, "userId==")
		if len(parts) > 1 {
			userID = strings.Trim(parts[1], "\"")
		}
	}

	loans := []*models.Loan{}
	for _, loan := range m.Loans {
		if loan.UserID == userID {
			loans = append(loans, loan)
		}
	}

	response := struct {
		Loans        []*models.Loan `json:"loans"`
		TotalRecords int            `json:"totalRecords"`
	}{
		Loans:        loans,
		TotalRecords: len(loans),
	}

	json.NewEncoder(w).Encode(response)
}

// isValidToken checks if a token is valid (and not expired when TokenLifetime is set)
func (m *FolioMockServer) isValidToken(token string) bool {
	// Check time-aware token entries first
	if entry, ok := m.tokenEntries[token]; ok {
		if m.TokenLifetime > 0 {
			// Check if token has expired
			expiry := entry.CreatedAt.Add(time.Duration(m.TokenLifetime) * time.Second)
			if time.Now().After(expiry) {
				return false
			}
		}
		return true
	}
	// Fallback to legacy token check
	for _, t := range m.Tokens {
		if t == token {
			return true
		}
	}
	return false
}

// GetLoginCount returns the number of successful logins
func (m *FolioMockServer) GetLoginCount() int {
	return m.LoginCount
}

// SetTokenLifetime sets the token lifetime in seconds for new tokens
func (m *FolioMockServer) SetTokenLifetime(seconds int) {
	m.TokenLifetime = seconds
}

// SetRejectLogins configures whether to reject all login attempts
func (m *FolioMockServer) SetRejectLogins(reject bool) {
	m.RejectLogins = reject
}

// InvalidateAllTokens removes all token entries (simulates FOLIO token revocation)
func (m *FolioMockServer) InvalidateAllTokens() {
	m.tokenEntries = make(map[string]*tokenEntry)
	m.Tokens = make(map[string]string)
}

// addDefaultTestData adds default test data, loading users and items from fixture files
// so that the mock server shares canonical test identifiers with the fixture library.
func (m *FolioMockServer) addDefaultTestData() {
	// Load default user from fixture.
	var user models.User
	if err := fixtures.LoadFixtureAs("users/valid_user.json", &user); err == nil {
		m.AddUser(user.Barcode, &user)
	}

	// Load default item from fixture.
	var item models.Item
	if err := fixtures.LoadFixtureAs("items/available_item.json", &item); err == nil {
		m.AddItem(item.Barcode, &item)
	}

	// Holdings and instances remain hard-coded (no fixture files for these yet).
	m.Holdings["holdings-001"] = &models.Holdings{ID: "holdings-001", InstanceID: "instance-001"}
	m.Instances["instance-001"] = &models.Instance{ID: "instance-001", Title: "Test Book Title"}
}

// AddUser adds a user to the mock server
func (m *FolioMockServer) AddUser(barcode string, user *models.User) {
	m.Users[barcode] = user
}

// AddItem adds an item to the mock server
func (m *FolioMockServer) AddItem(barcode string, item *models.Item) {
	m.Items[barcode] = item
}

// AddHoldings adds a holdings record to the mock server
func (m *FolioMockServer) AddHoldings(id string, holdings *models.Holdings) {
	m.Holdings[id] = holdings
}

// AddInstance adds an instance to the mock server
func (m *FolioMockServer) AddInstance(id string, instance *models.Instance) {
	m.Instances[id] = instance
}

// handleManualBlocks handles GET /manualblocks?query=userId==XXX
func (m *FolioMockServer) handleManualBlocks(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("query")

	var userID string
	if idx := strings.Index(query, `userId=="`); idx >= 0 {
		rest := query[idx+9:] // skip userId=="
		if end := strings.Index(rest, `"`); end >= 0 {
			userID = rest[:end]
		}
	} else if idx := strings.Index(query, "userId=="); idx >= 0 {
		rest := query[idx+8:] // skip userId==
		if end := strings.IndexAny(rest, " \t&|"); end >= 0 {
			userID = rest[:end]
		} else {
			userID = rest
		}
	}

	blocks := m.ManualBlocks[userID]
	manualBlocks := make([]models.ManualBlock, 0, len(blocks))
	for _, b := range blocks {
		if b != nil {
			manualBlocks = append(manualBlocks, *b)
		}
	}

	response := models.ManualBlockCollection{
		ManualBlocks: manualBlocks,
		TotalRecords: len(manualBlocks),
	}
	json.NewEncoder(w).Encode(response)
}

// handleAutomatedBlocks handles GET /automated-patron-blocks/{userID}
func (m *FolioMockServer) handleAutomatedBlocks(w http.ResponseWriter, r *http.Request) {
	// Extract userID from path: /automated-patron-blocks/{userID}
	pathParts := strings.Split(r.URL.Path, "/")
	userID := pathParts[len(pathParts)-1]

	if block, ok := m.AutomatedBlocks[userID]; ok {
		json.NewEncoder(w).Encode(block)
	} else {
		response := &models.AutomatedPatronBlock{
			AutomatedPatronBlocks: []models.AutomatedBlock{},
		}
		json.NewEncoder(w).Encode(response)
	}
}

// handleAccounts dispatches GET /accounts?query=... and POST /accounts/{id}/pay
func (m *FolioMockServer) handleAccounts(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/pay") {
		m.handlePayAccount(w, r)
		return
	}
	m.handleGetAccounts(w, r)
}

// handleGetAccounts handles GET /accounts?query=... matching by userId or id
func (m *FolioMockServer) handleGetAccounts(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("query")

	var matchingAccounts []models.Account

	if idx := strings.Index(query, `userId=="`); idx >= 0 {
		rest := query[idx+9:] // skip userId=="
		if end := strings.Index(rest, `"`); end >= 0 {
			userID := rest[:end]
			for _, account := range m.Accounts {
				if account.UserID == userID {
					matchingAccounts = append(matchingAccounts, *account)
				}
			}
		}
	} else if idx := strings.Index(query, `id=="`); idx >= 0 {
		rest := query[idx+5:] // skip id=="
		if end := strings.Index(rest, `"`); end >= 0 {
			accountID := rest[:end]
			if account, ok := m.Accounts[accountID]; ok {
				matchingAccounts = append(matchingAccounts, *account)
			}
		}
	}

	if matchingAccounts == nil {
		matchingAccounts = []models.Account{}
	}

	response := models.AccountCollection{
		Accounts:     matchingAccounts,
		TotalRecords: len(matchingAccounts),
	}
	json.NewEncoder(w).Encode(response)
}

// handlePayAccount handles POST /accounts/{id}/pay
func (m *FolioMockServer) handlePayAccount(w http.ResponseWriter, r *http.Request) {
	// Extract account ID from path: /accounts/{id}/pay
	withoutSuffix := strings.TrimSuffix(r.URL.Path, "/pay")
	pathParts := strings.Split(withoutSuffix, "/")
	accountID := pathParts[len(pathParts)-1]

	var payReq models.PaymentRequest
	if err := json.NewDecoder(r.Body).Decode(&payReq); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	response := models.PaymentResponse{
		AccountID:       accountID,
		Amount:          fmt.Sprintf("%.2f", payReq.Amount),
		RemainingAmount: "0.00",
		FeeFineActions:  []models.FeeFineAction{},
	}
	json.NewEncoder(w).Encode(response)
}

// handlePatronPin handles POST /patron-pin/verify
func (m *FolioMockServer) handlePatronPin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var pinReq struct {
		ID  string `json:"id"`
		PIN string `json:"pin"`
	}
	if err := json.NewDecoder(r.Body).Decode(&pinReq); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	validPIN, exists := m.PatronPINs[pinReq.ID]
	if exists && validPIN != "" && validPIN == pinReq.PIN {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Invalid PIN, user not found, or PIN is "" (disabled)
	w.WriteHeader(http.StatusUnprocessableEntity)
	json.NewEncoder(w).Encode(map[string]string{"error": "invalid pin"})
}

// handleHoldings handles holdings record requests
func (m *FolioMockServer) handleHoldings(w http.ResponseWriter, r *http.Request) {
	// Extract holdings ID from path: /holdings-storage/holdings/{id}
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 4 {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid path"})
		return
	}

	holdingsID := pathParts[3]

	if holdings, ok := m.Holdings[holdingsID]; ok {
		json.NewEncoder(w).Encode(holdings)
	} else {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "holdings not found"})
	}
}

// handleInstances handles instance requests
func (m *FolioMockServer) handleInstances(w http.ResponseWriter, r *http.Request) {
	// Extract instance ID from path: /inventory/instances/{id}
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 4 {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid path"})
		return
	}

	instanceID := pathParts[3]

	if instance, ok := m.Instances[instanceID]; ok {
		json.NewEncoder(w).Encode(instance)
	} else {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "instance not found"})
	}
}

// AddManualBlock appends a manual block for the given userID.
func (m *FolioMockServer) AddManualBlock(userID string, block *models.ManualBlock) {
	m.ManualBlocks[userID] = append(m.ManualBlocks[userID], block)
}

// AddAutomatedBlock sets the automated patron block for the given userID.
func (m *FolioMockServer) AddAutomatedBlock(userID string, block *models.AutomatedPatronBlock) {
	m.AutomatedBlocks[userID] = block
}

// AddAccount adds an account keyed by account.ID.
func (m *FolioMockServer) AddAccount(account *models.Account) {
	m.Accounts[account.ID] = account
}

// SetPatronPIN sets the valid PIN for a user. An empty string disables PIN verification for that user.
func (m *FolioMockServer) SetPatronPIN(userID, pin string) {
	m.PatronPINs[userID] = pin
}

// RemoveAllAccounts clears all accounts from the mock server.
func (m *FolioMockServer) RemoveAllAccounts() {
	m.Accounts = make(map[string]*models.Account)
}
