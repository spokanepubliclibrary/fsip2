package config

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// createTestReloader creates a minimal reloader with a file-based config source
func createTestReloader(t *testing.T, tenantYAML string, onChangeCallback func(*Config)) (*Reloader, string) {
	t.Helper()

	dir := t.TempDir()
	tenantFile := filepath.Join(dir, "tenant.yaml")
	if err := os.WriteFile(tenantFile, []byte(tenantYAML), 0644); err != nil {
		t.Fatalf("Failed to write tenant config file: %v", err)
	}

	cfg := &Config{
		Port:               6443,
		HealthCheckPort:    8081,
		OkapiURL:           "http://localhost:9130",
		TokenCacheCapacity: 100,
		ScanPeriod:         50,
		LogLevel:           "info",
		Tenants:            make(map[string]*TenantConfig),
		TenantConfigSources: []ConfigSource{
			{Type: "file", Path: tenantFile},
		},
	}

	reloader := NewReloader(cfg, onChangeCallback)
	return reloader, tenantFile
}

const reloaderTenantYAML = `
tenants:
  - tenant: reloader-test
    okapiUrl: http://localhost:9130
`

func TestNewReloader(t *testing.T) {
	cfg := &Config{
		ScanPeriod: 1000,
		Tenants:    make(map[string]*TenantConfig),
	}

	reloader := NewReloader(cfg, nil)
	if reloader == nil {
		t.Fatal("Expected non-nil reloader")
	}
	if reloader.config != cfg {
		t.Error("Expected reloader.config to match provided config")
	}
	if reloader.interval != cfg.GetScanPeriod() {
		t.Errorf("Expected interval %v, got %v", cfg.GetScanPeriod(), reloader.interval)
	}
}

func TestReloader_IsRunning_InitiallyFalse(t *testing.T) {
	cfg := &Config{ScanPeriod: 1000, Tenants: make(map[string]*TenantConfig)}
	reloader := NewReloader(cfg, nil)

	if reloader.IsRunning() {
		t.Error("Expected IsRunning to be false before Start()")
	}
}

func TestReloader_Start_Stop(t *testing.T) {
	reloader, _ := createTestReloader(t, reloaderTenantYAML, nil)

	ctx := context.Background()
	err := reloader.Start(ctx)
	if err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	if !reloader.IsRunning() {
		t.Error("Expected IsRunning to be true after Start()")
	}

	reloader.Stop()

	if reloader.IsRunning() {
		t.Error("Expected IsRunning to be false after Stop()")
	}
}

func TestReloader_Start_AlreadyRunning(t *testing.T) {
	reloader, _ := createTestReloader(t, reloaderTenantYAML, nil)

	ctx := context.Background()
	if err := reloader.Start(ctx); err != nil {
		t.Fatalf("First Start() failed: %v", err)
	}
	defer reloader.Stop()

	err := reloader.Start(ctx)
	if err == nil {
		t.Error("Expected error when starting already-running reloader")
	}
}

func TestReloader_Stop_NotRunning(t *testing.T) {
	cfg := &Config{ScanPeriod: 1000, Tenants: make(map[string]*TenantConfig)}
	reloader := NewReloader(cfg, nil)
	reloader.Stop() // should not panic
}

func TestReloader_GetCurrentConfig(t *testing.T) {
	cfg := &Config{
		ScanPeriod: 1000,
		OkapiURL:   "http://test.example.com",
		Tenants:    make(map[string]*TenantConfig),
	}
	reloader := NewReloader(cfg, nil)

	result := reloader.GetCurrentConfig()
	if result != cfg {
		t.Error("Expected GetCurrentConfig to return the same config pointer")
	}
	if result.OkapiURL != "http://test.example.com" {
		t.Errorf("Expected OkapiURL 'http://test.example.com', got %q", result.OkapiURL)
	}
}

