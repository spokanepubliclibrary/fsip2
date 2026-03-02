package config

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

const validTenantYAML = `
tenant: testlib
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
tenant: testlib2
okapiUrl: http://localhost:9130
okapiTenant: testlib2
fieldDelimiter: "|"
rollingRenewals:
  enabled: true
  renewWithin: "6M"
  extendFor: "1Y"
`

func TestFileLoader_Load_Success(t *testing.T) {
	// Create temp file with valid config
	tmpFile := writeTempFile(t, validTenantYAML)

	loader := &FileLoader{Path: tmpFile}
	cfg, err := loader.Load()
	if err != nil {
		t.Fatalf("FileLoader.Load() failed: %v", err)
	}

	if cfg.Tenant != "testlib" {
		t.Errorf("Expected tenant 'testlib', got %q", cfg.Tenant)
	}
	if cfg.OkapiURL != "http://localhost:9130" {
		t.Errorf("Expected OkapiURL 'http://localhost:9130', got %q", cfg.OkapiURL)
	}
}

func TestFileLoader_Load_FileNotFound(t *testing.T) {
	loader := &FileLoader{Path: "/nonexistent/path/to/config.yaml"}
	_, err := loader.Load()
	if err == nil {
		t.Error("Expected error for nonexistent file")
	}
}

func TestFileLoader_Load_InvalidYAML(t *testing.T) {
	tmpFile := writeTempFile(t, invalidYAML)

	loader := &FileLoader{Path: tmpFile}
	_, err := loader.Load()
	if err == nil {
		t.Error("Expected error for invalid YAML")
	}
}

func TestFileLoader_Load_AppliesDefaults(t *testing.T) {
	// Minimal config - should have defaults applied
	minimalYAML := `
tenant: minimal-tenant
okapiUrl: http://okapi.example.com
`
	tmpFile := writeTempFile(t, minimalYAML)

	loader := &FileLoader{Path: tmpFile}
	cfg, err := loader.Load()
	if err != nil {
		t.Fatalf("FileLoader.Load() failed: %v", err)
	}

	// applyTenantDefaults should fill these in
	if cfg.MessageDelimiter != "\\r" {
		t.Errorf("Expected default MessageDelimiter '\\r', got %q", cfg.MessageDelimiter)
	}
	if cfg.FieldDelimiter != "|" {
		t.Errorf("Expected default FieldDelimiter '|', got %q", cfg.FieldDelimiter)
	}
	if cfg.Charset != "IBM850" {
		t.Errorf("Expected default Charset 'IBM850', got %q", cfg.Charset)
	}
	if cfg.Timezone != "America/New_York" {
		t.Errorf("Expected default Timezone 'America/New_York', got %q", cfg.Timezone)
	}
	if cfg.LogLevel != "None" {
		t.Errorf("Expected default LogLevel 'None', got %q", cfg.LogLevel)
	}
}

func TestFileLoader_Load_InvalidRollingRenewals(t *testing.T) {
	badRollingYAML := `
tenant: badtenant
okapiUrl: http://localhost:9130
rollingRenewals:
  enabled: true
  # missing renewWithin and extendFor
`
	tmpFile := writeTempFile(t, badRollingYAML)

	loader := &FileLoader{Path: tmpFile}
	_, err := loader.Load()
	if err == nil {
		t.Error("Expected error for invalid rolling renewals config")
	}
}

func TestFileLoader_Load_WithRollingRenewals(t *testing.T) {
	tmpFile := writeTempFile(t, validTenantYAMLWithRollingRenewals)

	loader := &FileLoader{Path: tmpFile}
	cfg, err := loader.Load()
	if err != nil {
		t.Fatalf("FileLoader.Load() with rolling renewals failed: %v", err)
	}
	if cfg.RollingRenewals == nil {
		t.Error("Expected rolling renewals to be configured")
	}
	if !cfg.RollingRenewals.Enabled {
		t.Error("Expected rolling renewals to be enabled")
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
	cfg, err := loader.Load()
	if err != nil {
		t.Fatalf("HTTPLoader.Load() failed: %v", err)
	}

	if cfg.Tenant != "testlib" {
		t.Errorf("Expected tenant 'testlib', got %q", cfg.Tenant)
	}
}

func TestHTTPLoader_Load_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("server error"))
	}))
	defer server.Close()

	loader := &HTTPLoader{URL: server.URL + "/config"}
	_, err := loader.Load()
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
	_, err := loader.Load()
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
	_, err := loader.Load()
	if err == nil {
		t.Error("Expected error for invalid YAML from HTTP")
	}
}

func TestHTTPLoader_Load_AppliesDefaults(t *testing.T) {
	minimalYAML := `tenant: http-tenant
okapiUrl: http://okapi.example.com
`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yaml")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(minimalYAML))
	}))
	defer server.Close()

	loader := &HTTPLoader{URL: server.URL + "/config"}
	cfg, err := loader.Load()
	if err != nil {
		t.Fatalf("HTTPLoader.Load() failed: %v", err)
	}

	if cfg.Charset != "IBM850" {
		t.Errorf("Expected default Charset 'IBM850', got %q", cfg.Charset)
	}
}

func TestHTTPLoader_Load_InvalidURL(t *testing.T) {
	loader := &HTTPLoader{URL: "http://127.0.0.1:1/nonexistent"}
	_, err := loader.Load()
	if err == nil {
		t.Error("Expected error for unreachable URL")
	}
}

func TestHTTPLoader_Load_InvalidRollingRenewals(t *testing.T) {
	badYAML := `tenant: badtenant
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
	_, err := loader.Load()
	if err == nil {
		t.Error("Expected error for invalid rolling renewals in HTTP config")
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
	_, err := loader.Load()
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
