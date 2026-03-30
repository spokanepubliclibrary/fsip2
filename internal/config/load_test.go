package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTempMainConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	f := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(f, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}
	return f
}

func TestLoad_Success(t *testing.T) {
	f := writeTempMainConfig(t, "port: 7000\nokapiUrl: http://okapi.example.com\nhealthCheckPort: 9090\ntokenCacheCapacity: 50\nscanPeriod: 5000\nlogLevel: debug\n")
	cfg, err := Load(f)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}
	if cfg.Port != 7000 {
		t.Errorf("expected port 7000, got %d", cfg.Port)
	}
	if cfg.OkapiURL != "http://okapi.example.com" {
		t.Errorf("unexpected OkapiURL: %s", cfg.OkapiURL)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("unexpected LogLevel: %s", cfg.LogLevel)
	}
}

func TestLoad_FileNotFound(t *testing.T) {
	_, err := Load("/nonexistent/path/config.yaml")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	f := writeTempMainConfig(t, "port: [invalid yaml")
	_, err := Load(f)
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestLoad_Defaults(t *testing.T) {
	f := writeTempMainConfig(t, "okapiUrl: http://okapi.example.com\n")
	cfg, err := Load(f)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}
	if cfg.Port != 6443 {
		t.Errorf("expected default port 6443, got %d", cfg.Port)
	}
	if cfg.HealthCheckPort != 8081 {
		t.Errorf("expected default healthCheckPort 8081, got %d", cfg.HealthCheckPort)
	}
	if cfg.TokenCacheCapacity != 100 {
		t.Errorf("expected default tokenCacheCapacity 100, got %d", cfg.TokenCacheCapacity)
	}
	if cfg.ScanPeriod != 300000 {
		t.Errorf("expected default scanPeriod 300000, got %d", cfg.ScanPeriod)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("expected default logLevel 'info', got %s", cfg.LogLevel)
	}
}

func TestLoad_WithTenantSource(t *testing.T) {
	dir := t.TempDir()
	tenantFile := filepath.Join(dir, "tenant.yaml")
	tenantYAML := "tenants:\n  - tenant: load-test\n    okapiUrl: http://localhost:9130\n"
	if err := os.WriteFile(tenantFile, []byte(tenantYAML), 0644); err != nil {
		t.Fatalf("failed to write tenant file: %v", err)
	}

	mainYAML := "port: 6443\nokapiUrl: http://okapi.example.com\nhealthCheckPort: 8081\ntokenCacheCapacity: 100\ntenantConfigSources:\n  - type: file\n    path: " + tenantFile + "\n"
	mainFile := filepath.Join(dir, "main.yaml")
	if err := os.WriteFile(mainFile, []byte(mainYAML), 0644); err != nil {
		t.Fatalf("failed to write main config file: %v", err)
	}

	cfg, err := Load(mainFile)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if _, ok := cfg.Tenants["load-test"]; !ok {
		t.Error("expected 'load-test' tenant to be loaded")
	}
}

func TestLoad_UnsupportedSourceType(t *testing.T) {
	mainYAML := "port: 6443\nokapiUrl: http://okapi.example.com\nhealthCheckPort: 8081\ntokenCacheCapacity: 100\ntenantConfigSources:\n  - type: ftp\n    path: /some/path\n"
	f := writeTempMainConfig(t, mainYAML)
	_, err := Load(f)
	if err == nil {
		t.Error("expected error for unsupported source type")
	}
}

