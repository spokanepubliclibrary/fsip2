package folio

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/spokanepubliclibrary/fsip2/internal/folio/models"
)

func TestLogin_Success(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/authn/login" {
			t.Errorf("Unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("Unexpected method: %s", r.Method)
		}

		// Verify request body
		var loginReq models.LoginRequest
		json.NewDecoder(r.Body).Decode(&loginReq)
		if loginReq.Username != "testuser" || loginReq.Password != "testpass" {
			t.Error("Invalid login credentials")
		}

		// Return successful login response
		response := models.LoginResponse{
			OkapiToken:   "test-token-123",
			AccessToken:  "test-token-123",
			RefreshToken: "refresh-token-456",
			ExpiresIn:    600,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewAuthClient(server.URL, "test-tenant", 100)
	ctx := context.Background()

	resp, err := client.Login(ctx, "testuser", "testpass")
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}

	if resp.OkapiToken != "test-token-123" {
		t.Errorf("Expected token test-token-123, got %s", resp.OkapiToken)
	}

	if resp.ExpiresAt.IsZero() {
		t.Error("ExpiresAt should be set")
	}
}

func TestLogin_InvalidCredentials(t *testing.T) {
	// Create mock server that returns 401
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid credentials"})
	}))
	defer server.Close()

	client := NewAuthClient(server.URL, "test-tenant", 100)
	ctx := context.Background()

	_, err := client.Login(ctx, "wronguser", "wrongpass")
	if err == nil {
		t.Error("Expected error for invalid credentials")
	}
}

func TestLogin_NoToken(t *testing.T) {
	// Create mock server that returns response without token
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{})
	}))
	defer server.Close()

	client := NewAuthClient(server.URL, "test-tenant", 100)
	ctx := context.Background()

	_, err := client.Login(ctx, "testuser", "testpass")
	if err == nil {
		t.Error("Expected error when no token received")
	}

	if !strings.Contains(err.Error(), "no token received") {
		t.Errorf("Expected 'no token received' error, got: %v", err)
	}
}

func TestLogin_UsesCachedToken(t *testing.T) {
	loginCount := 0

	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		loginCount++
		response := models.LoginResponse{
			OkapiToken:   "cached-token",
			AccessToken:  "cached-token",
			RefreshToken: "refresh-token",
			ExpiresIn:    600,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewAuthClient(server.URL, "test-tenant", 100)
	ctx := context.Background()

	// First login - should hit server
	_, err := client.Login(ctx, "testuser", "testpass")
	if err != nil {
		t.Fatalf("First login failed: %v", err)
	}

	if loginCount != 1 {
		t.Errorf("Expected 1 login request, got %d", loginCount)
	}

	// Second login - should use cache
	resp2, err := client.Login(ctx, "testuser", "testpass")
	if err != nil {
		t.Fatalf("Second login failed: %v", err)
	}

	if loginCount != 1 {
		t.Errorf("Expected still 1 login request (cached), got %d", loginCount)
	}

	if resp2.OkapiToken != "cached-token" {
		t.Errorf("Expected cached token, got %s", resp2.OkapiToken)
	}
}

func TestValidateToken_Valid(t *testing.T) {
	// Create mock server that returns user info
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users/_self" {
			t.Errorf("Unexpected path: %s", r.URL.Path)
		}

		user := models.User{
			ID:       "user-123",
			Username: "testuser",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(user)
	}))
	defer server.Close()

	client := NewAuthClient(server.URL, "test-tenant", 100)
	ctx := context.Background()

	valid, err := client.ValidateToken(ctx, "valid-token")
	if err != nil {
		t.Fatalf("ValidateToken failed: %v", err)
	}

	if !valid {
		t.Error("Expected token to be valid")
	}
}

func TestValidateToken_Invalid(t *testing.T) {
	// Create mock server that returns 401
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid token"})
	}))
	defer server.Close()

	client := NewAuthClient(server.URL, "test-tenant", 100)
	ctx := context.Background()

	valid, err := client.ValidateToken(ctx, "invalid-token")
	if err != nil {
		t.Fatalf("ValidateToken should not error: %v", err)
	}

	if valid {
		t.Error("Expected token to be invalid")
	}
}

func TestGetCachedToken(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := models.LoginResponse{
			OkapiToken:   "test-token",
			AccessToken:  "test-token",
			RefreshToken: "refresh-token",
			ExpiresIn:    600,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewAuthClient(server.URL, "test-tenant", 100)
	ctx := context.Background()

	// Login to cache token
	_, err := client.Login(ctx, "testuser", "testpass")
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}

	// Get cached token
	cached, found := client.GetCachedToken("testuser")
	if !found {
		t.Fatal("Expected to find cached token")
	}

	if cached.AccessToken != "test-token" {
		t.Errorf("Expected cached token test-token, got %s", cached.AccessToken)
	}
}

