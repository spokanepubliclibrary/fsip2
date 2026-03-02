package testutil

import "github.com/spokanepubliclibrary/fsip2/internal/config"

// TenantOption is a functional option for building a TenantConfig.
type TenantOption func(*config.TenantConfig)

func WithOkapiURL(url string) TenantOption {
	return func(tc *config.TenantConfig) { tc.OkapiURL = url }
}

func WithTenant(tenant string) TenantOption {
	return func(tc *config.TenantConfig) { tc.Tenant = tenant }
}

func WithCurrency(currency string) TenantOption {
	return func(tc *config.TenantConfig) { tc.Currency = currency }
}

func WithErrorDetection(enabled bool) TenantOption {
	return func(tc *config.TenantConfig) { tc.ErrorDetectionEnabled = enabled }
}

func WithRollingRenewal(cfg *config.RollingRenewalConfig) TenantOption {
	return func(tc *config.TenantConfig) { tc.RollingRenewals = cfg }
}

// NewTenantConfig returns a TenantConfig suitable for tests.
// Defaults: test-tenant, loopback OkapiURL, "|" field delimiter, CR message delimiter, UTF-8, USD.
func NewTenantConfig(opts ...TenantOption) *config.TenantConfig {
	tc := &config.TenantConfig{
		Tenant:                "test-tenant",
		OkapiURL:              "http://127.0.0.1:9999",
		MessageDelimiter:      "\r",
		FieldDelimiter:        "|",
		Charset:               "UTF-8",
		ErrorDetectionEnabled: true,
		Currency:              "USD",
	}
	for _, opt := range opts {
		opt(tc)
	}
	return tc
}
