package folio

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/spokanepubliclibrary/fsip2/internal/folio/models"
)

const (
	testAccessExp  = "2026-10-23T22:35:43Z"
	testRefreshExp = "2026-10-24T22:25:43Z"
)

// writeTokenResponse writes a login-with-expiry style response: tokens as cookies,
// expirations in the body. An empty token value omits that cookie.
func writeTokenResponse(w http.ResponseWriter, access, refresh, body string) {
	if access != "" {
		http.SetCookie(w, &http.Cookie{Name: AccessTokenCookie, Value: access, Path: "/"})
	}
	if refresh != "" {
		http.SetCookie(w, &http.Cookie{Name: RefreshTokenCookie, Value: refresh, Path: "/authn"})
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(body))
}

func validExpiryBody() string {
	return `{"accessTokenExpiration":"` + testAccessExp + `","refreshTokenExpiration":"` + testRefreshExp + `"}`
}

func TestLogin_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/authn/login-with-expiry" {
			t.Errorf("Unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("Unexpected method: %s", r.Method)
		}
		if r.Header.Get("X-Okapi-Token") != "" {
			t.Error("Login must not send X-Okapi-Token")
		}

		var loginReq models.LoginRequest
		json.NewDecoder(r.Body).Decode(&loginReq)
		if loginReq.Username != "testuser" || loginReq.Password != "testpass" {
			t.Error("Invalid login credentials")
		}

		writeTokenResponse(w, "access-123", "refresh-456", validExpiryBody())
	}))
	defer server.Close()

	client := NewAuthClient(server.URL, "test-tenant")
	resp, err := client.Login(context.Background(), "testuser", "testpass")
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}

	if resp.OkapiToken != "access-123" {
		t.Errorf("OkapiToken = %q, want access-123", resp.OkapiToken)
	}
	if resp.RefreshToken != "refresh-456" {
		t.Errorf("RefreshToken = %q, want refresh-456", resp.RefreshToken)
	}
	wantAccess, _ := time.Parse(time.RFC3339, testAccessExp)
	wantRefresh, _ := time.Parse(time.RFC3339, testRefreshExp)
	if !resp.ExpiresAt.Equal(wantAccess) {
		t.Errorf("ExpiresAt = %v, want server-provided %v", resp.ExpiresAt, wantAccess)
	}
	if !resp.RefreshTokenExpiration.Equal(wantRefresh) {
		t.Errorf("RefreshTokenExpiration = %v, want %v", resp.RefreshTokenExpiration, wantRefresh)
	}
}

func TestLogin_MissingCookie(t *testing.T) {
	tests := []struct {
		name          string
		access        string
		refresh       string
		wantErrSubstr string
	}{
		{"no access cookie", "", "refresh-456", AccessTokenCookie},
		{"no refresh cookie", "access-123", "", RefreshTokenCookie},
		{"no cookies", "", "", AccessTokenCookie},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				writeTokenResponse(w, tt.access, tt.refresh, validExpiryBody())
			}))
			defer server.Close()

			client := NewAuthClient(server.URL, "test-tenant")
			_, err := client.Login(context.Background(), "testuser", "testpass")
			if err == nil {
				t.Fatal("Expected error for missing cookie")
			}
			if !strings.Contains(err.Error(), tt.wantErrSubstr) {
				t.Errorf("error %q should name %s", err, tt.wantErrSubstr)
			}
		})
	}
}

func TestLogin_ExpirationProblems(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"unparseable expiration", `{"accessTokenExpiration":"soon","refreshTokenExpiration":"` + testRefreshExp + `"}`},
		{"missing access expiration", `{"refreshTokenExpiration":"` + testRefreshExp + `"}`},
		{"empty body", ``},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				writeTokenResponse(w, "access-123", "refresh-456", tt.body)
			}))
			defer server.Close()

			client := NewAuthClient(server.URL, "test-tenant")
			if _, err := client.Login(context.Background(), "testuser", "testpass"); err == nil {
				t.Error("Expected error, got nil")
			}
		})
	}
}

