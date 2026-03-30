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
	Load() ([]*TenantConfig, []SCTenantConfig, error)
}

// TenantFileConfig is the top-level structure of a tenant config YAML file.
// It supports both a "tenants:" list and an optional "scTenants:" list.
type TenantFileConfig struct {
	Tenants   []*TenantConfig  `yaml:"tenants"`
	SCTenants []SCTenantConfig `yaml:"scTenants,omitempty"`
}

// parseTenantConfigFile parses raw YAML, validates the tenants list, applies defaults,
// and validates each entry. It is shared across all loader types.
func parseTenantConfigFile(data []byte) ([]*TenantConfig, []SCTenantConfig, error) {
	var file TenantFileConfig
	if err := yaml.Unmarshal(data, &file); err != nil {
		return nil, nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	if len(file.Tenants) == 0 {
		return nil, nil, fmt.Errorf("tenant config file must contain a non-empty 'tenants' list")
	}

	for _, tc := range file.Tenants {
		if tc.Tenant == "" {
			return nil, nil, fmt.Errorf("each tenant entry must have a non-empty 'tenant' field")
		}
		applyTenantDefaults(tc)
		if err := tc.ValidateRollingRenewals(); err != nil {
			return nil, nil, fmt.Errorf("tenant %q: invalid rolling renewals: %w", tc.Tenant, err)
		}
		if err := tc.ValidatePatronCustomFields(); err != nil {
			return nil, nil, fmt.Errorf("tenant %q: invalid patron custom fields: %w", tc.Tenant, err)
		}
	}
	return file.Tenants, file.SCTenants, nil
}

// FileLoader loads tenant configuration from a local file
type FileLoader struct {
	Path string
}

// Load implements TenantConfigLoader
func (f *FileLoader) Load() ([]*TenantConfig, []SCTenantConfig, error) {
	data, err := os.ReadFile(f.Path)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read file: %w", err)
	}
	return parseTenantConfigFile(data)
}

// HTTPLoader loads tenant configuration from an HTTP endpoint
type HTTPLoader struct {
	URL string
}

// Load implements TenantConfigLoader
func (h *HTTPLoader) Load() ([]*TenantConfig, []SCTenantConfig, error) {
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := client.Get(h.URL)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to fetch from HTTP: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("HTTP request failed with status: %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read response body: %w", err)
	}
	return parseTenantConfigFile(data)
}

// S3Loader loads tenant configuration from AWS S3
type S3Loader struct {
	Bucket string
	Key    string
	Region string
}

// Load implements TenantConfigLoader
func (s *S3Loader) Load() ([]*TenantConfig, []SCTenantConfig, error) {
	ctx := context.Background()

	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(s.Region))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	client := s3.NewFromConfig(cfg)

	result, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.Bucket),
		Key:    aws.String(s.Key),
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get object from S3: %w", err)
	}
	defer result.Body.Close()

	data, err := io.ReadAll(result.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read S3 object: %w", err)
	}
	return parseTenantConfigFile(data)
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
