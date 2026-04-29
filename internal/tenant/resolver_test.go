package tenant

import (
	"context"
	"testing"

	"github.com/spokanepubliclibrary/fsip2/internal/config"
)

// TestNewService tests service creation
func TestNewService(t *testing.T) {
	defaultCfg := &config.TenantConfig{
		Tenant:   "default",
		OkapiURL: "https://folio.example.com",
	}
	cfg := &config.Config{
		OkapiURL: "https://folio.example.com",
		Tenants: map[string]*config.TenantConfig{
			"default": defaultCfg,
		},
		TenantsOrdered: []*config.TenantConfig{defaultCfg},
	}

	svc := NewService(cfg)
	if svc == nil {
		t.Fatal("NewService() should not return nil")
	}

	if svc.defaultConfig == nil {
		t.Error("Default config should be set")
	}

	if svc.defaultConfig.Tenant != "default" {
		t.Errorf("Expected default tenant 'default', got '%s'", svc.defaultConfig.Tenant)
	}
}

// TestNewServiceDefaultTenantDeterminism proves that NewService picks the first
// unreferenced tenant in TenantsOrdered declaration order, not map iteration order.
func TestNewServiceDefaultTenantDeterminism(t *testing.T) {
	alphaCfg := &config.TenantConfig{Tenant: "alpha", OkapiURL: "https://alpha.example.com"}
	betaCfg := &config.TenantConfig{Tenant: "beta", OkapiURL: "https://beta.example.com"}
	gammaCfg := &config.TenantConfig{Tenant: "gamma", OkapiURL: "https://gamma.example.com"}

	cfg := &config.Config{
		Tenants: map[string]*config.TenantConfig{
			"alpha": alphaCfg,
			"beta":  betaCfg,
			"gamma": gammaCfg,
		},
		TenantsOrdered: []*config.TenantConfig{alphaCfg, betaCfg, gammaCfg},
		SCTenants: []config.SCTenantConfig{
			{Tenant: "alpha", Port: 6001},
			{Tenant: "beta", Port: 6002},
		},
	}

	svc := NewService(cfg)

	defaultTenant := svc.GetDefaultTenant()
	if defaultTenant == nil {
		t.Fatal("GetDefaultTenant() should not return nil")
	}
	if defaultTenant.Tenant != "gamma" {
		t.Errorf("Expected default tenant 'gamma' (first unreferenced in TenantsOrdered), got '%s'", defaultTenant.Tenant)
	}
}

// TestResolveAtConnectDefault tests connection resolution with default tenant
func TestResolveAtConnectDefault(t *testing.T) {
	defaultCfg := &config.TenantConfig{Tenant: "default"}
	cfg := &config.Config{
		Tenants: map[string]*config.TenantConfig{
			"default": defaultCfg,
		},
		TenantsOrdered: []*config.TenantConfig{defaultCfg},
	}

	svc := NewService(cfg)
	tenant, err := svc.ResolveAtConnect(context.Background(), "127.0.0.1", 12345, 6443)
	if err != nil {
		t.Fatalf("ResolveAtConnect() error = %v", err)
	}

	if tenant.Tenant != "default" {
		t.Errorf("Expected default tenant, got '%s'", tenant.Tenant)
	}
}

