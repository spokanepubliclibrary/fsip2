package mocks

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"time"

	"github.com/spokanepubliclibrary/fsip2/internal/folio/models"
	"github.com/spokanepubliclibrary/fsip2/tests/fixtures"
)

// tokenEntry tracks an issued access token for expiration-aware validation
type tokenEntry struct {
	Token     string
	Username  string
	SessionID int // Mock Keycloak session the token belongs to
	CreatedAt time.Time
	ExpiresAt time.Time
}

// refreshEntry tracks an issued refresh token and the access token issued with it
type refreshEntry struct {
	Username    string
	AccessToken string
	SessionID   int // Mock Keycloak session; carried over when the token is rotated
	ExpiresAt   time.Time
}

// logoutExpiredAccessGrace is how long past expiry the gateway still accepts an access token
// on /authn/logout. Measured on Trillium: ~30 s worked (script 21-A), ~5 min got 401 (V4b-4).
const logoutExpiredAccessGrace = 30 * time.Second

// Default token lifetimes when TokenLifetime / RefreshTokenLifetime are 0 (seconds)
const (
	defaultAccessTokenLifetime  = 3600
	defaultRefreshTokenLifetime = 86400
)

// FolioMockServer is a mock FOLIO API server for testing
type FolioMockServer struct {
	Server               *httptest.Server
	Users                map[string]*models.User
	Items                map[string]*models.Item
	Loans                map[string]*models.Loan
	Holdings             map[string]*models.Holdings             // id -> holdings
	Instances            map[string]*models.Instance             // id -> instance
	ManualBlocks         map[string][]*models.ManualBlock        // userID -> []block
	AutomatedBlocks      map[string]*models.AutomatedPatronBlock // userID -> blocks
	Accounts             map[string]*models.Account              // accountID -> account
	PatronPINs           map[string]string                       // userID -> valid PIN ("" = reject all PINs)
	Tokens               map[string]string                       // username -> token (legacy)
	tokenEntries         map[string]*tokenEntry                  // access token -> entry
	refreshEntries       map[string]*refreshEntry                // refresh token -> entry (rotated on use)
	TokenLifetime        int                                     // Access token lifetime in seconds (0 = default 3600)
	RefreshTokenLifetime int                                     // Refresh token lifetime in seconds (0 = default 86400)
	RejectLogins         bool                                    // When true, reject all login attempts (for testing renewal failure)
	RejectRefresh        bool                                    // When true, reject all /authn/refresh calls with 401
	LoginCount           int                                     // Number of successful logins (for verification)
	RefreshCount         int                                     // Number of successful refreshes
	DeprecatedLoginCount int                                     // Number of calls to the removed /authn/login endpoint
	authMu               sync.Mutex                              // Guards token bookkeeping and counters (handlers run concurrently)
	tokenSeq             int                                     // Makes issued tokens unique
	sessionSeq           int                                     // Mock Keycloak session IDs (one per login)
	issuedAccess         map[string]*tokenEntry                  // every access token ever issued (never deleted; for /authn/logout)
	issuedRefresh        map[string]int                          // every refresh token ever issued -> session ID (never deleted)
	LogoutCount          int                                     // /authn/logout calls that ended (or re-ended) one session: 204
	TrapTriggeredCount   int                                     // /authn/logout calls that ended EVERY session of a user (the blank-refresh trap)
	LogoutHeaderCount    int                                     // /authn/logout calls that carried an X-Okapi-Token header (fsip2 must never send one)
	LogoutTenants        []string                                // X-Okapi-Tenant of each /authn/logout call
	LogoutDelay          time.Duration                           // Delay before answering /authn/logout (timeout and async tests)
	RefreshStatus        int                                     // When non-zero, /authn/refresh answers with this status (e.g. 503)
	BulkPayCallCount     int                                     // Number of POST /accounts-bulk/pay calls received
	BulkPayRequests      []models.Payment                        // All bulk payment request bodies received
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
		refreshEntries:  make(map[string]*refreshEntry),
		issuedAccess:    make(map[string]*tokenEntry),
		issuedRefresh:   make(map[string]int),
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

	// /authn/* endpoints are public (no X-Okapi-Token required), as on Eureka and Okapi
	if strings.HasPrefix(r.URL.Path, "/authn/") {
		m.handleAuthn(w, r)
		return
	}

	// Check authentication for protected endpoints
	token := r.Header.Get("X-Okapi-Token")
	if token == "" || !m.isValidToken(token) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
		return
	}

	// Route requests
	switch {
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
	case strings.HasPrefix(r.URL.Path, "/accounts-bulk"):
		m.handleBulkPayAccounts(w, r)
	case strings.HasPrefix(r.URL.Path, "/accounts"):
		m.handleAccounts(w, r)
	case strings.HasPrefix(r.URL.Path, "/patron-pin/verify"):
		m.handlePatronPin(w, r)
	default:
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "not found"})
	}
}

