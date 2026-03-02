package tenant

import (
	"context"
	"fmt"

	"github.com/spokanepubliclibrary/fsip2/internal/config"
)

// LocationCodeResolver resolves tenants based on SIP2 location code (CP field in LOGIN message)
type LocationCodeResolver struct {
	*BaseResolver
	locationCodes []string
	tenantConfig  *config.TenantConfig
}

// NewLocationCodeResolver creates a new location code resolver
func NewLocationCodeResolver(locationCodes []string, tenantConfig *config.TenantConfig) *LocationCodeResolver {
	return &LocationCodeResolver{
		BaseResolver:  NewBaseResolver("LocationCode", PhaseLogin, 80),
		locationCodes: locationCodes,
		tenantConfig:  tenantConfig,
	}
}

// Resolve resolves a tenant based on location code
func (r *LocationCodeResolver) Resolve(ctx context.Context, data *ResolverData) (*config.TenantConfig, error) {
	if data.LocationCode == "" {
		return nil, fmt.Errorf("no location code provided")
	}

	// Check if location code matches any configured codes
	for _, code := range r.locationCodes {
		if data.LocationCode == code {
			return r.tenantConfig, nil
		}
	}

	return nil, nil
}