// TestResolveAtConnectWithIPResolver tests IP-based resolution using top-level SCTenants.
func TestResolveAtConnectWithIPResolver(t *testing.T) {
	defaultCfg := &config.TenantConfig{
		Tenant:      "default",
		OkapiURL:    "https://default.example.com",
		OkapiTenant: "default",
	}
	tenant1Cfg := &config.TenantConfig{
		Tenant:      "tenant1",
		OkapiURL:    "https://tenant1.example.com",
		OkapiTenant: "tenant1",
	}
	cfg := &config.Config{
		Tenants: map[string]*config.TenantConfig{
			"default": defaultCfg,
			"tenant1": tenant1Cfg,
		},
		TenantsOrdered: []*config.TenantConfig{defaultCfg, tenant1Cfg},
		SCTenants: []config.SCTenantConfig{
			{
				Tenant:   "tenant1",
				SCSubnet: "192.168.1.0/24",
			},
		},
	}

	svc := NewService(cfg)

	// Matching IP resolves to the looked-up TenantConfig with correct OkapiURL.
	result, err := svc.ResolveAtConnect(context.Background(), "192.168.1.100", 12345, 6443)
	if err != nil {
		t.Fatalf("ResolveAtConnect() error = %v", err)
	}
	if result.Tenant != "tenant1" {
		t.Errorf("Expected tenant1 for IP 192.168.1.100, got '%s'", result.Tenant)
	}
	if result.OkapiURL != "https://tenant1.example.com" {
		t.Errorf("Expected OkapiURL 'https://tenant1.example.com', got '%s'", result.OkapiURL)
	}

	// Non-matching IP falls back to default.
	result, err = svc.ResolveAtConnect(context.Background(), "10.0.0.1", 12345, 6443)
	if err != nil {
		t.Fatalf("ResolveAtConnect() error = %v", err)
	}
	if result.Tenant != "default" {
		t.Errorf("Expected default for IP 10.0.0.1, got '%s'", result.Tenant)
	}
}

// TestResolveAtConnectWithPortResolver tests port-based resolution using top-level SCTenants.
// Verifies that the resolver is registered with the correct looked-up TenantConfig.
func TestResolveAtConnectWithPortResolver(t *testing.T) {
	defaultCfg := &config.TenantConfig{
		Tenant:      "default",
		OkapiURL:    "https://default.example.com",
		OkapiTenant: "default",
	}
	tenant2Cfg := &config.TenantConfig{
		Tenant:      "tenant2",
		OkapiURL:    "https://tenant2.example.com",
		OkapiTenant: "tenant2",
	}
	cfg := &config.Config{
		Tenants: map[string]*config.TenantConfig{
			"default": defaultCfg,
			"tenant2": tenant2Cfg,
		},
		TenantsOrdered: []*config.TenantConfig{defaultCfg, tenant2Cfg},
		SCTenants: []config.SCTenantConfig{
			{
				Tenant: "tenant2",
				Port:   6444,
			},
		},
	}

	svc := NewService(cfg)

	// Matching port resolves to looked-up TenantConfig with correct OkapiURL/OkapiTenant.
	result, err := svc.ResolveAtConnect(context.Background(), "127.0.0.1", 12345, 6444)
	if err != nil {
		t.Fatalf("ResolveAtConnect() error = %v", err)
	}
	if result.Tenant != "tenant2" {
		t.Errorf("Expected tenant2 for port 6444, got '%s'", result.Tenant)
	}
	if result.OkapiURL != "https://tenant2.example.com" {
		t.Errorf("Expected OkapiURL 'https://tenant2.example.com', got '%s'", result.OkapiURL)
	}
	if result.OkapiTenant != "tenant2" {
		t.Errorf("Expected OkapiTenant 'tenant2', got '%s'", result.OkapiTenant)
	}

	// Non-matching port falls back to default.
	result, err = svc.ResolveAtConnect(context.Background(), "127.0.0.1", 12345, 6443)
	if err != nil {
		t.Fatalf("ResolveAtConnect() error = %v", err)
	}
	if result.Tenant != "default" {
		t.Errorf("Expected default for port 6443, got '%s'", result.Tenant)
	}
}