// handleAuthn routes the /authn/* endpoints
func (m *FolioMockServer) handleAuthn(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/authn/login-with-expiry":
		m.handleLogin(w, r)
	case "/authn/refresh":
		m.handleRefresh(w, r)
	case "/authn/logout":
		m.handleLogout(w, r)
	default:
		// Includes the deprecated /authn/login, which fsip2 must never call
		if r.URL.Path == "/authn/login" {
			m.authMu.Lock()
			m.DeprecatedLoginCount++
			m.authMu.Unlock()
		}
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "not found"})
	}
}

// handleLogin handles /authn/login-with-expiry: tokens in Set-Cookie, expirations in the body
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

	m.authMu.Lock()
	defer m.authMu.Unlock()

	// Reject logins if configured (for testing renewal failure)
	if m.RejectLogins {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{"error": "service unavailable"})
		return
	}

	// Check credentials
	if loginReq.Username == "testuser" && loginReq.Password == "testpass" {
		m.LoginCount++
		m.sessionSeq++ // Each password login starts its own Keycloak session
		m.issueTokens(w, loginReq.Username, m.sessionSeq)
	} else {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid credentials"})
	}
}

// handleRefresh handles /authn/refresh: requires the folioRefreshToken cookie and rotates it
func (m *FolioMockServer) handleRefresh(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	m.authMu.Lock()
	defer m.authMu.Unlock()

	cookie, err := r.Cookie("folioRefreshToken")
	if err != nil || cookie.Value == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "no refresh token"})
		return
	}

	if m.RejectRefresh {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "refresh rejected"})
		return
	}
	if m.RefreshStatus != 0 {
		w.WriteHeader(m.RefreshStatus)
		json.NewEncoder(w).Encode(map[string]string{"error": "refresh failed"})
		return
	}

	// Trillium answers a reused (rotated) refresh token with 422 (test tenant, gate V3)
	entry, ok := m.refreshEntries[cookie.Value]
	if !ok || time.Now().After(entry.ExpiresAt) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid refresh token"})
		return
	}

	// Rotation: the old refresh token can never be used again
	delete(m.refreshEntries, cookie.Value)

	m.RefreshCount++
	m.issueTokens(w, entry.Username, entry.SessionID)
}

