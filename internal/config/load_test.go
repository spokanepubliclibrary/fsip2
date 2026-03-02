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
	tenantYAML := "tenant: load-test\nokapiUrl: http://localhost:9130\n"
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
	tenantYAML := "tenant: main-tenant\nokapiUrl: http://localhost:9130\nscTenants:\n  - tenant: sub-tenant\n"
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
	if _, ok := cfg.Tenants["sub-tenant"]; !ok {
		t.Error("expected 'sub-tenant' to be loaded from scTenants")
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