// TestSCTenantUnknownTenantSkipped verifies that an SCTenant referencing a name not present
// in cfg.Tenants is skipped gracefully — no panic, and no resolver is registered.
func TestSCTenantUnknownTenantSkipped(t *testing.T) {
	defaultCfg := &config.TenantConfig{
		Tenant:   "default",
		OkapiURL: "https://default.example.com",
	}
	cfg := &config.Config{
		Tenants: map[string]*config.TenantConfig{
			"default": defaultCfg,
		},
		TenantsOrdered: []*config.TenantConfig{defaultCfg},
		SCTenants: []config.SCTenantConfig{
			{
				Tenant:   "nonexistent",
				SCSubnet: "10.0.0.0/8",
				Port:     7000,
			},
		},
	}

	// Must not panic.
	svc := NewService(cfg)

	// No connect resolvers should have been registered for the unknown tenant.
	if count := svc.GetResolverCount(PhaseConnect); count != 0 {
		t.Errorf("Expected 0 connect resolvers for unknown SCTenant, got %d", count)
	}

	// Resolution still works and returns the default.
	result, err := svc.ResolveAtConnect(context.Background(), "10.0.0.1", 12345, 7000)
	if err != nil {
		t.Fatalf("ResolveAtConnect() error = %v", err)
	}
	if result.Tenant != "default" {
		t.Errorf("Expected default when SCTenant is unknown, got '%s'", result.Tenant)
	}
}

// TestGetDefaultTenant tests getting default tenant
func TestGetDefaultTenant(t *testing.T) {
	defaultCfg := &config.TenantConfig{Tenant: "default"}
	cfg := &config.Config{
		Tenants: map[string]*config.TenantConfig{
			"default": defaultCfg,
		},
		TenantsOrdered: []*config.TenantConfig{defaultCfg},
	}

	svc := NewService(cfg)
	defaultTenant := svc.GetDefaultTenant()

	if defaultTenant == nil {
		t.Fatal("GetDefaultTenant() should not return nil")
	}

	if defaultTenant.Tenant != "default" {
		t.Errorf("Expected default tenant, got '%s'", defaultTenant.Tenant)
	}
}

// TestGetTenantByName tests retrieving tenants by name
func TestGetTenantByName(t *testing.T) {
	tenant1Cfg := &config.TenantConfig{Tenant: "tenant1"}
	tenant2Cfg := &config.TenantConfig{Tenant: "tenant2"}
	cfg := &config.Config{
		Tenants: map[string]*config.TenantConfig{
			"tenant1": tenant1Cfg,
			"tenant2": tenant2Cfg,
		},
		TenantsOrdered: []*config.TenantConfig{tenant1Cfg, tenant2Cfg},
	}

	svc := NewService(cfg)

	// Test existing tenant
	tenant, ok := svc.GetTenantByName("tenant1")
	if !ok {
		t.Error("GetTenantByName() should return true for existing tenant")
	}

	if tenant.Tenant != "tenant1" {
		t.Errorf("Expected tenant1, got '%s'", tenant.Tenant)
	}

	// Test non-existing tenant
	_, ok = svc.GetTenantByName("nonexistent")
	if ok {
		t.Error("GetTenantByName() should return false for non-existing tenant")
	}
}

// TestGetAllTenants tests getting all tenant configurations
func TestGetAllTenants(t *testing.T) {
	tenant1Cfg := &config.TenantConfig{Tenant: "tenant1"}
	tenant2Cfg := &config.TenantConfig{Tenant: "tenant2"}
	cfg := &config.Config{
		Tenants: map[string]*config.TenantConfig{
			"tenant1": tenant1Cfg,
			"tenant2": tenant2Cfg,
		},
		TenantsOrdered: []*config.TenantConfig{tenant1Cfg, tenant2Cfg},
	}

	svc := NewService(cfg)
	allTenants := svc.GetAllTenants()

	if len(allTenants) != 2 {
		t.Errorf("Expected 2 tenants, got %d", len(allTenants))
	}

	if _, ok := allTenants["tenant1"]; !ok {
		t.Error("tenant1 should be in all tenants")
	}

	if _, ok := allTenants["tenant2"]; !ok {
		t.Error("tenant2 should be in all tenants")
	}
}

