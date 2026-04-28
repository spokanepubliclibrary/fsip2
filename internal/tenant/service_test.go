package tenant

import (
	"sync"
	"testing"

	"github.com/spokanepubliclibrary/fsip2/internal/config"
)

// TestService_Reinitialize_UpdatesTenantConfig verifies that Reinitialize replaces
// tenantConfigs so GetTenantByName reflects the updated values.
func TestService_Reinitialize_UpdatesTenantConfig(t *testing.T) {
	tenantA := &config.TenantConfig{
		Tenant:                      "tenant-a",
		UsePinForPatronVerification: true,
	}
	cfg := &config.Config{
		Tenants: map[string]*config.TenantConfig{
			"tenant-a": tenantA,
		},
		TenantsOrdered: []*config.TenantConfig{tenantA},
	}

	svc := NewService(cfg)

	got, ok := svc.GetTenantByName("tenant-a")
	if !ok {
		t.Fatal("GetTenantByName(\"tenant-a\") should return ok=true before Reinitialize")
	}
	if !got.UsePinForPatronVerification {
		t.Error("Expected UsePinForPatronVerification=true before Reinitialize")
	}

	// Build a new config with the same tenant but the flag flipped.
	tenantAv2 := &config.TenantConfig{
		Tenant:                      "tenant-a",
		UsePinForPatronVerification: false,
	}
	newCfg := &config.Config{
		Tenants: map[string]*config.TenantConfig{
			"tenant-a": tenantAv2,
		},
		TenantsOrdered: []*config.TenantConfig{tenantAv2},
	}

	svc.Reinitialize(newCfg)

	got, ok = svc.GetTenantByName("tenant-a")
	if !ok {
		t.Fatal("GetTenantByName(\"tenant-a\") should return ok=true after Reinitialize")
	}
	if got.UsePinForPatronVerification {
		t.Error("Expected UsePinForPatronVerification=false after Reinitialize")
	}
}

// TestService_Reinitialize_RebuildsResolvers verifies that Reinitialize tears down and
// rebuilds the resolver list from the new config.
func TestService_Reinitialize_RebuildsResolvers(t *testing.T) {
	tenantA := &config.TenantConfig{
		Tenant:   "tenant-a",
		OkapiURL: "https://tenant-a.example.com",
	}
	cfg := &config.Config{
		Tenants: map[string]*config.TenantConfig{
			"tenant-a": tenantA,
		},
		TenantsOrdered: []*config.TenantConfig{tenantA},
		SCTenants: []config.SCTenantConfig{
			{
				Tenant: "tenant-a",
				Port:   6443,
			},
		},
	}

	svc := NewService(cfg)

	if svc.GetResolverCount(PhaseConnect) == 0 {
		t.Fatal("Expected GetResolverCount(PhaseConnect) > 0 before Reinitialize")
	}

	// Reinitialize with a config that has no SCTenant entries.
	tenantAbare := &config.TenantConfig{
		Tenant: "tenant-a",
	}
	emptyCfg := &config.Config{
		Tenants: map[string]*config.TenantConfig{
			"tenant-a": tenantAbare,
		},
		TenantsOrdered: []*config.TenantConfig{tenantAbare},
		SCTenants:      []config.SCTenantConfig{},
	}

	svc.Reinitialize(emptyCfg)

	if count := svc.GetResolverCount(PhaseConnect); count != 0 {
		t.Errorf("Expected GetResolverCount(PhaseConnect)=0 after Reinitialize, got %d", count)
	}
}

// TestService_Reinitialize_ConcurrentAccess exercises concurrent reads and reinitializations
// to catch data races when run with -race.
func TestService_Reinitialize_ConcurrentAccess(t *testing.T) {
	tenantA := &config.TenantConfig{
		Tenant:                      "tenant-a",
		UsePinForPatronVerification: true,
	}
	cfgA := &config.Config{
		Tenants: map[string]*config.TenantConfig{
			"tenant-a": tenantA,
		},
		TenantsOrdered: []*config.TenantConfig{tenantA},
	}

	tenantB := &config.TenantConfig{
		Tenant:                      "tenant-a",
		UsePinForPatronVerification: false,
	}
	cfgB := &config.Config{
		Tenants: map[string]*config.TenantConfig{
			"tenant-a": tenantB,
		},
		TenantsOrdered: []*config.TenantConfig{tenantB},
	}

	svc := NewService(cfgA)

	var wg sync.WaitGroup

	// 5 reader goroutines each calling GetTenantByName 50 times.
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				svc.GetTenantByName("tenant-a")
			}
		}()
	}

	// 2 writer goroutines each calling Reinitialize 10 times with alternating configs.
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				if (id+j)%2 == 0 {
					svc.Reinitialize(cfgA)
				} else {
					svc.Reinitialize(cfgB)
				}
			}
		}(i)
	}

	wg.Wait()
}