func TestInvalidateToken(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := models.LoginResponse{
			OkapiToken:  "test-token",
			AccessToken: "test-token",
			ExpiresIn:   600,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewAuthClient(server.URL, "test-tenant", 100)
	ctx := context.Background()

	// Login to cache token
	_, err := client.Login(ctx, "testuser", "testpass")
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}

	// Verify token is cached
	_, found := client.GetCachedToken("testuser")
	if !found {
		t.Fatal("Expected to find cached token")
	}

	// Invalidate token
	client.InvalidateToken("testuser")

	// Verify token is no longer cached
	_, found = client.GetCachedToken("testuser")
	if found {
		t.Error("Expected token to be invalidated")
	}
}

func TestClearCache(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := models.LoginResponse{
			OkapiToken:  "test-token",
			AccessToken: "test-token",
			ExpiresIn:   600,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewAuthClient(server.URL, "test-tenant", 100)
	ctx := context.Background()

	// Login multiple users
	client.Login(ctx, "user1", "pass1")
	client.Login(ctx, "user2", "pass2")

	// Clear cache
	client.ClearCache()

	// Verify all tokens are cleared
	_, found1 := client.GetCachedToken("user1")
	_, found2 := client.GetCachedToken("user2")

	if found1 || found2 {
		t.Error("Expected all tokens to be cleared")
	}
}

func TestLoginAndCache(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := models.LoginResponse{
			OkapiToken:  "test-token",
			AccessToken: "test-token",
			ExpiresIn:   600,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewAuthClient(server.URL, "test-tenant", 100)
	ctx := context.Background()

	token, err := client.LoginAndCache(ctx, "testuser", "testpass")
	if err != nil {
		t.Fatalf("LoginAndCache failed: %v", err)
	}

	if token != "test-token" {
		t.Errorf("Expected token test-token, got %s", token)
	}

	// Verify token is cached
	_, found := client.GetCachedToken("testuser")
	if !found {
		t.Error("Expected token to be cached")
	}
}

func TestGetOrRefreshToken_UsesCached(t *testing.T) {
	loginCount := 0

	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		loginCount++
		response := models.LoginResponse{
			OkapiToken:  "test-token",
			AccessToken: "test-token",
			ExpiresIn:   600,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewAuthClient(server.URL, "test-tenant", 100)
	ctx := context.Background()

	// First call - should login
	token1, err := client.GetOrRefreshToken(ctx, "testuser", "testpass")
	if err != nil {
		t.Fatalf("First GetOrRefreshToken failed: %v", err)
	}

	// Second call - should use cache
	token2, err := client.GetOrRefreshToken(ctx, "testuser", "testpass")
	if err != nil {
		t.Fatalf("Second GetOrRefreshToken failed: %v", err)
	}

	if token1 != token2 {
		t.Error("Expected same token from cache")
	}

	if loginCount != 1 {
		t.Errorf("Expected only 1 login call (used cache), got %d", loginCount)
	}
}

func TestGetOrRefreshToken_RefreshesExpired(t *testing.T) {
	loginCount := 0

	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		loginCount++
		response := models.LoginResponse{
			OkapiToken:  "refreshed-token",
			AccessToken: "refreshed-token",
			ExpiresIn:   1, // 1 second TTL; the 90s NeedsRefresh buffer treats it as already expired
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewAuthClient(server.URL, "test-tenant", 100)
	ctx := context.Background()

	// First call - should login
	_, err := client.GetOrRefreshToken(ctx, "testuser", "testpass")
	if err != nil {
		t.Fatalf("First GetOrRefreshToken failed: %v", err)
	}

	// Wait for token to expire
	time.Sleep(100 * time.Millisecond)

	// Second call - should refresh (login again)
	token2, err := client.GetOrRefreshToken(ctx, "testuser", "testpass")
	if err != nil {
		t.Fatalf("Second GetOrRefreshToken failed: %v", err)
	}

	if token2 != "refreshed-token" {
		t.Errorf("Expected refreshed token, got %s", token2)
	}

	if loginCount != 2 {
		t.Errorf("Expected 2 login calls (refreshed expired token), got %d", loginCount)
	}
}

func TestLogin_DefaultExpiration(t *testing.T) {
	// Create mock server that doesn't include expiresIn
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := models.LoginResponse{
			OkapiToken:  "test-token",
			AccessToken: "test-token",
			// ExpiresIn not set - should default to 600
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewAuthClient(server.URL, "test-tenant", 100)
	ctx := context.Background()

	resp, err := client.Login(ctx, "testuser", "testpass")
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}

	// ExpiresAt should be approximately 10 minutes from now
	expectedExpiry := time.Now().Add(10 * time.Minute)
	diff := resp.ExpiresAt.Sub(expectedExpiry).Abs()

	if diff > 5*time.Second {
		t.Errorf("Expected expiry around %v, got %v (diff: %v)", expectedExpiry, resp.ExpiresAt, diff)
	}
}