// TestGetResolverCount tests getting resolver counts using top-level SCTenants.
func TestGetResolverCount(t *testing.T) {
	defaultCfg := &config.TenantConfig{Tenant: "default"}
	tenant1Cfg := &config.TenantConfig{
		Tenant:      "tenant1",
		OkapiURL:    "https://tenant1.example.com",
		OkapiTenant: "tenant1",
	}
	cfg := &config.Config{
		Tenants: map[string]*config.TenantConfig{
			"default": defaultCfg,
			"tenant1": tenant1Cfg,
		},
		TenantsOrdered: []*config.TenantConfig{defaultCfg, tenant1Cfg},
		SCTenants: []config.SCTenantConfig{
			{
				Tenant:           "tenant1",
				SCSubnet:         "192.168.1.0/24",
				Port:             6444,
				LocationCodes:    []string{"MAIN"},
				UsernamePrefixes: []string{"lib1_"},
			},
		},
	}

	svc := NewService(cfg)

	// Should have 2 connect resolvers (IP and port)
	connectCount := svc.GetResolverCount(PhaseConnect)
	if connectCount != 2 {
		t.Errorf("Expected 2 connect resolvers, got %d", connectCount)
	}

	// Should have 2 login resolvers (location and username)
	loginCount := svc.GetResolverCount(PhaseLogin)
	if loginCount != 2 {
		t.Errorf("Expected 2 login resolvers, got %d", loginCount)
	}
}

// TestResolverPriority tests that resolvers are sorted by priority using top-level SCTenants.
func TestResolverPriority(t *testing.T) {
	defaultCfg := &config.TenantConfig{Tenant: "default"}
	tenant1Cfg := &config.TenantConfig{
		Tenant:   "tenant1",
		OkapiURL: "https://tenant1.example.com",
	}
	cfg := &config.Config{
		Tenants: map[string]*config.TenantConfig{
			"default": defaultCfg,
			"tenant1": tenant1Cfg,
		},
		TenantsOrdered: []*config.TenantConfig{defaultCfg, tenant1Cfg},
		SCTenants: []config.SCTenantConfig{
			{
				Tenant:   "tenant1",
				SCSubnet: "192.168.1.0/24",
				Port:     6444,
			},
		},
	}

	svc := NewService(cfg)

	// IP resolver should have higher priority than port resolver
	if len(svc.connectResolvers) >= 2 {
		firstResolver := svc.connectResolvers[0]
		if firstResolver.Priority() < svc.connectResolvers[1].Priority() {
			t.Error("Resolvers should be sorted by priority (highest first)")
		}
	}
}

// TestNoTenantConfiguration tests behavior when no tenants are configured
func TestNoTenantConfiguration(t *testing.T) {
	cfg := &config.Config{
		Tenants: map[string]*config.TenantConfig{},
	}

	svc := NewService(cfg)

	// Should return error when no default tenant
	_, err := svc.ResolveAtConnect(context.Background(), "127.0.0.1", 12345, 6443)
	if err == nil {
		t.Error("ResolveAtConnect() should return error when no tenants configured")
	}
}

// TestResolutionPhaseString tests the String() method on ResolutionPhase
func TestResolutionPhaseString(t *testing.T) {
	testCases := []struct {
		phase    ResolutionPhase
		expected string
	}{
		{PhaseConnect, "CONNECT"},
		{PhaseLogin, "LOGIN"},
		{ResolutionPhase(99), "UNKNOWN"},
	}

	for _, tc := range testCases {
		result := tc.phase.String()
		if result != tc.expected {
			t.Errorf("ResolutionPhase(%d).String() = %q, expected %q", tc.phase, result, tc.expected)
		}
	}
}

// TestBaseResolverName tests the Name() method on BaseResolver
func TestBaseResolverName(t *testing.T) {
	br := NewBaseResolver("TestName", PhaseConnect, 50)
	if br.Name() != "TestName" {
		t.Errorf("Name() = %q, expected %q", br.Name(), "TestName")
	}
}

