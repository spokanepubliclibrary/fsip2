package mocks

import (
	"bytes"
	"net/http"
	"testing"
	"time"
)

type mockPair struct{ access, refresh string }

func mockLogin(t *testing.T, m *FolioMockServer) mockPair {
	t.Helper()
	resp, err := http.Post(m.GetURL()+"/authn/login-with-expiry", "application/json",
		bytes.NewBufferString(`{"username":"testuser","password":"testpass"}`))
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	defer resp.Body.Close()
	return pairFromCookies(t, resp)
}

func mockRefresh(t *testing.T, m *FolioMockServer, refresh string) (mockPair, int) {
	t.Helper()
	resp := mockPost(t, m, "/authn/refresh", "", &http.Cookie{Name: "folioRefreshToken", Value: refresh})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		return mockPair{}, resp.StatusCode
	}
	return pairFromCookies(t, resp), resp.StatusCode
}

func pairFromCookies(t *testing.T, resp *http.Response) mockPair {
	t.Helper()
	var p mockPair
	for _, c := range resp.Cookies() {
		switch c.Name {
		case "folioAccessToken":
			p.access = c.Value
		case "folioRefreshToken":
			p.refresh = c.Value
		}
	}
	if p.access == "" || p.refresh == "" {
		t.Fatalf("missing token cookies (status %d)", resp.StatusCode)
	}
	return p
}

func mockPost(t *testing.T, m *FolioMockServer, path, headerToken string, cookies ...*http.Cookie) *http.Response {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPost, m.GetURL()+path, nil)
	req.Header.Set("X-Okapi-Tenant", "test")
	if headerToken != "" {
		req.Header.Set("X-Okapi-Token", headerToken)
	}
	for _, c := range cookies {
		req.AddCookie(c)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	return resp
}

func logoutStatus(t *testing.T, m *FolioMockServer, headerToken string, cookies ...*http.Cookie) int {
	t.Helper()
	resp := mockPost(t, m, "/authn/logout", headerToken, cookies...)
	resp.Body.Close()
	return resp.StatusCode
}

func rtCookie(v string) *http.Cookie { return &http.Cookie{Name: "folioRefreshToken", Value: v} }
func atCookie(v string) *http.Cookie { return &http.Cookie{Name: "folioAccessToken", Value: v} }

