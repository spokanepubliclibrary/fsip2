package folio

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestCapabilities_RoleSizes(t *testing.T) {
	// 22 direct + 8 transitive (verified on Trillium, T1/T2/T4 2026-10-27)
	assert.Len(t, RoleCapabilities(RoleSIP2Service), 30)
	assert.Len(t, RolePermissions(RoleSIP2Service), 30)
	assert.Equal(t, []string{"users_item.view", "users_item.edit"}, RoleCapabilities(RoleSIP2RollingRenewal))
	assert.Equal(t, []string{"users.item.get", "users.item.put"}, RolePermissions(RoleSIP2RollingRenewal))
}

func TestCapabilities_RowsAreWellFormed(t *testing.T) {
	for _, e := range Capabilities {
		switch e.Usage {
		case UsageDirect:
			assert.NotEmpty(t, e.Method, "direct row needs a method: %+v", e)
			assert.True(t, strings.HasPrefix(e.PathPattern, "/"), "direct row needs a path: %+v", e)
			assert.NotEmpty(t, e.OkapiPermission, "%+v", e)
			assert.NotEmpty(t, e.Capability, "%+v", e)
			assert.NotEmpty(t, e.Role, "%+v", e)
		case UsageTransitive:
			assert.Empty(t, e.Method, "%+v", e)
			assert.Equal(t, RoleSIP2Service, e.Role, "Decision 8: transitive rows stay in SIP2 Service: %+v", e)
		case UsagePublic:
			assert.True(t, strings.HasPrefix(e.PathPattern, "/authn/"), "%+v", e)
			assert.Empty(t, e.Capability, "%+v", e)
		case UsageFuture:
			assert.Empty(t, e.Role, "future rows are granted by no role: %+v", e)
		default:
			t.Errorf("unknown usage %q: %+v", e.Usage, e)
		}
	}
}

func TestLookupCapability(t *testing.T) {
	tests := []struct {
		method, path string
		wantCap      string
		wantRole     string
		wantFound    bool
	}{
		{"GET", "/users?query=barcode%3D%3D123", "users_collection.view", RoleSIP2Service, true},
		{"GET", "/users/abc-123", "users_item.view", RoleSIP2Service, true},
		{"PUT", "/users/abc-123", "users_item.edit", RoleSIP2RollingRenewal, true},
		{"POST", "/accounts/acc-1/pay", "feesfines_accounts_pay.execute", RoleSIP2Service, true},
		{"POST", "/accounts-bulk/pay", "feesfines_accounts-bulk_pay.execute", RoleSIP2Service, true},
		{"GET", "/accounts?query=x", "accounts_collection.view", RoleSIP2Service, true},
		{"GET", "/automated-patron-blocks/u1", "patron-blocks_automated-patron-blocks_collection.view", RoleSIP2Service, true},
		{"POST", "/patron-pin/verify", "patron-pin_validate.execute", RoleSIP2Service, true},
		{"post", "/circulation/check-out-by-barcode", "circulation_check-out-by-barcode.execute", RoleSIP2Service, true},
		{"POST", "/authn/login-with-expiry", "", "", true},
		// Future rows never match
		{"POST", "/circulation/requests", "", "", false},
		{"PUT", "/circulation/requests/r1", "", "", false},
		// Unknown
		{"DELETE", "/users/abc", "", "", false},
		{"GET", "/users/abc/extra", "", "", false},
		{"POST", "/accounts//pay", "", "", false}, // {id} needs a non-empty segment
	}
	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			e, found := LookupCapability(tt.method, tt.path)
			assert.Equal(t, tt.wantFound, found)
			assert.Equal(t, tt.wantCap, e.Capability)
			assert.Equal(t, tt.wantRole, e.Role)
		})
	}
}

func TestClient_403ReturnsPermissionError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]string{"code": "authorization_error", "message": "Access Denied"})
	}))
	defer server.Close()

	core, logs := observer.New(zap.WarnLevel)
	client := NewClient(server.URL, "test-tenant")
	client.logger = zap.New(core)

	err := client.Get(context.Background(), "/circulation/loans?query=userId%3D%3Du1", "tok", nil)
	require.Error(t, err)

	var permErr *PermissionError
	require.True(t, errors.As(err, &permErr))
	assert.True(t, IsPermissionError(err))
	assert.Equal(t, "GET", permErr.Method)
	assert.Equal(t, "/circulation/loans", permErr.Path, "query string (barcodes) must not be kept")
	assert.Equal(t, "circulation.loans.collection.get", permErr.OkapiPermission)
	assert.Equal(t, "circulation_loans_collection.view", permErr.Capability)
	assert.Equal(t, RoleSIP2Service, permErr.Role)
	assert.Contains(t, err.Error(), "circulation_loans_collection.view")
	assert.Contains(t, err.Error(), `"SIP2 Service"`)

	// The underlying HTTPError stays reachable (ExtractFolioErrorMessage relies on it)
	var httpErr *HTTPError
	require.True(t, errors.As(err, &httpErr))
	assert.True(t, httpErr.IsForbidden())
	assert.Equal(t, "Access Denied", httpErr.ParseErrorMessage())

	require.Equal(t, 1, logs.Len())
	fields := logs.All()[0].ContextMap()
	assert.Equal(t, "circulation_loans_collection.view", fields["eureka_capability"])
	assert.Equal(t, "circulation.loans.collection.get", fields["okapi_permission"])
	assert.Equal(t, RoleSIP2Service, fields["role"])
}

