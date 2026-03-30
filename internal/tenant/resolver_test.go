package tenant

import (
	"context"
	"testing"

	"github.com/spokanepubliclibrary/fsip2/internal/config"
)

// TestNewService tests service creation
func TestNewService(t *testing.T) {
	cfg := &config.Config{
		OkapiURL: "https://folio.example.com",
		Tenants: map[string]*config.TenantConfig{
			"default": {
				Tenant:   "default",
				OkapiURL: "https://folio.example.com",
			},
		},
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

// TestResolveAtConnectDefault tests connection resolution with default tenant
func TestResolveAtConnectDefault(t *testing.T) {
	cfg := &config.Config{
		Tenants: map[string]*config.TenantConfig{
			"default": {
				Tenant: "default",
			},
		},
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
	cfg := &config.Config{
		Tenants: map[string]*config.TenantConfig{
			"default": {
				Tenant:      "default",
				OkapiURL:    "https://default.example.com",
				OkapiTenant: "default",
			},
			"tenant1": {
				Tenant:      "tenant1",
				OkapiURL:    "https://tenant1.example.com",
				OkapiTenant: "tenant1",
			},
		},
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
	cfg := &config.Config{
		Tenants: map[string]*config.TenantConfig{
			"default": {
				Tenant:      "default",
				OkapiURL:    "https://default.example.com",
				OkapiTenant: "default",
			},
			"tenant2": {
				Tenant:      "tenant2",
				OkapiURL:    "https://tenant2.example.com",
				OkapiTenant: "tenant2",
			},
		},
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

// TestResolveAtLoginWithLocationCode tests location code resolution using top-level SCTenants.
func TestResolveAtLoginWithLocationCode(t *testing.T) {
	defaultConfig := &config.TenantConfig{
		Tenant:      "default",
		OkapiURL:    "https://default.example.com",
		OkapiTenant: "default",
	}

	cfg := &config.Config{
		Tenants: map[string]*config.TenantConfig{
			"default": defaultConfig,
			"tenant3": {
				Tenant:      "tenant3",
				OkapiURL:    "https://tenant3.example.com",
				OkapiTenant: "tenant3",
			},
		},
		SCTenants: []config.SCTenantConfig{
			{
				Tenant:        "tenant3",
				LocationCodes: []string{"MAIN", "BRANCH1"},
			},
		},
	}

	svc := NewService(cfg)

	// Matching location code.
	result, err := svc.ResolveAtLogin(context.Background(), "user1", "MAIN", defaultConfig)
	if err != nil {
		t.Fatalf("ResolveAtLogin() error = %v", err)
	}
	if result.Tenant != "tenant3" {
		t.Errorf("Expected tenant3 for location MAIN, got '%s'", result.Tenant)
	}

	// Non-matching location code falls back to default.
	result, err = svc.ResolveAtLogin(context.Background(), "user1", "UNKNOWN", defaultConfig)
	if err != nil {
		t.Fatalf("ResolveAtLogin() error = %v", err)
	}
	if result.Tenant != "default" {
		t.Errorf("Expected default for unknown location, got '%s'", result.Tenant)
	}
}

// TestResolveAtLoginWithUsernamePrefix tests username prefix resolution using top-level SCTenants.
// Verifies the resolver returns the correct looked-up TenantConfig (correct OkapiURL/OkapiTenant).
func TestResolveAtLoginWithUsernamePrefix(t *testing.T) {
	defaultConfig := &config.TenantConfig{
		Tenant:      "default",
		OkapiURL:    "https://default.example.com",
		OkapiTenant: "default",
	}

	cfg := &config.Config{
		Tenants: map[string]*config.TenantConfig{
			"default": defaultConfig,
			"tenant4": {
				Tenant:      "tenant4",
				OkapiURL:    "https://tenant4.example.com",
				OkapiTenant: "tenant4",
			},
		},
		SCTenants: []config.SCTenantConfig{
			{
				Tenant:           "tenant4",
				UsernamePrefixes: []string{"lib4_", "test4_"},
			},
		},
	}

	svc := NewService(cfg)

	// Matching prefix resolves to the correct TenantConfig — not the parent's.
	result, err := svc.ResolveAtLogin(context.Background(), "lib4_john", "", defaultConfig)
	if err != nil {
		t.Fatalf("ResolveAtLogin() error = %v", err)
	}
	if result.Tenant != "tenant4" {
		t.Errorf("Expected tenant4 for username lib4_john, got '%s'", result.Tenant)
	}
	if result.OkapiURL != "https://tenant4.example.com" {
		t.Errorf("Expected OkapiURL 'https://tenant4.example.com', got '%s'", result.OkapiURL)
	}
	if result.OkapiTenant != "tenant4" {
		t.Errorf("Expected OkapiTenant 'tenant4', got '%s'", result.OkapiTenant)
	}

	// Non-matching username falls back to default.
	result, err = svc.ResolveAtLogin(context.Background(), "john", "", defaultConfig)
	if err != nil {
		t.Fatalf("ResolveAtLogin() error = %v", err)
	}
	if result.Tenant != "default" {
		t.Errorf("Expected default for username john, got '%s'", result.Tenant)
	}
}

// TestResolveAtLoginWithFullUsernamePrefix tests that a full username used as a usernamePrefixes
// value (e.g. "main_sip1") matches exactly via strings.HasPrefix semantics.
func TestResolveAtLoginWithFullUsernamePrefix(t *testing.T) {
	defaultConfig := &config.TenantConfig{
		Tenant:      "default",
		OkapiURL:    "https://default.example.com",
		OkapiTenant: "default",
	}

	cfg := &config.Config{
		Tenants: map[string]*config.TenantConfig{
			"default": defaultConfig,
			"main": {
				Tenant:      "main",
				OkapiURL:    "https://main.example.com",
				OkapiTenant: "main",
			},
		},
		SCTenants: []config.SCTenantConfig{
			{
				Tenant:           "main",
				UsernamePrefixes: []string{"main_sip1"},
			},
		},
	}

	svc := NewService(cfg)

	// Exact full username "main_sip1" matches via HasPrefix.
	result, err := svc.ResolveAtLogin(context.Background(), "main_sip1", "", defaultConfig)
	if err != nil {
		t.Fatalf("ResolveAtLogin() error = %v", err)
	}
	if result.Tenant != "main" {
		t.Errorf("Expected 'main' for username main_sip1, got '%s'", result.Tenant)
	}
	if result.OkapiURL != "https://main.example.com" {
		t.Errorf("Expected OkapiURL 'https://main.example.com', got '%s'", result.OkapiURL)
	}

	// A username that merely contains the prefix as a substring but doesn't start with it
	// should not match (e.g. "other_main_sip1").
	result, err = svc.ResolveAtLogin(context.Background(), "other_main_sip1", "", defaultConfig)
	if err != nil {
		t.Fatalf("ResolveAtLogin() error = %v", err)
	}
	if result.Tenant != "default" {
		t.Errorf("Expected default for username other_main_sip1, got '%s'", result.Tenant)
	}
}

// TestSCTenantUnknownTenantSkipped verifies that an SCTenant referencing a name not present
// in cfg.Tenants is skipped gracefully — no panic, and no resolver is registered.
func TestSCTenantUnknownTenantSkipped(t *testing.T) {
	cfg := &config.Config{
		Tenants: map[string]*config.TenantConfig{
			"default": {
				Tenant:   "default",
				OkapiURL: "https://default.example.com",
			},
		},
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
	cfg := &config.Config{
		Tenants: map[string]*config.TenantConfig{
			"default": {
				Tenant: "default",
			},
		},
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
	cfg := &config.Config{
		Tenants: map[string]*config.TenantConfig{
			"tenant1": {
				Tenant: "tenant1",
			},
			"tenant2": {
				Tenant: "tenant2",
			},
		},
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
	cfg := &config.Config{
		Tenants: map[string]*config.TenantConfig{
			"tenant1": {Tenant: "tenant1"},
			"tenant2": {Tenant: "tenant2"},
		},
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
	cfg := &config.Config{
		Tenants: map[string]*config.TenantConfig{
			"default": {Tenant: "default"},
			"tenant1": {
				Tenant:      "tenant1",
				OkapiURL:    "https://tenant1.example.com",
				OkapiTenant: "tenant1",
			},
		},
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
	cfg := &config.Config{
		Tenants: map[string]*config.TenantConfig{
			"default": {Tenant: "default"},
			"tenant1": {
				Tenant:   "tenant1",
				OkapiURL: "https://tenant1.example.com",
			},
		},
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
