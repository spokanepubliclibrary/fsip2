package folio

import "strings"

// FOLIO role names for the fsip2 kiosk user on Eureka. On Okapi the same groupings are
// permission sets (see documentation/eureka_capabilities.md, "Okapi equivalent").
const (
	// RoleSIP2Service is required on every kiosk (93 Login CN/CO) user.
	RoleSIP2Service = "SIP2 Service"
	// RoleSIP2RollingRenewal is an optional add-on, assigned in addition to RoleSIP2Service,
	// only where rolling renewals are enabled.
	RoleSIP2RollingRenewal = "SIP2 Rolling Renewal"
)

// CapabilityUsage says how fsip2 relates to a capability table row.
type CapabilityUsage string

const (
	// UsageDirect rows are endpoints fsip2 calls itself.
	UsageDirect CapabilityUsage = "direct"
	// UsageTransitive rows are permissions FOLIO modules use internally while serving an
	// fsip2 call (modulePermissions). Kept in RoleSIP2Service as a precaution.
	UsageTransitive CapabilityUsage = "transitive"
	// UsagePublic rows are endpoints that need no permission (the /authn/* calls).
	UsagePublic CapabilityUsage = "public"
	// UsageFuture rows belong to placeholder client methods (ErrNotImplemented). They are
	// granted by no role and are documented as "future" only.
	UsageFuture CapabilityUsage = "future"
)

// CapabilityEntry maps one FOLIO endpoint (or transitive permission) to the Okapi
// permission, the Eureka capability and the fsip2 role that grants it.
type CapabilityEntry struct {
	Method          string // HTTP method; empty for transitive rows
	PathPattern     string // Path without query; a "{name}" segment matches any one segment
	OkapiPermission string
	Capability      string // Eureka capability name, verified on Trillium (T1, 2026-10-27)
	Role            string // RoleSIP2Service, RoleSIP2RollingRenewal, or "" (public / future)
	Usage           CapabilityUsage
	Feature         string // SIP messages or client method that need it
}