func TestLogin_HTTPErrors(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusUnprocessableEntity} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(status)
			json.NewEncoder(w).Encode(map[string]string{"error": "Invalid credentials"})
		}))

		client := NewAuthClient(server.URL, "test-tenant")
		_, err := client.Login(context.Background(), "wronguser", "wrongpass")
		server.Close()

		var httpErr *HTTPError
		if !errors.As(err, &httpErr) || httpErr.StatusCode != status {
			t.Errorf("status %d: expected wrapped HTTPError, got %v", status, err)
		}
	}
}

func TestRefresh_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/authn/refresh" {
			t.Errorf("Unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("X-Okapi-Token") != "" {
			t.Error("Refresh must not send X-Okapi-Token")
		}
		if r.ContentLength > 0 {
			t.Errorf("Refresh must send no body, got %d bytes", r.ContentLength)
		}
		cookie, err := r.Cookie(RefreshTokenCookie)
		if err != nil || cookie.Value != "old-refresh" {
			t.Errorf("Expected %s cookie old-refresh, got %v (%v)", RefreshTokenCookie, cookie, err)
		}
		writeTokenResponse(w, "new-access", "new-refresh", validExpiryBody())
	}))
	defer server.Close()

	client := NewAuthClient(server.URL, "test-tenant")
	resp, err := client.Refresh(context.Background(), "old-refresh")
	if err != nil {
		t.Fatalf("Refresh failed: %v", err)
	}
	if resp.OkapiToken != "new-access" || resp.RefreshToken != "new-refresh" {
		t.Errorf("tokens = (%q, %q), want (new-access, new-refresh)", resp.OkapiToken, resp.RefreshToken)
	}
	if resp.ExpiresAt.IsZero() {
		t.Error("ExpiresAt should be set")
	}
}

func TestRefresh_EmptyToken_NoHTTPCall(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
	}))
	defer server.Close()

	client := NewAuthClient(server.URL, "test-tenant")
	_, err := client.Refresh(context.Background(), "")
	if !errors.Is(err, ErrNoRefreshToken) {
		t.Errorf("err = %v, want ErrNoRefreshToken", err)
	}
	if atomic.LoadInt32(&calls) != 0 {
		t.Errorf("Expected no HTTP call, got %d", calls)
	}
}

func TestRefresh_Unauthorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	client := NewAuthClient(server.URL, "test-tenant")
	_, err := client.Refresh(context.Background(), "stale-refresh")

	var httpErr *HTTPError
	if !errors.As(err, &httpErr) || !httpErr.IsUnauthorized() {
		t.Errorf("expected wrapped 401 HTTPError, got %v", err)
	}
}

func TestRefresh_MissingCookie(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeTokenResponse(w, "new-access", "", validExpiryBody())
	}))
	defer server.Close()

	client := NewAuthClient(server.URL, "test-tenant")
	_, err := client.Refresh(context.Background(), "old-refresh")
	if err == nil || !strings.Contains(err.Error(), RefreshTokenCookie) {
		t.Errorf("expected error naming %s, got %v", RefreshTokenCookie, err)
	}
}

func TestExtractTokens_FirstNonEmptyWins(t *testing.T) {
	cookies := []*http.Cookie{
		{Name: "other", Value: "x"},
		{Name: AccessTokenCookie, Value: "a1"},
		{Name: AccessTokenCookie, Value: "a2"},
		{Name: RefreshTokenCookie, Value: "r1"},
	}
	access, refresh, err := extractTokens(cookies)
	if err != nil {
		t.Fatalf("extractTokens() error = %v", err)
	}
	if access != "a1" || refresh != "r1" {
		t.Errorf("got (%q, %q), want (a1, r1)", access, refresh)
	}
}
