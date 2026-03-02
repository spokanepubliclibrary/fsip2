package config

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"gopkg.in/yaml.v3"
)

// TenantConfigLoader is an interface for loading tenant configurations from various sources
type TenantConfigLoader interface {
	Load() (*TenantConfig, error)
}

// FileLoader loads tenant configuration from a local file
type FileLoader struct {
	Path string
}

// Load implements TenantConfigLoader
func (f *FileLoader) Load() (*TenantConfig, error) {
	data, err := os.ReadFile(f.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var tenantCfg TenantConfig
	if err := yaml.Unmarshal(data, &tenantCfg); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	// Set defaults
	f.setDefaults(&tenantCfg)

	// Validate rolling renewals configuration
	if err := tenantCfg.ValidateRollingRenewals(); err != nil {
		return nil, fmt.Errorf("invalid rolling renewals configuration: %w", err)
	}

	// Validate patron custom fields configuration
	if err := tenantCfg.ValidatePatronCustomFields(); err != nil {
		return nil, fmt.Errorf("invalid patron custom fields configuration: %w", err)
	}

	return &tenantCfg, nil
}

// HTTPLoader loads tenant configuration from an HTTP endpoint
type HTTPLoader struct {
	URL string
}

// Load implements TenantConfigLoader
func (h *HTTPLoader) Load() (*TenantConfig, error) {
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := client.Get(h.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch from HTTP: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP request failed with status: %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var tenantCfg TenantConfig
	if err := yaml.Unmarshal(data, &tenantCfg); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	// Set defaults
	h.setDefaults(&tenantCfg)

	// Validate rolling renewals configuration
	if err := tenantCfg.ValidateRollingRenewals(); err != nil {
		return nil, fmt.Errorf("invalid rolling renewals configuration: %w", err)
	}

	// Validate patron custom fields configuration
	if err := tenantCfg.ValidatePatronCustomFields(); err != nil {
		return nil, fmt.Errorf("invalid patron custom fields configuration: %w", err)
	}

	return &tenantCfg, nil
}

// S3Loader loads tenant configuration from AWS S3
type S3Loader struct {
	Bucket string
	Key    string
	Region string
}

// Load implements TenantConfigLoader
func (s *S3Loader) Load() (*TenantConfig, error) {
	ctx := context.Background()

	// Load AWS configuration
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(s.Region))
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Create S3 client
	client := s3.NewFromConfig(cfg)

	// Get object from S3
	result, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.Bucket),
		Key:    aws.String(s.Key),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get object from S3: %w", err)
	}
	defer result.Body.Close()

	// Read response body
	data, err := io.ReadAll(result.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read S3 object: %w", err)
	}

	var tenantCfg TenantConfig
	if err := yaml.Unmarshal(data, &tenantCfg); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	// Set defaults
	s.setDefaults(&tenantCfg)

	// Validate rolling renewals configuration
	if err := tenantCfg.ValidateRollingRenewals(); err != nil {
		return nil, fmt.Errorf("invalid rolling renewals configuration: %w", err)
	}

	// Validate patron custom fields configuration
	if err := tenantCfg.ValidatePatronCustomFields(); err != nil {
		return nil, fmt.Errorf("invalid patron custom fields configuration: %w", err)
	}

	return &tenantCfg, nil
}

// setDefaults sets default values for tenant configuration
func (f *FileLoader) setDefaults(tc *TenantConfig) {
	applyTenantDefaults(tc)
}

func (h *HTTPLoader) setDefaults(tc *TenantConfig) {
	applyTenantDefaults(tc)
}

func (s *S3Loader) setDefaults(tc *TenantConfig) {
	applyTenantDefaults(tc)
}

func applyTenantDefaults(tc *TenantConfig) {
	if tc.MessageDelimiter == "" {
		tc.MessageDelimiter = "\\r"
	}
	if tc.FieldDelimiter == "" {
		tc.FieldDelimiter = "|"
	}
	if tc.Charset == "" {
		tc.Charset = "IBM850"
	}
	if tc.Timezone == "" {
		tc.Timezone = "America/New_York"
	}
	if tc.LogLevel == "" {
		tc.LogLevel = "None"
	}
}