// Capabilities is the single source of truth for what fsip2 needs from FOLIO. The README,
// documentation/eureka_capabilities.md and the mock server's enforcement mode are checked
// against it (capabilities_test.go). Target: Trillium (platform-lsp R1-2026-GA).
var Capabilities = []CapabilityEntry{
	// Public: no permission (permissionsRequired: [])
	{"POST", "/authn/login-with-expiry", "", "", "", UsagePublic, "93 login, patron password verification (Okapi only)"},
	{"POST", "/authn/refresh", "", "", "", UsagePublic, "token renewal"},
	{"POST", "/authn/logout", "", "", "", UsagePublic, "logout on disconnect"},

	// SIP2 Service: called directly by fsip2
	{"POST", "/circulation/check-out-by-barcode", "circulation.check-out-by-barcode.post", "circulation_check-out-by-barcode.execute", RoleSIP2Service, UsageDirect, "11 checkout"},
	{"POST", "/circulation/check-in-by-barcode", "circulation.check-in-by-barcode.post", "circulation_check-in-by-barcode.execute", RoleSIP2Service, UsageDirect, "09 checkin"},
	{"POST", "/circulation/renew-by-barcode", "circulation.renew-by-barcode.post", "circulation_renew-by-barcode.execute", RoleSIP2Service, UsageDirect, "29 renew"},
	{"POST", "/circulation/renew-by-id", "circulation.renew-by-id.post", "circulation_renew-by-id.execute", RoleSIP2Service, UsageDirect, "65 renew all"},
	{"GET", "/circulation/loans", "circulation.loans.collection.get", "circulation_loans_collection.view", RoleSIP2Service, UsageDirect, "63, 17, 65"},
	{"GET", "/circulation/requests", "circulation.requests.collection.get", "circulation_requests_collection.view", RoleSIP2Service, UsageDirect, "63, 17, 09"},
	{"GET", "/accounts", "accounts.collection.get", "accounts_collection.view", RoleSIP2Service, UsageDirect, "37, 63, 23"},
	{"POST", "/accounts/{id}/pay", "feesfines.accounts.pay.post", "feesfines_accounts_pay.execute", RoleSIP2Service, UsageDirect, "37 single payment"},
	{"POST", "/accounts-bulk/pay", "feesfines.accounts-bulk.pay.post", "feesfines_accounts-bulk_pay.execute", RoleSIP2Service, UsageDirect, "37 bulk payment"},
	{"GET", "/manualblocks", "manualblocks.collection.get", "manualblocks_collection.view", RoleSIP2Service, UsageDirect, "63, 23"},
	{"GET", "/automated-patron-blocks/{id}", "patron-blocks.automated-patron-blocks.collection.get", "patron-blocks_automated-patron-blocks_collection.view", RoleSIP2Service, UsageDirect, "63, 23"},
	{"GET", "/inventory/items", "inventory.items.collection.get", "inventory_items_collection.view", RoleSIP2Service, UsageDirect, "most messages"},
	{"GET", "/inventory/items/{id}", "inventory.items.item.get", "inventory_items_item.view", RoleSIP2Service, UsageDirect, "most messages (barcode lookup, then by id); 63, 29, 65 loans"},
	{"GET", "/inventory/instances/{id}", "inventory.instances.item.get", "inventory_instances_item.view", RoleSIP2Service, UsageDirect, "most messages"},
	{"GET", "/holdings-storage/holdings/{id}", "inventory-storage.holdings.item.get", "inventory-storage_holdings_item.view", RoleSIP2Service, UsageDirect, "most messages"},
	{"GET", "/locations/{id}", "inventory-storage.locations.item.get", "inventory-storage_locations_item.view", RoleSIP2Service, UsageDirect, "09"},
	{"GET", "/material-types/{id}", "inventory-storage.material-types.item.get", "inventory-storage_material-types_item.view", RoleSIP2Service, UsageDirect, "09"},
	{"GET", "/service-points/{id}", "inventory-storage.service-points.item.get", "inventory-storage_service-points_item.view", RoleSIP2Service, UsageDirect, "09, 17"},
	{"GET", "/users", "users.collection.get", "users_collection.view", RoleSIP2Service, UsageDirect, "most messages"},
	{"GET", "/users/{id}", "users.item.get", "users_item.view", RoleSIP2Service, UsageDirect, "09, 17, 63, 23"},
	{"GET", "/groups/{id}", "usergroups.item.get", "usergroups_item.view", RoleSIP2Service, UsageDirect, "63 patron group"},
	{"POST", "/patron-pin/verify", "patron-pin.validate", "patron-pin_validate.execute", RoleSIP2Service, UsageDirect, "PIN verification"},

	// SIP2 Service: used internally by FOLIO modules (Decision 8: keep all eight)
	{"", "", "manualblocks.item.get", "manualblocks_item.view", RoleSIP2Service, UsageTransitive, "mod-circulation check-out, renew"},
	{"", "", "usergroups.collection.get", "usergroups_collection.view", RoleSIP2Service, UsageTransitive, "mod-circulation check-out, check-in, loans, requests"},
	{"", "", "feefines.collection.get", "feefines_collection.view", RoleSIP2Service, UsageTransitive, "mod-circulation check-in, renew"},
	{"", "", "configuration.entries.collection.get", "configuration_entries_collection.view", RoleSIP2Service, UsageTransitive, "mod-login-keycloak login-with-expiry"},
	{"", "", "configuration.entries.item.get", "configuration_entries_item.view", RoleSIP2Service, UsageTransitive, "precaution"},
	{"", "", "accounts.item.get", "accounts_item.view", RoleSIP2Service, UsageTransitive, "precaution"},
	{"", "", "users-bl.item.get", "users-bl_item.view", RoleSIP2Service, UsageTransitive, "precaution"},
	{"", "", "search.instances.collection.get", "search_instances_collection.view", RoleSIP2Service, UsageTransitive, "precaution"},

	// SIP2 Rolling Renewal (optional add-on). users_item.view repeats the SIP2 Service grant
	// so the role is self-describing; lookups resolve GET /users/{id} to SIP2 Service first.
	{"GET", "/users/{id}", "users.item.get", "users_item.view", RoleSIP2RollingRenewal, UsageDirect, "rolling renewal (re-fetch)"},
	{"PUT", "/users/{id}", "users.item.put", "users_item.edit", RoleSIP2RollingRenewal, UsageDirect, "rolling renewal"},

	// Future: placeholder client methods (plan 02). Granted by no role.
	{"POST", "/circulation/requests", "circulation.requests.item.post", "circulation_requests_item.create", "", UsageFuture, "CreateRequest (SIP 15 hold +)"},
	{"GET", "/circulation/requests/{id}", "circulation.requests.item.get", "circulation_requests_item.view", "", UsageFuture, "CancelRequest (SIP 15 hold -)"},
	{"PUT", "/circulation/requests/{id}", "circulation.requests.item.put", "circulation_requests_item.edit", "", UsageFuture, "CancelRequest (SIP 15 hold -)"},
	{"POST", "/inventory/items/{id}/mark-{status}", "inventory.items.item.mark-{status}.post", "inventory_items_item_mark-{status}.execute", "", UsageFuture, "UpdateItemStatus (SIP 19)"},
	{"GET", "/search/instances", "search.instances.collection.get", "search_instances_collection.view", "", UsageFuture, "SearchInstances* (SIP 15 title-level holds)"},
	{"GET", "/users", "users.collection.get", "users_collection.view", "", UsageFuture, "GetUserByUsername (request cancel)"},
	{"POST", "/circulation/renew-by-barcode-all", "", "", "", UsageFuture, "RenewAll (no such endpoint in Trillium)"},
}

