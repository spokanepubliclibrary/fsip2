package config

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

const validTenantYAML = `
tenants:
  - tenant: testlib
    okapiUrl: http://localhost:9130
    okapiTenant: testlib
    fieldDelimiter: "|"
    messageDelimiter: "\\r"
    charset: UTF-8
    timezone: America/Chicago
    supportedMessages:
      - code: "23"
        enabled: true
      - code: "11"
        enabled: true
`

const invalidYAML = `
tenant: [invalid yaml
  this is broken
`

const validTenantYAMLWithRollingRenewals = `
tenants:
  - tenant: testlib2
    okapiUrl: http://localhost:9130
    okapiTenant: testlib2
    fieldDelimiter: "|"
    rollingRenewals:
      enabled: true
      renewWithin: "6M"
      extendFor: "1Y"
`

func TestFileLoader_Load_Success(t *testing.T) {
	tmpFile := writeTempFile(t, validTenantYAML)

	loader := &FileLoader{Path: tmpFile}
	cfgs, _, err := loader.Load()
	if err != nil {
		t.Fatalf("FileLoader.Load() failed: %v", err)
	}

	if cfgs[0].Tenant != "testlib" {
		t.Errorf("Expected tenant 'testlib', got %q", cfgs[0].Tenant)
	}
	if cfgs[0].OkapiURL != "http://localhost:9130" {
		t.Errorf("Expected OkapiURL 'http://localhost:9130', got %q", cfgs[0].OkapiURL)
	}
}

func TestFileLoader_Load_FileNotFound(t *testing.T) {
	loader := &FileLoader{Path: "/nonexistent/path/to/config.yaml"}
	_, _, err := loader.Load()
	if err == nil {
		t.Error("Expected error for nonexistent file")
	}
}

func TestFileLoader_Load_InvalidYAML(t *testing.T) {
	tmpFile := writeTempFile(t, invalidYAML)

	loader := &FileLoader{Path: tmpFile}
	_, _, err := loader.Load()
	if err == nil {
		t.Error("Expected error for invalid YAML")
	}
}

func TestFileLoader_Load_AppliesDefaults(t *testing.T) {
	minimalYAML := `
tenants:
  - tenant: minimal-tenant
    okapiUrl: http://okapi.example.com
`
	tmpFile := writeTempFile(t, minimalYAML)

	loader := &FileLoader{Path: tmpFile}
	cfgs, _, err := loader.Load()
	if err != nil {
		t.Fatalf("FileLoader.Load() failed: %v", err)
	}

	// applyTenantDefaults should fill these in
	if cfgs[0].MessageDelimiter != "\\r" {
		t.Errorf("Expected default MessageDelimiter '\\r', got %q", cfgs[0].MessageDelimiter)
	}
	if cfgs[0].FieldDelimiter != "|" {
		t.Errorf("Expected default FieldDelimiter '|', got %q", cfgs[0].FieldDelimiter)
	}
	if cfgs[0].Charset != "IBM850" {
		t.Errorf("Expected default Charset 'IBM850', got %q", cfgs[0].Charset)
	}
	if cfgs[0].Timezone != "America/New_York" {
		t.Errorf("Expected default Timezone 'America/New_York', got %q", cfgs[0].Timezone)
	}
	if cfgs[0].LogLevel != "None" {
		t.Errorf("Expected default LogLevel 'None', got %q", cfgs[0].LogLevel)
	}
}

func TestFileLoader_Load_InvalidRollingRenewals(t *testing.T) {
	badRollingYAML := `
tenants:
  - tenant: badtenant
    okapiUrl: http://localhost:9130
    rollingRenewals:
      enabled: true
      # missing renewWithin and extendFor
`
	tmpFile := writeTempFile(t, badRollingYAML)

	loader := &FileLoader{Path: tmpFile}
	_, _, err := loader.Load()
	if err == nil {
		t.Error("Expected error for invalid rolling renewals config")
	}
}

