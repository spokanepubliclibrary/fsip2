package folio

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const (
	testLogoutAccess  = "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJhY2Nlc3MifQ.c2ln"
	testLogoutRefresh = "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJyZWZyZXNoIn0.c2ln"
)

// TestLogout_GuardsMakeNoHTTPCall covers S1 (blank/whitespace refresh), S1b (not JWT-shaped)
// and the empty access token guard: none of them may reach the server, because a logout
// with an access token and a blank refresh token ends every session of the user.
func TestLogout_GuardsMakeNoHTTPCall(t *testing.T) {
	tests := []struct {
		name    string
		access  string
		refresh string
		wantErr error
	}{
		{"empty refresh", testLogoutAccess, "", ErrNoRefreshToken},
		{"single space refresh", testLogoutAccess, " ", ErrNoRefreshToken},
		{"whitespace refresh", testLogoutAccess, "\t\r\n ", ErrNoRefreshToken},
		{"null literal", testLogoutAccess, "null", ErrInvalidRefreshToken},
		{"two segments", testLogoutAccess, "a.b", ErrInvalidRefreshToken},
		{"four segments", testLogoutAccess, "a.b.c.d", ErrInvalidRefreshToken},
		{"six segments", testLogoutAccess, "a.b.c.d.e.f", ErrInvalidRefreshToken},
		{"empty middle segment", testLogoutAccess, "a..c", ErrInvalidRefreshToken},
		{"JWE empty IV", testLogoutAccess, "h.k..ct.tag", ErrInvalidRefreshToken},
		{"JWE empty tag", testLogoutAccess, "h..iv.ct.", ErrInvalidRefreshToken},
		{"JWE empty header", testLogoutAccess, ".k.iv.ct.tag", ErrInvalidRefreshToken},
		{"all dots", testLogoutAccess, "....", ErrInvalidRefreshToken},
		{"trailing dot", testLogoutAccess, "a.b.", ErrInvalidRefreshToken},
		{"padded with spaces", testLogoutAccess, " " + testLogoutRefresh + " ", ErrInvalidRefreshToken},
		{"empty access", "", testLogoutRefresh, ErrNoAccessToken},
		{"whitespace access", "  ", testLogoutRefresh, ErrNoAccessToken},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var calls int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				atomic.AddInt32(&calls, 1)
				w.WriteHeader(http.StatusNoContent)
			}))
			defer server.Close()

			err := NewAuthClient(server.URL, "test-tenant").Logout(context.Background(), tt.access, tt.refresh)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("err = %v, want %v", err, tt.wantErr)
			}
			if n := atomic.LoadInt32(&calls); n != 0 {
				t.Errorf("expected no HTTP call, got %d", n)
			}
		})
	}
}

// TestLogout_SendsC5Shape checks the only request shape that ends exactly one session:
// both cookies, and no X-Okapi-Token header.
func TestLogout_SendsC5Shape(t *testing.T) {
	var gotRefresh, gotAccess, gotHeaderToken, gotTenant, gotPath, gotMethod string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotMethod = r.URL.Path, r.Method
		gotHeaderToken = r.Header.Get("X-Okapi-Token")
		gotTenant = r.Header.Get("X-Okapi-Tenant")
		if c, err := r.Cookie(RefreshTokenCookie); err == nil {
			gotRefresh = c.Value
		}
		if c, err := r.Cookie(AccessTokenCookie); err == nil {
			gotAccess = c.Value
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	err := NewAuthClient(server.URL, "test-tenant").Logout(context.Background(), testLogoutAccess, testLogoutRefresh)
	if err != nil {
		t.Fatalf("Logout() error = %v", err)
	}
	if gotMethod != http.MethodPost || gotPath != "/authn/logout" {
		t.Errorf("request = %s %s, want POST /authn/logout", gotMethod, gotPath)
	}
	if gotHeaderToken != "" {
		t.Error("X-Okapi-Token header must never be sent on logout")
	}
	if gotRefresh != testLogoutRefresh {
		t.Errorf("refresh cookie = %q, want %q", gotRefresh, testLogoutRefresh)
	}
	if gotAccess != testLogoutAccess {
		t.Errorf("access cookie = %q, want %q", gotAccess, testLogoutAccess)
	}
	if gotTenant != "test-tenant" {
		t.Errorf("X-Okapi-Tenant = %q, want test-tenant", gotTenant)
	}
}

func TestLogout_AcceptsEncryptedJWTShape(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	for name, token := range map[string]string{
		"JWE with encrypted key":      "h.k.iv.ct.tag",
		"JWE direct encryption (dir)": "eyJlbmMiOiJBMjU2R0NNIiwiYWxnIjoiZGlyIn0..aXY.Y3Q.dGFn",
	} {
		err := NewAuthClient(server.URL, "test-tenant").Logout(context.Background(), testLogoutAccess, token)
		if err != nil {
			t.Errorf("%s: refresh token should be accepted, got %v", name, err)
		}
	}
}

func TestTokenShape_RevealsStructureOnly(t *testing.T) {
	tests := map[string]string{
		"h..iv.ct.tag": "5 segments, length 12, empty: 2",
		"null":         "1 segments, length 4",
		"a b":          "1 segments, length 3, contains whitespace",
	}
	for token, want := range tests {
		if got := TokenShape(token); got != want {
			t.Errorf("TokenShape(%q) = %q, want %q", token, got, want)
		}
	}
	if shape := TokenShape(testLogoutRefresh); strings.Contains(shape, "eyJ") {
		t.Errorf("shape must not contain token content: %q", shape)
	}
}

func TestLogout_HTTPErrors(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusUnprocessableEntity, http.StatusInternalServerError} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(status)
		}))

		err := NewAuthClient(server.URL, "test-tenant").Logout(context.Background(), testLogoutAccess, testLogoutRefresh)
		var httpErr *HTTPError
		if !errors.As(err, &httpErr) || httpErr.StatusCode != status {
			t.Errorf("status %d: expected wrapped HTTPError, got %v", status, err)
		}
		server.Close()
	}
}

func TestLogout_Timeout(t *testing.T) {
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-release
	}))
	defer server.Close()
	defer close(release)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := NewAuthClient(server.URL, "test-tenant").Logout(ctx, testLogoutAccess, testLogoutRefresh)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("err = %v, want context.DeadlineExceeded", err)
	}
}