// OkapiPermissionAliases maps a permission name from the Trillium module descriptors to the
// name Okapi tenants grant for the same endpoint. The Okapi names are the ones verified in
// production; the Trillium names are the ones Eureka capabilities derive from (T1). The README
// lists both.
var OkapiPermissionAliases = map[string]string{
	"feesfines.accounts.pay.post":                          "accounts.pay.post",
	"patron-blocks.automated-patron-blocks.collection.get": "automated-patron-blocks.collection.get",
}

// LookupCapability returns the first direct or public entry matching method and path.
// The query string, if any, is ignored. Transitive and future rows never match.
func LookupCapability(method, path string) (CapabilityEntry, bool) {
	if i := strings.IndexByte(path, '?'); i >= 0 {
		path = path[:i]
	}
	for _, e := range Capabilities {
		if e.Usage != UsageDirect && e.Usage != UsagePublic {
			continue
		}
		if strings.EqualFold(e.Method, method) && pathMatches(e.PathPattern, path) {
			return e, true
		}
	}
	return CapabilityEntry{}, false
}

// RoleCapabilities returns the distinct capability names a role grants, in table order.
func RoleCapabilities(role string) []string {
	return roleColumn(role, func(e CapabilityEntry) string { return e.Capability })
}

// RolePermissions returns the distinct Okapi permissions a role grants, in table order.
func RolePermissions(role string) []string {
	return roleColumn(role, func(e CapabilityEntry) string { return e.OkapiPermission })
}

func roleColumn(role string, col func(CapabilityEntry) string) []string {
	seen := make(map[string]bool)
	var out []string
	for _, e := range Capabilities {
		if e.Role != role || e.Role == "" {
			continue
		}
		if v := col(e); v != "" && !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}

// pathMatches compares a pattern such as /accounts/{id}/pay with a concrete path,
// segment by segment. A segment that is exactly "{name}" matches any non-empty segment.
func pathMatches(pattern, path string) bool {
	ps := strings.Split(strings.Trim(pattern, "/"), "/")
	xs := strings.Split(strings.Trim(path, "/"), "/")
	if len(ps) != len(xs) {
		return false
	}
	for i, p := range ps {
		if strings.HasPrefix(p, "{") && strings.HasSuffix(p, "}") && !strings.Contains(p[1:], "{") {
			if xs[i] == "" {
				return false
			}
			continue
		}
		if p != xs[i] {
			return false
		}
	}
	return true
}