// TestByPrioritySwap tests the Swap method on ByPriority
func TestByPrioritySwap(t *testing.T) {
	tenantCfg := &config.TenantConfig{Tenant: "t"}
	res1 := NewIPResolver("192.168.1.0/24", tenantCfg)
	res1.BaseResolver = NewBaseResolver("r1", PhaseConnect, 10)
	res2 := NewIPResolver("10.0.0.0/8", tenantCfg)
	res2.BaseResolver = NewBaseResolver("r2", PhaseConnect, 20)

	bp := ByPriority{res1, res2}
	bp.Swap(0, 1)

	if bp[0].Name() != "r2" {
		t.Errorf("After Swap: bp[0].Name() = %q, expected %q", bp[0].Name(), "r2")
	}
	if bp[1].Name() != "r1" {
		t.Errorf("After Swap: bp[1].Name() = %q, expected %q", bp[1].Name(), "r1")
	}
}

// TestNewServiceCatchAllSCTenant verifies that when ALL tenants are referenced by scTenant entries,
// the one with no routing fields (empty SCSubnet, zero Port, no LocationCodes, no UsernamePrefixes)
// is picked as the default via the explicit catch-all logic (Priority 1 in NewService).
func TestNewServiceCatchAllSCTenant(t *testing.T) {
	defaultCfg := &config.TenantConfig{
		Tenant:   "default",
		OkapiURL: "https://default.example.com",
	}
	institutionCfg := &config.TenantConfig{
		Tenant:   "institution-test",
		OkapiURL: "https://institution.example.com",
	}

	cfg := &config.Config{
		Tenants: map[string]*config.TenantConfig{
			"default":          defaultCfg,
			"institution-test": institutionCfg,
		},
		TenantsOrdered: []*config.TenantConfig{defaultCfg, institutionCfg},
		SCTenants: []config.SCTenantConfig{
			// "default" has no routing fields — explicit catch-all
			{Tenant: "default"},
			// "institution-test" has a routing rule
			{Tenant: "institution-test", Port: 6444},
		},
	}

	svc := NewService(cfg)

	defaultTenant := svc.GetDefaultTenant()
	if defaultTenant == nil {
		t.Fatal("GetDefaultTenant() should not return nil")
	}
	if defaultTenant.Tenant != "default" {
		t.Errorf("Expected default tenant 'default' (explicit catch-all scTenant), got '%s'", defaultTenant.Tenant)
	}
	if defaultTenant.Tenant == "institution-test" {
		t.Error("Default tenant must not be 'institution-test'")
	}
}