// handleLogout emulates /authn/logout as measured on Trillium (01b, PermissionTesting
// scripts 20, 21 and 22). The access token may come as the X-Okapi-Token header or the
// folioAccessToken cookie; the refresh token only as the folioRefreshToken cookie.
//
//	access + blank/whitespace/missing refresh -> TRAP: every session of the user ends, 204
//	no access token                           -> 422, nothing ends
//	access expired more than ~30 s            -> 401, nothing ends
//	refresh ever issued (current, rotated or already logged out) -> its session ends, 204
//	unknown / garbage refresh                 -> 422, nothing ends
func (m *FolioMockServer) handleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	m.authMu.Lock()
	delay := m.LogoutDelay
	m.authMu.Unlock()
	if delay > 0 {
		select {
		case <-time.After(delay):
		case <-r.Context().Done():
			return
		}
	}

	m.authMu.Lock()
	defer m.authMu.Unlock()

	m.LogoutTenants = append(m.LogoutTenants, r.Header.Get("X-Okapi-Tenant"))
	access := r.Header.Get("X-Okapi-Token")
	if access != "" {
		m.LogoutHeaderCount++
	} else if c, err := r.Cookie("folioAccessToken"); err == nil {
		access = c.Value
	}
	refresh := ""
	if c, err := r.Cookie("folioRefreshToken"); err == nil {
		refresh = c.Value
	}

	if access == "" {
		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}

	accessEntry, known := m.issuedAccess[access]
	if !known {
		// Legacy tokens seeded directly into Tokens have no expiry
		for username, t := range m.Tokens {
			if t == access {
				accessEntry, known = &tokenEntry{Token: t, Username: username, ExpiresAt: time.Now().Add(time.Hour)}, true
				break
			}
		}
	}
	if !known || time.Now().After(accessEntry.ExpiresAt.Add(logoutExpiredAccessGrace)) {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	if strings.TrimSpace(refresh) == "" {
		m.TrapTriggeredCount++
		m.invalidateUserLocked(accessEntry.Username)
		w.WriteHeader(http.StatusNoContent)
		return
	}

	sessionID, issued := m.issuedRefresh[refresh]
	if !issued {
		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}
	m.LogoutCount++
	m.invalidateSessionLocked(sessionID)
	w.WriteHeader(http.StatusNoContent)
}

// invalidateSessionLocked ends one mock Keycloak session. Caller must hold authMu.
func (m *FolioMockServer) invalidateSessionLocked(sessionID int) {
	for token, entry := range m.tokenEntries {
		if entry.SessionID == sessionID {
			delete(m.tokenEntries, token)
		}
	}
	for token, entry := range m.refreshEntries {
		if entry.SessionID == sessionID {
			delete(m.refreshEntries, token)
		}
	}
}

// invalidateUserLocked ends every mock Keycloak session of username. Caller must hold authMu.
func (m *FolioMockServer) invalidateUserLocked(username string) {
	for token, entry := range m.tokenEntries {
		if entry.Username == username {
			delete(m.tokenEntries, token)
		}
	}
	for token, entry := range m.refreshEntries {
		if entry.Username == username {
			delete(m.refreshEntries, token)
		}
	}
	delete(m.Tokens, username)
}

// issueTokens creates and records a new access/refresh token pair for a mock Keycloak
// session and writes a login-with-expiry style response. Caller must hold authMu.
func (m *FolioMockServer) issueTokens(w http.ResponseWriter, username string, sessionID int) {
	now := time.Now()
	accessLifetime := defaultAccessTokenLifetime
	if m.TokenLifetime > 0 {
		accessLifetime = m.TokenLifetime
	}
	refreshLifetime := defaultRefreshTokenLifetime
	if m.RefreshTokenLifetime > 0 {
		refreshLifetime = m.RefreshTokenLifetime
	}
	accessExp := now.Add(time.Duration(accessLifetime) * time.Second)
	refreshExp := now.Add(time.Duration(refreshLifetime) * time.Second)

	m.tokenSeq++ // UnixNano alone collides on coarse clocks (Windows)
	token := fmt.Sprintf("token-%s-%d-%d", username, now.UnixNano(), m.tokenSeq)
	refresh := "eyJtb2NrIjoicmVmcmVzaCJ9." + strings.ReplaceAll(token, ".", "-") + ".c2ln" // JWT-shaped (3 segments)

	entry := &tokenEntry{Token: token, Username: username, SessionID: sessionID, CreatedAt: now, ExpiresAt: accessExp}
	m.Tokens[username] = token
	m.tokenEntries[token] = entry
	m.issuedAccess[token] = entry
	m.refreshEntries[refresh] = &refreshEntry{Username: username, AccessToken: token, SessionID: sessionID, ExpiresAt: refreshExp}
	m.issuedRefresh[refresh] = sessionID

	http.SetCookie(w, &http.Cookie{Name: "folioAccessToken", Value: token, Path: "/", MaxAge: accessLifetime, HttpOnly: true})
	http.SetCookie(w, &http.Cookie{Name: "folioRefreshToken", Value: refresh, Path: "/authn", MaxAge: refreshLifetime, HttpOnly: true})
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{
		"accessTokenExpiration":  accessExp.UTC().Format(time.RFC3339Nano),
		"refreshTokenExpiration": refreshExp.UTC().Format(time.RFC3339Nano),
	})
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