func TestLoadTenantConfigs_SCTenants(t *testing.T) {
	dir := t.TempDir()
	tenantFile := filepath.Join(dir, "tenant.yaml")
	tenantYAML := `tenants:
  - tenant: main-tenant
    okapiUrl: http://localhost:9130
scTenants:
  - tenant: sub-tenant
    port: 7000
    usernamePrefixes:
      - main_sip1
      - main_sip2
`
	if err := os.WriteFile(tenantFile, []byte(tenantYAML), 0644); err != nil {
		t.Fatalf("failed to write tenant file: %v", err)
	}

	cfg := &Config{
		Port:               6443,
		HealthCheckPort:    8081,
		OkapiURL:           "http://okapi.example.com",
		TokenCacheCapacity: 100,
		ScanPeriod:         5000,
		Tenants:            make(map[string]*TenantConfig),
		TenantConfigSources: []ConfigSource{
			{Type: "file", Path: tenantFile},
		},
	}

	if err := cfg.loadTenantConfigs(); err != nil {
		t.Fatalf("loadTenantConfigs() failed: %v", err)
	}

	if _, ok := cfg.Tenants["main-tenant"]; !ok {
		t.Error("expected 'main-tenant' to be loaded")
	}

	if len(cfg.SCTenants) != 1 {
		t.Fatalf("expected 1 SCTenant, got %d", len(cfg.SCTenants))
	}
	sc := cfg.SCTenants[0]
	if sc.Tenant != "sub-tenant" {
		t.Errorf("expected SCTenant Tenant 'sub-tenant', got %q", sc.Tenant)
	}
	if sc.Port != 7000 {
		t.Errorf("expected SCTenant Port 7000, got %d", sc.Port)
	}
	if len(sc.UsernamePrefixes) != 2 || sc.UsernamePrefixes[0] != "main_sip1" || sc.UsernamePrefixes[1] != "main_sip2" {
		t.Errorf("unexpected UsernamePrefixes: %v", sc.UsernamePrefixes)
	}
}

func TestLoadTenantConfigs_SCTenants_FullUsernamePrefix(t *testing.T) {
	// A full username (e.g. "main_sip1") used as a usernamePrefixes entry is
	// treated as a plain string — no special parsing or transformation.
	dir := t.TempDir()
	tenantFile := filepath.Join(dir, "tenant.yaml")
	tenantYAML := `tenants:
  - tenant: alpha
    okapiUrl: http://localhost:9130
scTenants:
  - tenant: alpha-sc
    port: 6500
    usernamePrefixes:
      - main_sip1
`
	if err := os.WriteFile(tenantFile, []byte(tenantYAML), 0644); err != nil {
		t.Fatalf("failed to write tenant file: %v", err)
	}

	cfg := &Config{
		Port:               6443,
		HealthCheckPort:    8081,
		OkapiURL:           "http://okapi.example.com",
		TokenCacheCapacity: 100,
		ScanPeriod:         5000,
		Tenants:            make(map[string]*TenantConfig),
		TenantConfigSources: []ConfigSource{
			{Type: "file", Path: tenantFile},
		},
	}

	if err := cfg.loadTenantConfigs(); err != nil {
		t.Fatalf("loadTenantConfigs() failed: %v", err)
	}

	if len(cfg.SCTenants) != 1 {
		t.Fatalf("expected 1 SCTenant, got %d", len(cfg.SCTenants))
	}
	sc := cfg.SCTenants[0]
	if len(sc.UsernamePrefixes) != 1 {
		t.Fatalf("expected 1 UsernamePrefixes entry, got %d", len(sc.UsernamePrefixes))
	}
	if sc.UsernamePrefixes[0] != "main_sip1" {
		t.Errorf("expected UsernamePrefixes[0] = %q, got %q", "main_sip1", sc.UsernamePrefixes[0])
	}
}

func TestLoad_FlatFormatTenantSource_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	tenantFile := filepath.Join(dir, "tenant.yaml")
	// Old flat format — no "tenants:" key — must be rejected.
	tenantYAML := "tenant: load-test\nokapiUrl: http://localhost:9130\n"
	if err := os.WriteFile(tenantFile, []byte(tenantYAML), 0644); err != nil {
		t.Fatalf("failed to write tenant file: %v", err)
	}

	mainYAML := "port: 6443\nokapiUrl: http://okapi.example.com\nhealthCheckPort: 8081\ntokenCacheCapacity: 100\ntenantConfigSources:\n  - type: file\n    path: " + tenantFile + "\n"
	mainFile := filepath.Join(dir, "main.yaml")
	if err := os.WriteFile(mainFile, []byte(mainYAML), 0644); err != nil {
		t.Fatalf("failed to write main config file: %v", err)
	}

	_, err := Load(mainFile)
	if err == nil {
		t.Error("expected error for flat-format tenant source, got nil")
	}
}