func TestReloader_UpdateInterval(t *testing.T) {
	cfg := &Config{ScanPeriod: 1000, Tenants: make(map[string]*TenantConfig)}
	reloader := NewReloader(cfg, nil)

	newInterval := 5 * time.Second
	reloader.UpdateInterval(newInterval)

	if reloader.interval != newInterval {
		t.Errorf("Expected interval %v after UpdateInterval, got %v", newInterval, reloader.interval)
	}
}

func TestReloader_TriggerReload(t *testing.T) {
	reloader, _ := createTestReloader(t, reloaderTenantYAML, nil)

	err := reloader.TriggerReload()
	if err != nil {
		t.Fatalf("TriggerReload() failed: %v", err)
	}
}

func TestReloader_TriggerReload_LoadsTenantConfig(t *testing.T) {
	reloader, _ := createTestReloader(t, reloaderTenantYAML, nil)

	// Start the reloader to initialize loaders, then stop before the timer fires
	ctx := context.Background()
	if err := reloader.Start(ctx); err != nil {
		t.Fatalf("Start() failed: %v", err)
	}
	reloader.Stop()

	if err := reloader.TriggerReload(); err != nil {
		t.Fatalf("TriggerReload() failed: %v", err)
	}

	cfg := reloader.GetCurrentConfig()
	if _, ok := cfg.Tenants["reloader-test"]; !ok {
		t.Error("Expected 'reloader-test' tenant to be loaded after TriggerReload")
	}
}

func TestReloader_Start_WithContextCancellation(t *testing.T) {
	reloader, _ := createTestReloader(t, reloaderTenantYAML, nil)

	ctx, cancel := context.WithCancel(context.Background())

	if err := reloader.Start(ctx); err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	cancel()
	time.Sleep(100 * time.Millisecond)
	reloader.Stop()
}

func TestReloader_Start_UnsupportedSourceType(t *testing.T) {
	cfg := &Config{
		ScanPeriod: 50,
		Tenants:    make(map[string]*TenantConfig),
		TenantConfigSources: []ConfigSource{
			{Type: "unsupported", Path: "/some/path"},
		},
	}
	reloader := NewReloader(cfg, nil)

	ctx := context.Background()
	err := reloader.Start(ctx)
	if err == nil {
		t.Error("Expected error for unsupported config source type")
	}
}

func TestReloader_OnChangeCallback(t *testing.T) {
	changeCalled := false

	reloader, tenantFile := createTestReloader(t, reloaderTenantYAML, func(cfg *Config) {
		changeCalled = true
	})

	// Start to initialize loaders
	ctx := context.Background()
	if err := reloader.Start(ctx); err != nil {
		t.Fatalf("Start() failed: %v", err)
	}
	reloader.Stop()

	if err := reloader.TriggerReload(); err != nil {
		t.Fatalf("TriggerReload() failed: %v", err)
	}

	updatedYAML := `
tenants:
  - tenant: reloader-test
    okapiUrl: http://updated.example.com
`
	if err := os.WriteFile(tenantFile, []byte(updatedYAML), 0644); err != nil {
		t.Fatalf("Failed to update tenant file: %v", err)
	}

	if err := reloader.TriggerReload(); err != nil {
		t.Fatalf("Second TriggerReload() failed: %v", err)
	}

	if !changeCalled {
		t.Error("Expected onChange callback to be called after config change")
	}
}

