package tenant

import (
	"context"
	"fmt"
	"strings"

	"github.com/spokanepubliclibrary/fsip2/internal/config"
)

// UsernamePrefixResolver resolves tenants based on username prefix
type UsernamePrefixResolver struct {
	*BaseResolver
	prefixes     []string
	tenantConfig *config.TenantConfig
}

// NewUsernamePrefixResolver creates a new username prefix resolver
func NewUsernamePrefixResolver(prefixes []string, tenantConfig *config.TenantConfig) *UsernamePrefixResolver {
	return &UsernamePrefixResolver{
		BaseResolver: NewBaseResolver("UsernamePrefix", PhaseLogin, 70), // Lower priority than location
		prefixes:     prefixes,
		tenantConfig: tenantConfig,
	}
}

// Resolve resolves a tenant based on username prefix
func (r *UsernamePrefixResolver) Resolve(ctx context.Context, data *ResolverData) (*config.TenantConfig, error) {
	if data.Username == "" {
		return nil, fmt.Errorf("no username provided")
	}

	// Check if username starts with any configured prefix
	for _, prefix := range r.prefixes {
		if strings.HasPrefix(data.Username, prefix) {
			return r.tenantConfig, nil
		}
	}

	return nil, nil
}