func TestLoad_WithListFormatTenantSource_MultiTenant(t *testing.T) {
	dir := t.TempDir()
	tenantFile := filepath.Join(dir, "tenant.yaml")
	tenantYAML := "tenants:\n  - tenant: tenant-a\n    okapiUrl: http://localhost:9130\n  - tenant: tenant-b\n    okapiUrl: http://localhost:9131\n"
	if err := os.WriteFile(tenantFile, []byte(tenantYAML), 0644); err != nil {
		t.Fatalf("failed to write tenant file: %v", err)
	}

	mainYAML := "port: 6443\nokapiUrl: http://okapi.example.com\nhealthCheckPort: 8081\ntokenCacheCapacity: 100\ntenantConfigSources:\n  - type: file\n    path: " + tenantFile + "\n"
	mainFile := filepath.Join(dir, "main.yaml")
	if err := os.WriteFile(mainFile, []byte(mainYAML), 0644); err != nil {
		t.Fatalf("failed to write main config file: %v", err)
	}

	cfg, err := Load(mainFile)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}
	if _, ok := cfg.Tenants["tenant-a"]; !ok {
		t.Error("expected 'tenant-a' to be loaded")
	}
	if _, ok := cfg.Tenants["tenant-b"]; !ok {
		t.Error("expected 'tenant-b' to be loaded")
	}
}

func TestLoadTenantConfigs_MultiTenantFile(t *testing.T) {
	dir := t.TempDir()
	tenantFile := filepath.Join(dir, "tenant.yaml")
	tenantYAML := `tenants:
  - tenant: alpha
    okapiUrl: http://localhost:9130
  - tenant: beta
    okapiUrl: http://localhost:9131
  - tenant: gamma
    okapiUrl: http://localhost:9132
scTenants:
  - tenant: alpha-sc
    port: 6500
`
	if err := os.WriteFile(tenantFile, []byte(tenantYAML), 0644); err != nil {
		t.Fatalf("failed to write tenant file: %v", err)
	}

	cfg := &Config{
		Port:               6443,
		HealthCheckPort:    8081,
		OkapiURL:           "http://okapi.example.com",
		TokenCacheCapacity: 100,
		ScanPeriod:         5000,
		Tenants:            make(map[string]*TenantConfig),
		TenantConfigSources: []ConfigSource{
			{Type: "file", Path: tenantFile},
		},
	}

	if err := cfg.loadTenantConfigs(); err != nil {
		t.Fatalf("loadTenantConfigs() failed: %v", err)
	}

	for _, name := range []string{"alpha", "beta", "gamma"} {
		if _, ok := cfg.Tenants[name]; !ok {
			t.Errorf("expected tenant %q to be loaded", name)
		}
	}

	if len(cfg.SCTenants) != 1 {
		t.Fatalf("expected 1 SCTenant, got %d", len(cfg.SCTenants))
	}
	if cfg.SCTenants[0].Tenant != "alpha-sc" {
		t.Errorf("expected SCTenant 'alpha-sc', got %q", cfg.SCTenants[0].Tenant)
	}
}

func TestLoadTenantConfigs_MultiSource_MultiTenantFiles(t *testing.T) {
	dir := t.TempDir()

	fileA := filepath.Join(dir, "a.yaml")
	yamlA := "tenants:\n  - tenant: a1\n    okapiUrl: http://localhost:9130\n  - tenant: a2\n    okapiUrl: http://localhost:9131\n"
	if err := os.WriteFile(fileA, []byte(yamlA), 0644); err != nil {
		t.Fatalf("failed to write fileA: %v", err)
	}

	fileB := filepath.Join(dir, "b.yaml")
	yamlB := "tenants:\n  - tenant: b1\n    okapiUrl: http://localhost:9132\n  - tenant: b2\n    okapiUrl: http://localhost:9133\n"
	if err := os.WriteFile(fileB, []byte(yamlB), 0644); err != nil {
		t.Fatalf("failed to write fileB: %v", err)
	}

	cfg := &Config{
		Port:               6443,
		HealthCheckPort:    8081,
		OkapiURL:           "http://okapi.example.com",
		TokenCacheCapacity: 100,
		ScanPeriod:         5000,
		Tenants:            make(map[string]*TenantConfig),
		TenantConfigSources: []ConfigSource{
			{Type: "file", Path: fileA},
			{Type: "file", Path: fileB},
		},
	}

	if err := cfg.loadTenantConfigs(); err != nil {
		t.Fatalf("loadTenantConfigs() failed: %v", err)
	}

	for _, name := range []string{"a1", "a2", "b1", "b2"} {
		if _, ok := cfg.Tenants[name]; !ok {
			t.Errorf("expected tenant %q to be loaded", name)
		}
	}
}

func TestValidatePatronCustomFields_Nil(t *testing.T) {
	tc := &TenantConfig{}
	if err := tc.ValidatePatronCustomFields(); err != nil {
		t.Errorf("unexpected error for nil PatronCustomFields: %v", err)
	}
}