func TestReloader_TriggerReload_DetectsModifiedTenant(t *testing.T) {
	// Use a YAML with explicit messageDelimiter to trigger compareFields/escapeDelimiter
	initialYAML := "tenants:\n  - tenant: change-test\n    okapiUrl: http://initial.example.com\n    messageDelimiter: INIT\n"
	reloader, tenantFile := createTestReloader(t, initialYAML, nil)

	// Start to initialize loaders
	ctx := context.Background()
	if err := reloader.Start(ctx); err != nil {
		t.Fatalf("Start() failed: %v", err)
	}
	reloader.Stop()

	// First reload - adds the tenant (covers findConfigChanges "added" path)
	if err := reloader.TriggerReload(); err != nil {
		t.Fatalf("First TriggerReload() failed: %v", err)
	}
	if _, ok := reloader.GetCurrentConfig().Tenants["change-test"]; !ok {
		t.Fatal("Expected 'change-test' tenant after first reload")
	}

	// Modify the tenant - change okapiUrl and messageDelimiter
	updatedYAML := "tenants:\n  - tenant: change-test\n    okapiUrl: http://updated.example.com\n    messageDelimiter: UPDT\n"
	if err := os.WriteFile(tenantFile, []byte(updatedYAML), 0644); err != nil {
		t.Fatalf("Failed to update tenant file: %v", err)
	}

	// Second reload - modifies the tenant
	// Covers: deepCompareTenants, compareFields, escapeDelimiter, logConfigChanges "else" branch
	if err := reloader.TriggerReload(); err != nil {
		t.Fatalf("Second TriggerReload() failed: %v", err)
	}

	tc := reloader.GetCurrentConfig().Tenants["change-test"]
	if tc == nil {
		t.Fatal("Expected 'change-test' tenant after second reload")
	}
	if tc.OkapiURL != "http://updated.example.com" {
		t.Errorf("Expected updated OkapiURL, got %q", tc.OkapiURL)
	}
}

func TestReloader_TriggerReload_DetectsRemovedTenant(t *testing.T) {
	initialYAML := "tenants:\n  - tenant: tenant-a\n    okapiUrl: http://a.example.com\n"
	reloader, tenantFile := createTestReloader(t, initialYAML, nil)

	// Start to initialize loaders
	ctx := context.Background()
	if err := reloader.Start(ctx); err != nil {
		t.Fatalf("Start() failed: %v", err)
	}
	reloader.Stop()

	// First reload - adds tenant-a
	if err := reloader.TriggerReload(); err != nil {
		t.Fatalf("First TriggerReload() failed: %v", err)
	}

	// Change the file to a different tenant
	updatedYAML := "tenants:\n  - tenant: tenant-b\n    okapiUrl: http://b.example.com\n"
	if err := os.WriteFile(tenantFile, []byte(updatedYAML), 0644); err != nil {
		t.Fatalf("Failed to update tenant file: %v", err)
	}

	// Second reload - removes tenant-a, adds tenant-b
	// Covers: logConfigChanges "removed" branch
	if err := reloader.TriggerReload(); err != nil {
		t.Fatalf("Second TriggerReload() failed: %v", err)
	}

	cfg := reloader.GetCurrentConfig()
	if _, ok := cfg.Tenants["tenant-a"]; ok {
		t.Error("Expected 'tenant-a' to be removed")
	}
	if _, ok := cfg.Tenants["tenant-b"]; !ok {
		t.Error("Expected 'tenant-b' to be added")
	}
}

func TestReloader_TriggerReload_ListFormat_MultiTenant(t *testing.T) {
	yaml := `
tenants:
  - tenant: alpha
    okapiUrl: http://alpha.example.com
  - tenant: beta
    okapiUrl: http://beta.example.com
`
	reloader, _ := createTestReloader(t, yaml, nil)

	ctx := context.Background()
	if err := reloader.Start(ctx); err != nil {
		t.Fatalf("Start() failed: %v", err)
	}
	reloader.Stop()

	if err := reloader.TriggerReload(); err != nil {
		t.Fatalf("TriggerReload() failed: %v", err)
	}

	cfg := reloader.GetCurrentConfig()
	if _, ok := cfg.Tenants["alpha"]; !ok {
		t.Error("Expected 'alpha' tenant to be loaded")
	}
	if _, ok := cfg.Tenants["beta"]; !ok {
		t.Error("Expected 'beta' tenant to be loaded")
	}
}

