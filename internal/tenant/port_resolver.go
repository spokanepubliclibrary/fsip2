package tenant

import (
	"context"
	"fmt"

	"github.com/spokanepubliclibrary/fsip2/internal/config"
)

// PortResolver resolves tenants based on server port
type PortResolver struct {
	*BaseResolver
	port         int
	tenantConfig *config.TenantConfig
}

// NewPortResolver creates a new port resolver
func NewPortResolver(port int, tenantConfig *config.TenantConfig) *PortResolver {
	return &PortResolver{
		BaseResolver: NewBaseResolver("Port", PhaseConnect, 90), // Slightly lower priority than IP
		port:         port,
		tenantConfig: tenantConfig,
	}
}

// Resolve resolves a tenant based on server port
func (r *PortResolver) Resolve(ctx context.Context, data *ResolverData) (*config.TenantConfig, error) {
	if data.ServerPort == 0 {
		return nil, fmt.Errorf("no server port provided")
	}

	if data.ServerPort == r.port {
		return r.tenantConfig, nil
	}

	return nil, nil
}