// TestResolveComplete tests the holistic login-time tenant resolution that walks
// scTenants in declaration order and applies all present rules as conjunctive filters.
func TestResolveComplete(t *testing.T) {
	// Shared tenant configs used across cases.
	snapshotCfg := &config.TenantConfig{
		Tenant:   "snapshot",
		OkapiURL: "https://snapshot.example.com",
	}
	institutionCfg := &config.TenantConfig{
		Tenant:   "institution-test",
		OkapiURL: "https://institution.example.com",
	}
	userAuthCfg := &config.TenantConfig{
		Tenant:   "user-auth",
		OkapiURL: "https://user-auth.example.com",
	}
	defaultCfg := &config.TenantConfig{
		Tenant:   "default",
		OkapiURL: "https://default.example.com",
	}

	// buildStandardCfg builds the four-tenant config with snapshot declared first.
	buildStandardCfg := func() *config.Config {
		return &config.Config{
			Tenants: map[string]*config.TenantConfig{
				"snapshot":         snapshotCfg,
				"institution-test": institutionCfg,
				"user-auth":        userAuthCfg,
				"default":          defaultCfg,
			},
			TenantsOrdered: []*config.TenantConfig{snapshotCfg, institutionCfg, userAuthCfg, defaultCfg},
			SCTenants: []config.SCTenantConfig{
				// compound: port=6444 AND prefix=diku_
				{Tenant: "snapshot", Port: 6444, UsernamePrefixes: []string{"diku_"}},
				// port-only
				{Tenant: "institution-test", Port: 6444},
				// prefix-only
				{Tenant: "user-auth", UsernamePrefixes: []string{"bob-bob"}},
				// catch-all: no rules
				{Tenant: "default"},
			},
		}
	}

	tests := []struct {
		name         string
		cfg          *config.Config
		serverPort   int
		clientIP     string
		username     string
		locationCode string
		wantTenant   string
	}{
		{
			name:       "PortAndPrefixCompound_PrefixMatches",
			cfg:        buildStandardCfg(),
			serverPort: 6444,
			clientIP:   "127.0.0.1",
			username:   "diku_admin",
			wantTenant: "snapshot",
		},
		{
			name:       "PortOnly_SkipsCompound",
			cfg:        buildStandardCfg(),
			serverPort: 6444,
			clientIP:   "127.0.0.1",
			username:   "bob-bob",
			wantTenant: "institution-test",
		},
		{
			name:       "PrefixOnly_NoPortMatch",
			cfg:        buildStandardCfg(),
			serverPort: 6443,
			clientIP:   "127.0.0.1",
			username:   "bob-bob",
			wantTenant: "user-auth",
		},
		{
			name:       "CatchAll_NothingMatches",
			cfg:        buildStandardCfg(),
			serverPort: 6443,
			clientIP:   "127.0.0.1",
			username:   "unknown-user",
			wantTenant: "default",
		},
		{
			// institution-test (port=6444, no prefix) is declared BEFORE
			// snapshot (port=6444, prefix=diku_).  institution-test has no prefix
			// rule so it is a wildcard on username — it matches diku_admin first.
			name: "DeclarationOrderMatters",
			cfg: &config.Config{
				Tenants: map[string]*config.TenantConfig{
					"snapshot":         snapshotCfg,
					"institution-test": institutionCfg,
					"user-auth":        userAuthCfg,
					"default":          defaultCfg,
				},
				TenantsOrdered: []*config.TenantConfig{institutionCfg, snapshotCfg, userAuthCfg, defaultCfg},
				SCTenants: []config.SCTenantConfig{
					// port-only declared first — wins for any username on 6444
					{Tenant: "institution-test", Port: 6444},
					// compound declared second — never reached for diku_admin on 6444
					{Tenant: "snapshot", Port: 6444, UsernamePrefixes: []string{"diku_"}},
					{Tenant: "user-auth", UsernamePrefixes: []string{"bob-bob"}},
					{Tenant: "default"},
				},
			},
			serverPort: 6444,
			clientIP:   "127.0.0.1",
			username:   "diku_admin",
			wantTenant: "institution-test",
		},
		{
			// SubnetGateRejects: first scTenant has SCSubnet=10.0.0.0/24; clientIP
			// 192.168.1.1 is outside that subnet, so it is skipped. The second
			// scTenant has no subnet rule and matches as a catch-all.
			name: "SubnetGateRejects",
			cfg: &config.Config{
				Tenants: map[string]*config.TenantConfig{
					"subnet-tenant":   snapshotCfg,
					"fallback-tenant": defaultCfg,
				},
				TenantsOrdered: []*config.TenantConfig{snapshotCfg, defaultCfg},
				SCTenants: []config.SCTenantConfig{
					{Tenant: "subnet-tenant", SCSubnet: "10.0.0.0/24"},
					{Tenant: "fallback-tenant"},
				},
			},
			serverPort: 6443,
			clientIP:   "192.168.1.1",
			username:   "any-user",
			wantTenant: "default",
		},
		{
			// LocationCodeGateRejects: first scTenant requires locationCode "BRANCH2";
			// we pass "MAIN" which is not in that list, so it is skipped. The second
			// scTenant has no location rule and wins.
			name: "LocationCodeGateRejects",
			cfg: &config.Config{
				Tenants: map[string]*config.TenantConfig{
					"branch2-tenant":  institutionCfg,
					"fallback-tenant": defaultCfg,
				},
				TenantsOrdered: []*config.TenantConfig{institutionCfg, defaultCfg},
				SCTenants: []config.SCTenantConfig{
					{Tenant: "branch2-tenant", LocationCodes: []string{"BRANCH2"}},
					{Tenant: "fallback-tenant"},
				},
			},
			serverPort:   6443,
			clientIP:     "127.0.0.1",
			username:     "any-user",
			locationCode: "MAIN",
			wantTenant:   "default",
		},
		{
			// UnknownTenantNameSkipped: first scTenant references "ghost" which is not
			// present in the Tenants map. It is silently skipped and the next valid
			// entry (fallback-tenant) wins.
			name: "UnknownTenantNameSkipped",
			cfg: &config.Config{
				Tenants: map[string]*config.TenantConfig{
					"fallback-tenant": defaultCfg,
				},
				TenantsOrdered: []*config.TenantConfig{defaultCfg},
				SCTenants: []config.SCTenantConfig{
					{Tenant: "ghost"},
					{Tenant: "fallback-tenant"},
				},
			},
			serverPort: 6443,
			clientIP:   "127.0.0.1",
			username:   "any-user",
			wantTenant: "default",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			svc := NewService(tt.cfg)
			got, err := svc.ResolveComplete(context.Background(), tt.serverPort, tt.clientIP, tt.username, tt.locationCode)
			if err != nil {
				t.Fatalf("ResolveComplete() error = %v", err)
			}
			if got.Tenant != tt.wantTenant {
				t.Errorf("ResolveComplete() tenant = %q, want %q", got.Tenant, tt.wantTenant)
			}
		})
	}
}