// TestMockLogout_TrilliumMatrix reproduces the logout table measured on the test tenant
// (01b: V4, C0–C5, V4b-1…5).
func TestMockLogout_TrilliumMatrix(t *testing.T) {
	t.Run("C5 ends only the target session", func(t *testing.T) {
		m := NewFolioMockServer()
		defer m.Close()
		a, b := mockLogin(t, m), mockLogin(t, m)

		if s := logoutStatus(t, m, "", rtCookie(a.refresh), atCookie(a.access)); s != http.StatusNoContent {
			t.Fatalf("status = %d, want 204", s)
		}
		if m.IsRefreshTokenAlive(a.refresh) || m.isValidToken(a.access) {
			t.Error("target session should have ended")
		}
		if !m.IsRefreshTokenAlive(b.refresh) || !m.isValidToken(b.access) {
			t.Error("bystander session must stay alive")
		}
		if m.GetLogoutCount() != 1 || m.GetTrapTriggeredCount() != 0 {
			t.Errorf("logouts=%d traps=%d, want 1/0", m.GetLogoutCount(), m.GetTrapTriggeredCount())
		}
	})

	trapCases := []struct {
		name    string
		header  string
		cookies func(p mockPair) []*http.Cookie
	}{
		{"C1 header only", "header", func(p mockPair) []*http.Cookie { return nil }},
		{"C2 access cookie only", "", func(p mockPair) []*http.Cookie { return []*http.Cookie{atCookie(p.access)} }},
		{"C3 blank refresh + access cookie", "", func(p mockPair) []*http.Cookie {
			return []*http.Cookie{rtCookie(""), atCookie(p.access)}
		}},
		{"V4b-5 whitespace refresh + access cookie", "", func(p mockPair) []*http.Cookie {
			return []*http.Cookie{rtCookie(" "), atCookie(p.access)}
		}},
	}
	for _, tc := range trapCases {
		t.Run(tc.name+" triggers the trap", func(t *testing.T) {
			m := NewFolioMockServer()
			defer m.Close()
			a, b := mockLogin(t, m), mockLogin(t, m)
			header := ""
			if tc.header != "" {
				header = a.access
			}
			if s := logoutStatus(t, m, header, tc.cookies(a)...); s != http.StatusNoContent {
				t.Fatalf("status = %d, want 204", s)
			}
			if m.GetTrapTriggeredCount() != 1 {
				t.Errorf("traps = %d, want 1", m.GetTrapTriggeredCount())
			}
			if m.IsRefreshTokenAlive(a.refresh) || m.IsRefreshTokenAlive(b.refresh) {
				t.Error("the trap ends every session of the user")
			}
		})
	}

	nothingEnds := []struct {
		name    string
		status  int
		cookies func(p mockPair) []*http.Cookie
	}{
		{"V4 refresh only", http.StatusUnprocessableEntity, func(p mockPair) []*http.Cookie { return []*http.Cookie{rtCookie(p.refresh)} }},
		{"C0 nothing", http.StatusUnprocessableEntity, func(p mockPair) []*http.Cookie { return nil }},
		{"V4b-3 garbage refresh", http.StatusUnprocessableEntity, func(p mockPair) []*http.Cookie {
			return []*http.Cookie{rtCookie("eyJhbGciOiJub25lIn0.eyJzdWIiOiJ4In0.invalid"), atCookie(p.access)}
		}},
		{"unknown access token", http.StatusUnauthorized, func(p mockPair) []*http.Cookie {
			return []*http.Cookie{rtCookie(p.refresh), atCookie("not-issued")}
		}},
	}
	for _, tc := range nothingEnds {
		t.Run(tc.name+" ends nothing", func(t *testing.T) {
			m := NewFolioMockServer()
			defer m.Close()
			a := mockLogin(t, m)
			if s := logoutStatus(t, m, "", tc.cookies(a)...); s != tc.status {
				t.Errorf("status = %d, want %d", s, tc.status)
			}
			if !m.IsRefreshTokenAlive(a.refresh) {
				t.Error("session should still be alive")
			}
			if m.GetTrapTriggeredCount() != 0 || m.GetLogoutCount() != 0 {
				t.Error("no logout or trap expected")
			}
		})
	}

	t.Run("V4b-1 stale refresh ends that session's current tokens only", func(t *testing.T) {
		m := NewFolioMockServer()
		defer m.Close()
		a0, b := mockLogin(t, m), mockLogin(t, m)
		a1, status := mockRefresh(t, m, a0.refresh)
		if status != http.StatusCreated {
			t.Fatalf("refresh status = %d", status)
		}
		if s := logoutStatus(t, m, "", rtCookie(a0.refresh), atCookie(a0.access)); s != http.StatusNoContent {
			t.Fatalf("status = %d, want 204", s)
		}
		if m.IsRefreshTokenAlive(a1.refresh) {
			t.Error("the session's current (rotated) token should have ended")
		}
		if !m.IsRefreshTokenAlive(b.refresh) {
			t.Error("bystander must stay alive")
		}
	})

	t.Run("V4b-2 double logout returns 204 again", func(t *testing.T) {
		m := NewFolioMockServer()
		defer m.Close()
		a, b := mockLogin(t, m), mockLogin(t, m)
		logoutStatus(t, m, "", rtCookie(a.refresh), atCookie(a.access))
		if s := logoutStatus(t, m, "", rtCookie(a.refresh), atCookie(a.access)); s != http.StatusNoContent {
			t.Errorf("second logout status = %d, want 204", s)
		}
		if !m.IsRefreshTokenAlive(b.refresh) {
			t.Error("bystander must stay alive")
		}
	})

	t.Run("V4b-4 expired access token gets 401", func(t *testing.T) {
		m := NewFolioMockServer()
		defer m.Close()
		a := mockLogin(t, m)
		m.authMu.Lock()
		m.issuedAccess[a.access].ExpiresAt = time.Now().Add(-5 * time.Minute)
		m.authMu.Unlock()
		if s := logoutStatus(t, m, "", rtCookie(a.refresh), atCookie(a.access)); s != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", s)
		}
		if !m.IsRefreshTokenAlive(a.refresh) {
			t.Error("session should still be alive")
		}
	})

	t.Run("refresh after logout gets 422", func(t *testing.T) {
		m := NewFolioMockServer()
		defer m.Close()
		a := mockLogin(t, m)
		logoutStatus(t, m, "", rtCookie(a.refresh), atCookie(a.access))
		if _, status := mockRefresh(t, m, a.refresh); status != http.StatusUnprocessableEntity {
			t.Errorf("refresh status = %d, want 422", status)
		}
	})
}

func TestMockLogout_RecordsHeaderAndTenant(t *testing.T) {
	m := NewFolioMockServer()
	defer m.Close()
	a := mockLogin(t, m)
	logoutStatus(t, m, a.access, rtCookie(a.refresh))
	if m.GetLogoutHeaderCount() != 1 {
		t.Errorf("header count = %d, want 1", m.GetLogoutHeaderCount())
	}
	if tenants := m.GetLogoutTenants(); len(tenants) != 1 || tenants[0] != "test" {
		t.Errorf("tenants = %v, want [test]", tenants)
	}
}

func TestMockRefresh_SetRefreshStatus(t *testing.T) {
	m := NewFolioMockServer()
	defer m.Close()
	a := mockLogin(t, m)
	m.SetRefreshStatus(http.StatusServiceUnavailable)
	if _, status := mockRefresh(t, m, a.refresh); status != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", status)
	}
	if !m.IsRefreshTokenAlive(a.refresh) {
		t.Error("a failed refresh must not rotate the token")
	}
}
