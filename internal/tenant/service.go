package tenant

import (
	"context"
	"fmt"
	"sort"

	"github.com/spokanepubliclibrary/fsip2/internal/config"
)

// Service manages tenant resolution using multiple resolvers
type Service struct {
	connectResolvers []Resolver
	loginResolvers   []Resolver
	defaultConfig    *config.TenantConfig
	tenantConfigs    map[string]*config.TenantConfig
}

// NewService creates a new tenant resolution service
func NewService(cfg *config.Config) *Service {
	s := &Service{
		connectResolvers: []Resolver{},
		loginResolvers:   []Resolver{},
		defaultConfig:    nil,
		tenantConfigs:    cfg.Tenants,
	}

	// Find default tenant config: use first tenant not referenced by any SCTenant entry
	referencedTenants := make(map[string]bool)
	for _, scTenant := range cfg.SCTenants {
		referencedTenants[scTenant.Tenant] = true
	}
	for _, tenantCfg := range cfg.Tenants {
		if !referencedTenants[tenantCfg.Tenant] {
			s.defaultConfig = tenantCfg
			break
		}
	}

	// If no default found, use first tenant
	if s.defaultConfig == nil && len(cfg.Tenants) > 0 {
		for _, tenantCfg := range cfg.Tenants {
			s.defaultConfig = tenantCfg
			break
		}
	}

	// Initialize default resolvers
	s.initializeResolvers(cfg)

	return s
}

// initializeResolvers sets up the default resolvers
func (s *Service) initializeResolvers(cfg *config.Config) {
	// Create resolvers for each SC tenant entry, looking up the full TenantConfig by name
	for _, scTenant := range cfg.SCTenants {
		tenantCfg, ok := cfg.Tenants[scTenant.Tenant]
		if !ok {
			// Referenced tenant is not defined in Tenants map — skip
			continue
		}

		// Add IP resolver if subnet is configured
		if scTenant.SCSubnet != "" {
			s.AddResolver(NewIPResolver(scTenant.SCSubnet, tenantCfg))
		}

		// Add port resolver if port is configured
		if scTenant.Port > 0 {
			s.AddResolver(NewPortResolver(scTenant.Port, tenantCfg))
		}

		// Add location code resolver if location codes are configured
		if len(scTenant.LocationCodes) > 0 {
			s.AddResolver(NewLocationCodeResolver(scTenant.LocationCodes, tenantCfg))
		}

		// Add username prefix resolver if prefixes are configured
		if len(scTenant.UsernamePrefixes) > 0 {
			s.AddResolver(NewUsernamePrefixResolver(scTenant.UsernamePrefixes, tenantCfg))
		}
	}

	// Sort resolvers by priority
	sort.Sort(ByPriority(s.connectResolvers))
	sort.Sort(ByPriority(s.loginResolvers))
}

// AddResolver adds a resolver to the service
func (s *Service) AddResolver(resolver Resolver) {
	switch resolver.Phase() {
	case PhaseConnect:
		s.connectResolvers = append(s.connectResolvers, resolver)
	case PhaseLogin:
		s.loginResolvers = append(s.loginResolvers, resolver)
	}
}

// ResolveAtConnect resolves tenant at connection time using IP and port
func (s *Service) ResolveAtConnect(ctx context.Context, clientIP string, clientPort int, serverPort int) (*config.TenantConfig, error) {
	data := &ResolverData{
		ClientIP:      clientIP,
		ClientPort:    clientPort,
		ServerPort:    serverPort,
		CurrentTenant: s.defaultConfig,
	}

	// Try each CONNECT phase resolver in priority order
	for _, resolver := range s.connectResolvers {
		tenantCfg, err := resolver.Resolve(ctx, data)
		if err != nil {
			// Log error but continue to next resolver
			continue
		}

		if tenantCfg != nil {
			return tenantCfg, nil
		}
	}

	// No resolver matched, return default
	if s.defaultConfig == nil {
		return nil, fmt.Errorf("no tenant configuration available")
	}

	return s.defaultConfig, nil
}

// ResolveAtLogin resolves tenant at LOGIN time using login message fields
func (s *Service) ResolveAtLogin(ctx context.Context, username, locationCode string, currentTenant *config.TenantConfig) (*config.TenantConfig, error) {
	data := &ResolverData{
		Username:      username,
		LocationCode:  locationCode,
		CurrentTenant: currentTenant,
	}

	// Try each LOGIN phase resolver in priority order
	for _, resolver := range s.loginResolvers {
		tenantCfg, err := resolver.Resolve(ctx, data)
		if err != nil {
			// Log error but continue to next resolver
			continue
		}

		if tenantCfg != nil {
			return tenantCfg, nil
		}
	}

	// No resolver matched, return current tenant
	if currentTenant == nil {
		return s.defaultConfig, nil
	}

	return currentTenant, nil
}

// GetDefaultTenant returns the default tenant configuration
func (s *Service) GetDefaultTenant() *config.TenantConfig {
	return s.defaultConfig
}

// GetTenantByName retrieves a tenant configuration by name
func (s *Service) GetTenantByName(tenantName string) (*config.TenantConfig, bool) {
	tenant, ok := s.tenantConfigs[tenantName]
	return tenant, ok
}

// GetAllTenants returns all tenant configurations
func (s *Service) GetAllTenants() map[string]*config.TenantConfig {
	return s.tenantConfigs
}

// GetResolverCount returns the number of resolvers by phase
func (s *Service) GetResolverCount(phase ResolutionPhase) int {
	switch phase {
	case PhaseConnect:
		return len(s.connectResolvers)
	case PhaseLogin:
		return len(s.loginResolvers)
	default:
		return 0
	}
}