func TestFileLoader_Load_WithRollingRenewals(t *testing.T) {
	tmpFile := writeTempFile(t, validTenantYAMLWithRollingRenewals)

	loader := &FileLoader{Path: tmpFile}
	cfgs, _, err := loader.Load()
	if err != nil {
		t.Fatalf("FileLoader.Load() with rolling renewals failed: %v", err)
	}
	if cfgs[0].RollingRenewals == nil {
		t.Error("Expected rolling renewals to be configured")
	}
	if !cfgs[0].RollingRenewals.Enabled {
		t.Error("Expected rolling renewals to be enabled")
	}
}

// TestFileLoader_Load_FlatFormat_ReturnsError guards against regression of the old
// flat single-tenant format. The list format is the only supported format.
func TestFileLoader_Load_FlatFormat_ReturnsError(t *testing.T) {
	flatYAML := `
tenant: oldtenant
okapiUrl: http://localhost:9130
okapiTenant: oldtenant
`
	tmpFile := writeTempFile(t, flatYAML)

	loader := &FileLoader{Path: tmpFile}
	_, _, err := loader.Load()
	if err == nil {
		t.Error("Expected error for flat (non-list) format YAML")
	}
}

func TestFileLoader_Load_EmptyTenantsList_ReturnsError(t *testing.T) {
	emptyListYAML := `
tenants: []
`
	tmpFile := writeTempFile(t, emptyListYAML)

	loader := &FileLoader{Path: tmpFile}
	_, _, err := loader.Load()
	if err == nil {
		t.Error("Expected error for empty tenants list")
	}
}

func TestFileLoader_Load_MissingTenantName_ReturnsError(t *testing.T) {
	noNameYAML := `
tenants:
  - okapiUrl: http://localhost:9130
    okapiTenant: unnamed
`
	tmpFile := writeTempFile(t, noNameYAML)

	loader := &FileLoader{Path: tmpFile}
	_, _, err := loader.Load()
	if err == nil {
		t.Error("Expected error for tenant entry missing 'tenant' field")
	}
}

func TestFileLoader_Load_ListFormat_MultiTenant(t *testing.T) {
	multiYAML := `
tenants:
  - tenant: alpha
    okapiUrl: http://alpha.example.com
    okapiTenant: alpha
  - tenant: beta
    okapiUrl: http://beta.example.com
    okapiTenant: beta
`
	tmpFile := writeTempFile(t, multiYAML)

	loader := &FileLoader{Path: tmpFile}
	cfgs, _, err := loader.Load()
	if err != nil {
		t.Fatalf("FileLoader.Load() failed: %v", err)
	}
	if len(cfgs) != 2 {
		t.Fatalf("Expected 2 tenants, got %d", len(cfgs))
	}
	names := map[string]bool{cfgs[0].Tenant: true, cfgs[1].Tenant: true}
	if !names["alpha"] || !names["beta"] {
		t.Errorf("Expected tenants 'alpha' and 'beta', got %v", names)
	}
	// Defaults applied to each
	for _, cfg := range cfgs {
		if cfg.Charset != "IBM850" {
			t.Errorf("tenant %q: expected default Charset 'IBM850', got %q", cfg.Tenant, cfg.Charset)
		}
	}
}

func TestLoadTenantConfigsOrdered(t *testing.T) {
	multiYAML := `
tenants:
  - tenant: first
    okapiUrl: http://first.example.com
    okapiTenant: first
  - tenant: second
    okapiUrl: http://second.example.com
    okapiTenant: second
  - tenant: third
    okapiUrl: http://third.example.com
    okapiTenant: third
`
	tmpFile := writeTempFile(t, multiYAML)

	cfg := &Config{
		TenantConfigSources: []ConfigSource{
			{Type: "file", Path: tmpFile},
		},
		Tenants: make(map[string]*TenantConfig),
	}
	if err := cfg.loadTenantConfigs(); err != nil {
		t.Fatalf("loadTenantConfigs() failed: %v", err)
	}

	// TenantsOrdered must be non-nil and non-empty
	if len(cfg.TenantsOrdered) == 0 {
		t.Fatal("Expected TenantsOrdered to be non-empty after loading a multi-tenant config")
	}

	// Length must match the Tenants map
	if len(cfg.TenantsOrdered) != len(cfg.Tenants) {
		t.Errorf("TenantsOrdered length %d does not match Tenants map length %d",
			len(cfg.TenantsOrdered), len(cfg.Tenants))
	}

	// Order must match YAML declaration order
	wantOrder := []string{"first", "second", "third"}
	for i, tc := range cfg.TenantsOrdered {
		if tc.Tenant != wantOrder[i] {
			t.Errorf("TenantsOrdered[%d]: expected %q, got %q", i, wantOrder[i], tc.Tenant)
		}
	}
}