func TestValidatePatronCustomFields_Disabled(t *testing.T) {
	tc := &TenantConfig{
		PatronCustomFields: &PatronCustomFieldsConfig{
			Enabled: false,
		},
	}
	if err := tc.ValidatePatronCustomFields(); err != nil {
		t.Errorf("unexpected error for disabled PatronCustomFields: %v", err)
	}
}

func TestValidatePatronCustomFields_EnabledNoFields(t *testing.T) {
	tc := &TenantConfig{
		PatronCustomFields: &PatronCustomFieldsConfig{
			Enabled: true,
			Fields:  []CustomFieldMapping{},
		},
	}
	if err := tc.ValidatePatronCustomFields(); err != nil {
		t.Errorf("unexpected error for enabled empty fields: %v", err)
	}
}

func TestValidatePatronCustomFields_ValidField(t *testing.T) {
	tc := &TenantConfig{
		PatronCustomFields: &PatronCustomFieldsConfig{
			Enabled: true,
			Fields: []CustomFieldMapping{
				{Code: "SA", Source: "myField", Type: "string"},
			},
		},
	}
	if err := tc.ValidatePatronCustomFields(); err != nil {
		t.Errorf("unexpected error for valid field: %v", err)
	}
}

func TestValidatePatronCustomFields_InvalidCode(t *testing.T) {
	tc := &TenantConfig{
		PatronCustomFields: &PatronCustomFieldsConfig{
			Enabled: true,
			Fields: []CustomFieldMapping{
				{Code: "AA", Source: "myField", Type: "string"},
			},
		},
	}
	if err := tc.ValidatePatronCustomFields(); err == nil {
		t.Error("expected error for invalid field code AA")
	}
}

func TestValidatePatronCustomFields_DuplicateCode(t *testing.T) {
	tc := &TenantConfig{
		PatronCustomFields: &PatronCustomFieldsConfig{
			Enabled: true,
			Fields: []CustomFieldMapping{
				{Code: "SA", Source: "field1", Type: "string"},
				{Code: "SA", Source: "field2", Type: "string"},
			},
		},
	}
	if err := tc.ValidatePatronCustomFields(); err == nil {
		t.Error("expected error for duplicate field code")
	}
}

func TestValidatePatronCustomFields_EmptySource(t *testing.T) {
	tc := &TenantConfig{
		PatronCustomFields: &PatronCustomFieldsConfig{
			Enabled: true,
			Fields: []CustomFieldMapping{
				{Code: "SB", Source: "", Type: "string"},
			},
		},
	}
	if err := tc.ValidatePatronCustomFields(); err == nil {
		t.Error("expected error for empty source")
	}
}

func TestValidatePatronCustomFields_InvalidType(t *testing.T) {
	tc := &TenantConfig{
		PatronCustomFields: &PatronCustomFieldsConfig{
			Enabled: true,
			Fields: []CustomFieldMapping{
				{Code: "SC", Source: "myField", Type: "integer"},
			},
		},
	}
	if err := tc.ValidatePatronCustomFields(); err == nil {
		t.Error("expected error for invalid type 'integer'")
	}
}

func TestValidatePatronCustomFields_ArrayDefaultDelimiter(t *testing.T) {
	tc := &TenantConfig{
		PatronCustomFields: &PatronCustomFieldsConfig{
			Enabled: true,
			Fields: []CustomFieldMapping{
				{Code: "SD", Source: "myField", Type: "array"},
			},
		},
	}
	if err := tc.ValidatePatronCustomFields(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if tc.PatronCustomFields.Fields[0].ArrayDelimiter != "," {
		t.Errorf("expected default array delimiter ',', got %q", tc.PatronCustomFields.Fields[0].ArrayDelimiter)
	}
}

func TestValidatePatronCustomFields_DefaultMaxLength(t *testing.T) {
	tc := &TenantConfig{
		PatronCustomFields: &PatronCustomFieldsConfig{
			Enabled: true,
			Fields: []CustomFieldMapping{
				{Code: "SE", Source: "myField", Type: "boolean"},
			},
		},
	}
	if err := tc.ValidatePatronCustomFields(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if tc.PatronCustomFields.Fields[0].MaxLength != 60 {
		t.Errorf("expected default MaxLength 60, got %d", tc.PatronCustomFields.Fields[0].MaxLength)
	}
}
