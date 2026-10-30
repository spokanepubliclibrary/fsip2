package types

import (
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/spokanepubliclibrary/fsip2/internal/config"
)

func TestTakeTokens_ReturnsAndClears(t *testing.T) {
	session := NewSession("test-session", newTestTenantConfig())
	session.SetAuthenticated("kiosk", "", "", "access-1", time.Now().Add(10*time.Minute))
	session.SetAuthCredentials("secret")
	accessExp := time.Now().Add(10 * time.Minute)
	refreshExp := time.Now().Add(24 * time.Hour)
	session.UpdateTokens("access-1", accessExp, "refresh-1", refreshExp)

	pair := session.TakeTokens()
	if pair.AccessToken != "access-1" || pair.RefreshToken != "refresh-1" {
		t.Errorf("pair tokens = (%q, %q), want (access-1, refresh-1)", pair.AccessToken, pair.RefreshToken)
	}
	if !pair.AccessExpiresAt.Equal(accessExp) || !pair.RefreshExpiresAt.Equal(refreshExp) {
		t.Error("pair expirations not returned")
	}
	if pair.OkapiURL != "http://okapi.example.com" || pair.OkapiTenant != "test" {
		t.Errorf("issuer = (%q, %q), want the session's tenant", pair.OkapiURL, pair.OkapiTenant)
	}

	if session.GetAuthToken() != "" || !session.GetTokenExpiresAt().IsZero() {
		t.Error("access token should be cleared")
	}
	if rt, exp := session.GetRefreshToken(); rt != "" || !exp.IsZero() {
		t.Error("refresh token should be cleared")
	}
	if !session.IsAuth() || !session.HasAuthCredentials() {
		t.Error("TakeTokens must not clear authentication state or credentials")
	}

	if second := session.TakeTokens(); !second.IsEmpty() || second.AccessToken != "" {
		t.Errorf("second TakeTokens should return an empty pair, got %+v", second.IsEmpty())
	}
}

func TestSwapTokens_ReturnsOldStoresNew(t *testing.T) {
	session := NewSession("test-session", newTestTenantConfig())
	session.UpdateTokens("access-1", time.Now().Add(time.Minute), "refresh-1", time.Now().Add(time.Hour))

	old := session.SwapTokens("access-2", time.Now().Add(time.Minute), "refresh-2", time.Now().Add(time.Hour))
	if old.AccessToken != "access-1" || old.RefreshToken != "refresh-1" {
		t.Errorf("old pair = (%q, %q), want (access-1, refresh-1)", old.AccessToken, old.RefreshToken)
	}
	if session.GetAuthToken() != "access-2" {
		t.Errorf("access token = %q, want access-2", session.GetAuthToken())
	}
	if rt, _ := session.GetRefreshToken(); rt != "refresh-2" {
		t.Errorf("refresh token = %q, want refresh-2", rt)
	}
}

func TestSwapTokens_EmptySessionReturnsEmptyPair(t *testing.T) {
	session := NewSession("test-session", newTestTenantConfig())
	old := session.SwapTokens("access-1", time.Now().Add(time.Minute), "refresh-1", time.Now().Add(time.Hour))
	if !old.IsEmpty() {
		t.Error("swapping into an empty session should return an empty pair")
	}
}

// TestTokenIssuer_RecordedAtStoreTime: a 93 Login can move the session to another tenant
// (UpdateTenant). The stored pair must keep the tenant that issued it.
func TestTokenIssuer_RecordedAtStoreTime(t *testing.T) {
	session := NewSession("test-session", newTestTenantConfig())
	session.UpdateTokens("access-a", time.Now().Add(time.Minute), "refresh-a", time.Now().Add(time.Hour))

	if session.HasTokensFromOtherTenant() {
		t.Error("pair issued by the current tenant reported as foreign")
	}

	session.UpdateTenant(&config.TenantConfig{Tenant: "b", OkapiURL: "http://b.example.com", OkapiTenant: "b"})
	if !session.HasTokensFromOtherTenant() {
		t.Error("after a tenant switch the stored pair should be reported as from another tenant")
	}

	pair := session.TakeTokens()
	if pair.OkapiURL != "http://okapi.example.com" || pair.OkapiTenant != "test" {
		t.Errorf("issuer = (%q, %q), want the original tenant", pair.OkapiURL, pair.OkapiTenant)
	}
	if session.HasTokensFromOtherTenant() {
		t.Error("no stored pair: should not report another tenant")
	}
}

func TestTokenIssuer_SameTenantDifferentURL(t *testing.T) {
	session := NewSession("test-session", newTestTenantConfig())
	session.UpdateTokens("a", time.Now().Add(time.Minute), "r", time.Now().Add(time.Hour))
	session.UpdateTenant(&config.TenantConfig{Tenant: "test-tenant", OkapiURL: "http://other.example.com", OkapiTenant: "test"})
	if !session.HasTokensFromOtherTenant() {
		t.Error("a different gateway URL should count as another issuer")
	}
}

func TestClear_ClearsIssuer(t *testing.T) {
	session := NewSession("test-session", newTestTenantConfig())
	session.UpdateTokens("a", time.Now().Add(time.Minute), "r", time.Now().Add(time.Hour))
	session.Clear()
	if pair := session.TakeTokens(); !pair.IsEmpty() || pair.OkapiURL != "" || pair.OkapiTenant != "" {
		t.Error("Clear should wipe the token pair and its issuer")
	}
}

// TestTakeAndSwap_Concurrent: every stored pair is handed out exactly once, whether by
// TakeTokens (close) or SwapTokens (93 / renewal). Run with -race.
func TestTakeAndSwap_Concurrent(t *testing.T) {
	session := NewSession("test-session", newTestTenantConfig())
	session.UpdateTokens("access-0", time.Now().Add(time.Minute), "refresh-0", time.Now().Add(time.Hour))

	const swaps = 50
	var mu sync.Mutex
	seen := map[string]int{}
	record := func(p TokenPair) {
		if p.IsEmpty() {
			return
		}
		mu.Lock()
		seen[p.RefreshToken]++
		mu.Unlock()
	}

	var wg sync.WaitGroup
	for i := 1; i <= swaps; i++ {
		wg.Add(2)
		go func(i int) {
			defer wg.Done()
			record(session.SwapTokens("access", time.Now().Add(time.Minute), "refresh-"+strconv.Itoa(i), time.Now().Add(time.Hour)))
		}(i)
		go func() {
			defer wg.Done()
			record(session.TakeTokens())
		}()
	}
	wg.Wait()
	record(session.TakeTokens())

	for token, n := range seen {
		if n != 1 {
			t.Errorf("pair %s handed out %d times, want 1", token, n)
		}
	}
	if len(seen) != swaps+1 {
		t.Errorf("handed out %d distinct pairs, want %d", len(seen), swaps+1)
	}
}