func TestReloader_TriggerReload_ListFormat_AddTenant(t *testing.T) {
	initialYAML := "tenants:\n  - tenant: alpha\n    okapiUrl: http://alpha.example.com\n"
	reloader, tenantFile := createTestReloader(t, initialYAML, nil)

	ctx := context.Background()
	if err := reloader.Start(ctx); err != nil {
		t.Fatalf("Start() failed: %v", err)
	}
	reloader.Stop()

	if err := reloader.TriggerReload(); err != nil {
		t.Fatalf("First TriggerReload() failed: %v", err)
	}
	if _, ok := reloader.GetCurrentConfig().Tenants["alpha"]; !ok {
		t.Fatal("Expected 'alpha' after first reload")
	}

	updatedYAML := "tenants:\n  - tenant: alpha\n    okapiUrl: http://alpha.example.com\n  - tenant: beta\n    okapiUrl: http://beta.example.com\n"
	if err := os.WriteFile(tenantFile, []byte(updatedYAML), 0644); err != nil {
		t.Fatalf("Failed to update tenant file: %v", err)
	}

	if err := reloader.TriggerReload(); err != nil {
		t.Fatalf("Second TriggerReload() failed: %v", err)
	}

	cfg := reloader.GetCurrentConfig()
	if _, ok := cfg.Tenants["alpha"]; !ok {
		t.Error("Expected 'alpha' still present after adding beta")
	}
	if _, ok := cfg.Tenants["beta"]; !ok {
		t.Error("Expected 'beta' to be added on second reload")
	}
}

func TestReloader_TriggerReload_ListFormat_RemoveTenant(t *testing.T) {
	initialYAML := "tenants:\n  - tenant: alpha\n    okapiUrl: http://alpha.example.com\n  - tenant: beta\n    okapiUrl: http://beta.example.com\n"
	reloader, tenantFile := createTestReloader(t, initialYAML, nil)

	ctx := context.Background()
	if err := reloader.Start(ctx); err != nil {
		t.Fatalf("Start() failed: %v", err)
	}
	reloader.Stop()

	if err := reloader.TriggerReload(); err != nil {
		t.Fatalf("First TriggerReload() failed: %v", err)
	}
	if _, ok := reloader.GetCurrentConfig().Tenants["alpha"]; !ok {
		t.Fatal("Expected 'alpha' after first reload")
	}
	if _, ok := reloader.GetCurrentConfig().Tenants["beta"]; !ok {
		t.Fatal("Expected 'beta' after first reload")
	}

	updatedYAML := "tenants:\n  - tenant: alpha\n    okapiUrl: http://alpha.example.com\n"
	if err := os.WriteFile(tenantFile, []byte(updatedYAML), 0644); err != nil {
		t.Fatalf("Failed to update tenant file: %v", err)
	}

	changeCalled := false
	reloader.onChange = func(cfg *Config) { changeCalled = true }

	if err := reloader.TriggerReload(); err != nil {
		t.Fatalf("Second TriggerReload() failed: %v", err)
	}

	cfg := reloader.GetCurrentConfig()
	if _, ok := cfg.Tenants["alpha"]; !ok {
		t.Error("Expected 'alpha' to remain")
	}
	if _, ok := cfg.Tenants["beta"]; ok {
		t.Error("Expected 'beta' to be removed")
	}
	if !changeCalled {
		t.Error("Expected onChange to be called after removing a tenant")
	}
}

func TestReloader_TriggerReload_FlatFormat_LogsError(t *testing.T) {
	// Old flat format (no "tenants:" key) must be rejected; the reloader must not panic
	// and cfg.Tenants must remain unchanged (empty).
	flatYAML := "tenant: bad-tenant\nokapiUrl: http://bad.example.com\n"
	reloader, _ := createTestReloader(t, flatYAML, nil)

	ctx := context.Background()
	if err := reloader.Start(ctx); err != nil {
		t.Fatalf("Start() failed: %v", err)
	}
	reloader.Stop()

	// TriggerReload may return an error or swallow it internally; either way it must not panic.
	_ = reloader.TriggerReload()

	cfg := reloader.GetCurrentConfig()
	if len(cfg.Tenants) != 0 {
		t.Errorf("Expected Tenants map to remain empty after flat-format load, got %v", cfg.Tenants)
	}
}