// TestNewSimpleIPResolver tests SimpleIPResolver creation
func TestNewSimpleIPResolver(t *testing.T) {
	tenantCfg := &config.TenantConfig{Tenant: "simple-tenant"}

	t.Run("Valid CIDR creates resolver", func(t *testing.T) {
		resolver, err := NewSimpleIPResolver("10.0.0.0/8", tenantCfg)
		if err != nil {
			t.Fatalf("NewSimpleIPResolver() error = %v", err)
		}
		if resolver == nil {
			t.Fatal("NewSimpleIPResolver() returned nil")
		}
	})

	t.Run("Invalid CIDR returns error", func(t *testing.T) {
		_, err := NewSimpleIPResolver("notacidr", tenantCfg)
		if err == nil {
			t.Error("NewSimpleIPResolver() should return error for invalid CIDR")
		}
	})
}

// TestSimpleIPResolverResolve tests SimpleIPResolver.Resolve
func TestSimpleIPResolverResolve(t *testing.T) {
	tenantCfg := &config.TenantConfig{Tenant: "simple-tenant"}
	resolver, err := NewSimpleIPResolver("10.0.0.0/8", tenantCfg)
	if err != nil {
		t.Fatalf("NewSimpleIPResolver() error = %v", err)
	}

	t.Run("Matching IP returns tenant config", func(t *testing.T) {
		result, err := resolver.Resolve(context.Background(), &ResolverData{ClientIP: "10.1.2.3"})
		if err != nil {
			t.Fatalf("Resolve() error = %v", err)
		}
		if result == nil {
			t.Fatal("Resolve() returned nil for matching IP")
		}
		if result.Tenant != "simple-tenant" {
			t.Errorf("Expected tenant 'simple-tenant', got %q", result.Tenant)
		}
	})

	t.Run("Non-matching IP returns nil", func(t *testing.T) {
		result, err := resolver.Resolve(context.Background(), &ResolverData{ClientIP: "192.168.1.1"})
		if err != nil {
			t.Fatalf("Resolve() error = %v", err)
		}
		if result != nil {
			t.Error("Resolve() should return nil for non-matching IP")
		}
	})

	t.Run("Empty IP returns error", func(t *testing.T) {
		_, err := resolver.Resolve(context.Background(), &ResolverData{ClientIP: ""})
		if err == nil {
			t.Error("Resolve() should return error for empty IP")
		}
	})

	t.Run("Invalid IP returns error", func(t *testing.T) {
		_, err := resolver.Resolve(context.Background(), &ResolverData{ClientIP: "notanip"})
		if err == nil {
			t.Error("Resolve() should return error for invalid IP")
		}
	})
}