func TestFileLoader_Load_ListFormat_InvalidRollingRenewals(t *testing.T) {
	// One tenant has invalid rolling renewals — error should include the tenant name.
	badYAML := `
tenants:
  - tenant: goodtenant
    okapiUrl: http://localhost:9130
  - tenant: badtenant
    okapiUrl: http://localhost:9130
    rollingRenewals:
      enabled: true
      # missing renewWithin and extendFor
`
	tmpFile := writeTempFile(t, badYAML)

	loader := &FileLoader{Path: tmpFile}
	_, _, err := loader.Load()
	if err == nil {
		t.Error("Expected error for invalid rolling renewals in list format")
	}
}

// TestFileLoader_Load_ListFormat_DuplicateTenantName documents that the loader returns
// both entries when tenant names are duplicated. Last-write-wins when the caller inserts
// into its map — this is consistent with multi-source behavior and is not an error at
// the loader level.
func TestFileLoader_Load_ListFormat_DuplicateTenantName(t *testing.T) {
	dupYAML := `
tenants:
  - tenant: dupe
    okapiUrl: http://first.example.com
  - tenant: dupe
    okapiUrl: http://second.example.com
`
	tmpFile := writeTempFile(t, dupYAML)

	loader := &FileLoader{Path: tmpFile}
	cfgs, _, err := loader.Load()
	if err != nil {
		t.Fatalf("FileLoader.Load() failed: %v", err)
	}
	// Loader returns both entries; caller map semantics determine final value.
	if len(cfgs) != 2 {
		t.Errorf("Expected 2 entries for duplicate tenant name, got %d", len(cfgs))
	}
}

func TestHTTPLoader_Load_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yaml")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(validTenantYAML))
	}))
	defer server.Close()

	loader := &HTTPLoader{URL: server.URL + "/config"}
	cfgs, _, err := loader.Load()
	if err != nil {
		t.Fatalf("HTTPLoader.Load() failed: %v", err)
	}

	if cfgs[0].Tenant != "testlib" {
		t.Errorf("Expected tenant 'testlib', got %q", cfgs[0].Tenant)
	}
}

func TestHTTPLoader_Load_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("server error"))
	}))
	defer server.Close()

	loader := &HTTPLoader{URL: server.URL + "/config"}
	_, _, err := loader.Load()
	if err == nil {
		t.Error("Expected error for server error response")
	}
}

func TestHTTPLoader_Load_Non200Status(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("not found"))
	}))
	defer server.Close()

	loader := &HTTPLoader{URL: server.URL + "/config"}
	_, _, err := loader.Load()
	if err == nil {
		t.Error("Expected error for non-200 status")
	}
}

func TestHTTPLoader_Load_InvalidYAML(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yaml")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(invalidYAML))
	}))
	defer server.Close()

	loader := &HTTPLoader{URL: server.URL + "/config"}
	_, _, err := loader.Load()
	if err == nil {
		t.Error("Expected error for invalid YAML from HTTP")
	}
}

func TestHTTPLoader_Load_AppliesDefaults(t *testing.T) {
	minimalYAML := `
tenants:
  - tenant: http-tenant
    okapiUrl: http://okapi.example.com
`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yaml")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(minimalYAML))
	}))
	defer server.Close()

	loader := &HTTPLoader{URL: server.URL + "/config"}
	cfgs, _, err := loader.Load()
	if err != nil {
		t.Fatalf("HTTPLoader.Load() failed: %v", err)
	}

	if cfgs[0].Charset != "IBM850" {
		t.Errorf("Expected default Charset 'IBM850', got %q", cfgs[0].Charset)
	}
}

func TestHTTPLoader_Load_InvalidURL(t *testing.T) {
	loader := &HTTPLoader{URL: "http://127.0.0.1:1/nonexistent"}
	_, _, err := loader.Load()
	if err == nil {
		t.Error("Expected error for unreachable URL")
	}
}