func TestClient_403UnmappedEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	err := NewClient(server.URL, "t").Get(context.Background(), "/not-in-table/1", "tok", nil)
	var permErr *PermissionError
	require.True(t, errors.As(err, &permErr))
	assert.Empty(t, permErr.Capability)
	assert.Contains(t, err.Error(), "not in the fsip2 capability table")
}

func TestClient_Non403IsPlainHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
	}))
	defer server.Close()

	err := NewClient(server.URL, "t").Get(context.Background(), "/users", "tok", nil)
	_, ok := err.(*HTTPError)
	assert.True(t, ok, "non-403 errors must stay *HTTPError for existing status checks")
	assert.False(t, IsPermissionError(err))
}

// Drift test: the lists in README.md and documentation/eureka_capabilities.md must equal
// the Capabilities table. Each list sits between
// <!-- fsip2-capabilities:<section> --> and <!-- /fsip2-capabilities -->.

var (
	docCapabilityRe = regexp.MustCompile(`\b[a-z][a-z0-9-]*(?:_[a-z0-9-]+)+\.(?:view|execute|edit|create|delete|manage)\b`)
	docPermissionRe = regexp.MustCompile("[`\"]([a-z][a-z0-9-]*(?:\\.[a-z0-9-]+)+)[`\"]")
	docSectionRe    = regexp.MustCompile(`(?s)<!-- fsip2-capabilities:([a-z-]+) -->(.*?)<!-- /fsip2-capabilities -->`)
)

type docSection struct {
	capabilities []string // nil: the section lists no capability names
	permissions  []string // nil: the section lists no Okapi permissions
}

func column(role string, usage CapabilityUsage, col func(CapabilityEntry) string) []string {
	seen := map[string]bool{}
	var out []string
	for _, e := range Capabilities {
		if e.Role == role && e.Usage == usage && !seen[col(e)] {
			seen[col(e)] = true
			out = append(out, col(e))
		}
	}
	return out
}

func expectedDocSections() map[string]docSection {
	capOf := func(e CapabilityEntry) string { return e.Capability }
	permOf := func(e CapabilityEntry) string { return e.OkapiPermission }
	return map[string]docSection{
		"service-direct":        {column(RoleSIP2Service, UsageDirect, capOf), column(RoleSIP2Service, UsageDirect, permOf)},
		"service-transitive":    {column(RoleSIP2Service, UsageTransitive, capOf), column(RoleSIP2Service, UsageTransitive, permOf)},
		"rolling-renewal":       {RoleCapabilities(RoleSIP2RollingRenewal), RolePermissions(RoleSIP2RollingRenewal)},
		"post-service":          {RoleCapabilities(RoleSIP2Service), nil},
		"post-rolling-renewal":  {RoleCapabilities(RoleSIP2RollingRenewal), nil},
		"okapi-service":         {nil, RolePermissions(RoleSIP2Service)},
		"okapi-rolling-renewal": {nil, RolePermissions(RoleSIP2RollingRenewal)},
	}
}

func uniqueSorted(in []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	sort.Strings(out)
	return out
}

// withOkapiAliases adds the Okapi name of each permission that has one.
func withOkapiAliases(perms []string) []string {
	out := append([]string(nil), perms...)
	for _, p := range perms {
		if alias, ok := OkapiPermissionAliases[p]; ok {
			out = append(out, alias)
		}
	}
	return out
}

func TestCapabilities_DocsMatchTable(t *testing.T) {
	docs := map[string][]string{
		"README.md": {"service-direct", "service-transitive", "rolling-renewal"},
		"documentation/eureka_capabilities.md": {"service-direct", "service-transitive", "rolling-renewal",
			"post-service", "post-rolling-renewal", "okapi-service", "okapi-rolling-renewal"},
	}
	// Docs that must list the Okapi alias next to each Trillium permission name
	listsAliases := map[string]bool{"README.md": true}
	expected := expectedDocSections()

	for doc, sections := range docs {
		t.Run(doc, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("..", "..", filepath.FromSlash(doc)))
			require.NoError(t, err)

			found := map[string]string{}
			for _, m := range docSectionRe.FindAllStringSubmatch(string(data), -1) {
				_, dup := found[m[1]]
				assert.False(t, dup, "section %q appears more than once", m[1])
				found[m[1]] = m[2]
			}

			for _, name := range sections {
				body, ok := found[name]
				if !assert.True(t, ok, "missing section <!-- fsip2-capabilities:%s -->", name) {
					continue
				}
				want := expected[name]

				gotCaps := uniqueSorted(docCapabilityRe.FindAllString(body, -1))
				assert.Equal(t, uniqueSorted(want.capabilities), gotCaps, "%s: capability names drifted from capabilities.go", name)

				var perms []string
				for _, m := range docPermissionRe.FindAllStringSubmatch(body, -1) {
					perms = append(perms, m[1])
				}
				wantPerms := want.permissions
				if listsAliases[doc] {
					wantPerms = withOkapiAliases(wantPerms)
				}
				assert.Equal(t, uniqueSorted(wantPerms), uniqueSorted(perms), "%s: Okapi permissions drifted from capabilities.go", name)
			}
		})
	}
}