// isValidToken checks if an access token was issued and has not expired
func (m *FolioMockServer) isValidToken(token string) bool {
	m.authMu.Lock()
	defer m.authMu.Unlock()

	if entry, ok := m.tokenEntries[token]; ok {
		return time.Now().Before(entry.ExpiresAt)
	}
	// Fallback to legacy token check (tests that seed Tokens directly)
	for _, t := range m.Tokens {
		if t == token {
			return true
		}
	}
	return false
}

// GetLoginCount returns the number of successful logins
func (m *FolioMockServer) GetLoginCount() int {
	m.authMu.Lock()
	defer m.authMu.Unlock()
	return m.LoginCount
}

// GetRefreshCount returns the number of successful /authn/refresh calls
func (m *FolioMockServer) GetRefreshCount() int {
	m.authMu.Lock()
	defer m.authMu.Unlock()
	return m.RefreshCount
}

// GetDeprecatedLoginCount returns the number of calls to the removed /authn/login endpoint
func (m *FolioMockServer) GetDeprecatedLoginCount() int {
	m.authMu.Lock()
	defer m.authMu.Unlock()
	return m.DeprecatedLoginCount
}

// SetTokenLifetime sets the access token lifetime in seconds for new tokens
func (m *FolioMockServer) SetTokenLifetime(seconds int) {
	m.authMu.Lock()
	defer m.authMu.Unlock()
	m.TokenLifetime = seconds
}

// SetRefreshTokenLifetime sets the refresh token lifetime in seconds for new tokens
func (m *FolioMockServer) SetRefreshTokenLifetime(seconds int) {
	m.authMu.Lock()
	defer m.authMu.Unlock()
	m.RefreshTokenLifetime = seconds
}

// SetRejectLogins configures whether to reject all login attempts
func (m *FolioMockServer) SetRejectLogins(reject bool) {
	m.authMu.Lock()
	defer m.authMu.Unlock()
	m.RejectLogins = reject
}

// SetRejectRefresh configures whether to reject all /authn/refresh calls
func (m *FolioMockServer) SetRejectRefresh(reject bool) {
	m.authMu.Lock()
	defer m.authMu.Unlock()
	m.RejectRefresh = reject
}

// GetLogoutCount returns the number of /authn/logout calls that ended one session (204)
func (m *FolioMockServer) GetLogoutCount() int {
	m.authMu.Lock()
	defer m.authMu.Unlock()
	return m.LogoutCount
}

// GetTrapTriggeredCount returns the number of /authn/logout calls that ended every session
// of a user. Any value above 0 means fsip2 sent a logout without a usable refresh token.
func (m *FolioMockServer) GetTrapTriggeredCount() int {
	m.authMu.Lock()
	defer m.authMu.Unlock()
	return m.TrapTriggeredCount
}

// GetLogoutHeaderCount returns the number of /authn/logout calls that carried X-Okapi-Token
func (m *FolioMockServer) GetLogoutHeaderCount() int {
	m.authMu.Lock()
	defer m.authMu.Unlock()
	return m.LogoutHeaderCount
}