func TestHTTPLoader_Load_InvalidRollingRenewals(t *testing.T) {
	badYAML := `
tenants:
  - tenant: badtenant
    okapiUrl: http://localhost:9130
    rollingRenewals:
      enabled: true
`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yaml")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(badYAML))
	}))
	defer server.Close()

	loader := &HTTPLoader{URL: server.URL + "/config"}
	_, _, err := loader.Load()
	if err == nil {
		t.Error("Expected error for invalid rolling renewals in HTTP config")
	}
}

func TestHTTPLoader_Load_ListFormat(t *testing.T) {
	multiYAML := `
tenants:
  - tenant: http-alpha
    okapiUrl: http://alpha.example.com
    okapiTenant: http-alpha
  - tenant: http-beta
    okapiUrl: http://beta.example.com
    okapiTenant: http-beta
`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yaml")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(multiYAML))
	}))
	defer server.Close()

	loader := &HTTPLoader{URL: server.URL + "/config"}
	cfgs, _, err := loader.Load()
	if err != nil {
		t.Fatalf("HTTPLoader.Load() failed: %v", err)
	}
	if len(cfgs) != 2 {
		t.Fatalf("Expected 2 tenants, got %d", len(cfgs))
	}
	names := map[string]bool{cfgs[0].Tenant: true, cfgs[1].Tenant: true}
	if !names["http-alpha"] || !names["http-beta"] {
		t.Errorf("Expected tenants 'http-alpha' and 'http-beta', got %v", names)
	}
}

func TestHTTPLoader_Load_FlatFormat_ReturnsError(t *testing.T) {
	flatYAML := `tenant: oldtenant
okapiUrl: http://localhost:9130
okapiTenant: oldtenant
`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yaml")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(flatYAML))
	}))
	defer server.Close()

	loader := &HTTPLoader{URL: server.URL + "/config"}
	_, _, err := loader.Load()
	if err == nil {
		t.Error("Expected error for flat (non-list) format YAML from HTTP")
	}
}

func TestApplyTenantDefaults(t *testing.T) {
	tc := &TenantConfig{}
	applyTenantDefaults(tc)

	if tc.MessageDelimiter != "\\r" {
		t.Errorf("Expected '\\r', got %q", tc.MessageDelimiter)
	}
	if tc.FieldDelimiter != "|" {
		t.Errorf("Expected '|', got %q", tc.FieldDelimiter)
	}
	if tc.Charset != "IBM850" {
		t.Errorf("Expected 'IBM850', got %q", tc.Charset)
	}
	if tc.Timezone != "America/New_York" {
		t.Errorf("Expected 'America/New_York', got %q", tc.Timezone)
	}
	if tc.LogLevel != "None" {
		t.Errorf("Expected 'None', got %q", tc.LogLevel)
	}
}

func TestApplyTenantDefaults_DoesNotOverrideExisting(t *testing.T) {
	tc := &TenantConfig{
		MessageDelimiter: "\r\n",
		FieldDelimiter:   "^",
		Charset:          "UTF-8",
		Timezone:         "UTC",
		LogLevel:         "debug",
	}
	applyTenantDefaults(tc)

	if tc.MessageDelimiter != "\r\n" {
		t.Errorf("Expected '\\r\\n', got %q", tc.MessageDelimiter)
	}
	if tc.FieldDelimiter != "^" {
		t.Errorf("Expected '^', got %q", tc.FieldDelimiter)
	}
	if tc.Charset != "UTF-8" {
		t.Errorf("Expected 'UTF-8', got %q", tc.Charset)
	}
}

func TestS3Loader_Load_FailsWithoutCredentials(t *testing.T) {
	// S3Loader.Load should fail when GetObject fails (no real S3 available)
	loader := &S3Loader{
		Bucket: "nonexistent-bucket",
		Key:    "nonexistent-key",
		Region: "us-east-1",
	}
	_, _, err := loader.Load()
	if err == nil {
		t.Error("Expected error from S3Loader.Load without valid S3 credentials")
	}
}

// writeTempFile creates a temp file with given content and returns its path
func writeTempFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}
	return path
}