// GetLogoutTenants returns the X-Okapi-Tenant header of each /authn/logout call, in order
func (m *FolioMockServer) GetLogoutTenants() []string {
	m.authMu.Lock()
	defer m.authMu.Unlock()
	return append([]string(nil), m.LogoutTenants...)
}

// SetLogoutDelay delays every /authn/logout answer by d
func (m *FolioMockServer) SetLogoutDelay(d time.Duration) {
	m.authMu.Lock()
	defer m.authMu.Unlock()
	m.LogoutDelay = d
}

// SetRefreshStatus makes /authn/refresh answer with status (0 restores normal behaviour)
func (m *FolioMockServer) SetRefreshStatus(status int) {
	m.authMu.Lock()
	defer m.authMu.Unlock()
	m.RefreshStatus = status
}

// ExpireAccessToken sets an issued access token's server-side expiry (e.g. in the past, to
// make the gateway answer 401 on /authn/logout as on Trillium)
func (m *FolioMockServer) ExpireAccessToken(token string, expiresAt time.Time) {
	m.authMu.Lock()
	defer m.authMu.Unlock()
	if entry, ok := m.issuedAccess[token]; ok {
		entry.ExpiresAt = expiresAt
	}
}

// IsRefreshTokenAlive reports whether refreshToken would still be accepted by /authn/refresh
// (the "alive" check used on the test tenant), without rotating it.
func (m *FolioMockServer) IsRefreshTokenAlive(refreshToken string) bool {
	m.authMu.Lock()
	defer m.authMu.Unlock()
	entry, ok := m.refreshEntries[refreshToken]
	return ok && time.Now().Before(entry.ExpiresAt)
}

// InvalidateAllTokens removes all access and refresh tokens (simulates FOLIO token revocation)
func (m *FolioMockServer) InvalidateAllTokens() {
	m.authMu.Lock()
	defer m.authMu.Unlock()
	m.tokenEntries = make(map[string]*tokenEntry)
	m.refreshEntries = make(map[string]*refreshEntry)
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

// handleGetAccounts handles GET /accounts?query=... matching by userId or id.
// It honours the CQL clause paymentStatus.name<>"Suspended claim returned" when present.
func (m *FolioMockServer) handleGetAccounts(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("query")
	excludeSuspended := strings.Contains(query, `paymentStatus.name<>"Suspended claim returned"`)

	var matchingAccounts []models.Account

	if idx := strings.Index(query, `userId=="`); idx >= 0 {
		rest := query[idx+9:] // skip userId=="
		if end := strings.Index(rest, `"`); end >= 0 {
			userID := rest[:end]
			for _, account := range m.Accounts {
				if account.UserID == userID {
					if excludeSuspended && account.PaymentStatus.Name == "Suspended claim returned" {
						continue
					}
					matchingAccounts = append(matchingAccounts, *account)
				}
			}
		}
	} else if idx := strings.Index(query, `id=="`); idx >= 0 {
		rest := query[idx+5:] // skip id=="
		if end := strings.Index(rest, `"`); end >= 0 {
			accountID := rest[:end]
			if account, ok := m.Accounts[accountID]; ok {
				if !excludeSuspended || account.PaymentStatus.Name != "Suspended claim returned" {
					matchingAccounts = append(matchingAccounts, *account)
				}
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

// handleBulkPayAccounts handles POST /accounts-bulk/pay
func (m *FolioMockServer) handleBulkPayAccounts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var payReq models.Payment
	if err := json.NewDecoder(r.Body).Decode(&payReq); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	m.BulkPayCallCount++
	m.BulkPayRequests = append(m.BulkPayRequests, payReq)

	w.WriteHeader(http.StatusCreated)
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
		Amount:          fmt.Sprintf("%s", payReq.Amount),
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
