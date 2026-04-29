package tenant

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/seancfoley/ipaddress-go/ipaddr"
	"github.com/spokanepubliclibrary/fsip2/internal/config"
)

// Service manages tenant resolution using multiple resolvers
type Service struct {
	mu               sync.RWMutex
	connectResolvers []Resolver
	loginResolvers   []Resolver
	defaultConfig    *config.TenantConfig
	tenantConfigs    map[string]*config.TenantConfig
	scTenants        []config.SCTenantConfig
}

// NewService creates a new tenant resolution service
func NewService(cfg *config.Config) *Service {
	s := &Service{
		connectResolvers: []Resolver{},
		loginResolvers:   []Resolver{},
		defaultConfig:    nil,
		tenantConfigs:    cfg.GetTenants(),
		scTenants:        cfg.GetSCTenants(),
	}

	// Priority 1: explicit catch-all — scTenant with no routing rules
	for _, scTenant := range cfg.GetSCTenants() {
		if scTenant.SCSubnet == "" && scTenant.Port == 0 &&
			len(scTenant.LocationCodes) == 0 && len(scTenant.UsernamePrefixes) == 0 {
			if tenantCfg, ok := cfg.GetTenants()[scTenant.Tenant]; ok {
				s.defaultConfig = tenantCfg
				break
			}
		}
	}

	// Priority 2: first tenant in declaration order not referenced by any scTenant
	if s.defaultConfig == nil {
		referencedTenants := make(map[string]bool)
		for _, scTenant := range cfg.GetSCTenants() {
			referencedTenants[scTenant.Tenant] = true
		}
		for _, tenantCfg := range cfg.GetTenantsOrdered() {
			if !referencedTenants[tenantCfg.Tenant] {
				s.defaultConfig = tenantCfg
				break
			}
		}
	}

	// Priority 3: absolute fallback — first declared tenant
	if s.defaultConfig == nil && len(cfg.GetTenantsOrdered()) > 0 {
		s.defaultConfig = cfg.GetTenantsOrdered()[0]
	}

	// Priority 4: last-resort fallback — any tenant from the map (handles configs
	// built without TenantsOrdered, e.g. in tests or minimal programmatic configs)
	if s.defaultConfig == nil {
		for _, tenantCfg := range cfg.GetTenants() {
			s.defaultConfig = tenantCfg
			break
		}
	}

	// Initialize default resolvers
	s.initializeResolvers(cfg)

	return s
}

// Reinitialize reloads tenant configuration from a new config
func (s *Service) Reinitialize(cfg *config.Config) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.connectResolvers = []Resolver{}
	s.loginResolvers = []Resolver{}
	s.defaultConfig = nil
	s.tenantConfigs = cfg.GetTenants()
	s.scTenants = cfg.GetSCTenants()

	// Priority 1: explicit catch-all — scTenant with no routing rules
	for _, scTenant := range cfg.GetSCTenants() {
		if scTenant.SCSubnet == "" && scTenant.Port == 0 &&
			len(scTenant.LocationCodes) == 0 && len(scTenant.UsernamePrefixes) == 0 {
			if tenantCfg, ok := cfg.GetTenants()[scTenant.Tenant]; ok {
				s.defaultConfig = tenantCfg
				break
			}
		}
	}

	// Priority 2: first tenant in declaration order not referenced by any scTenant
	if s.defaultConfig == nil {
		referencedTenants := make(map[string]bool)
		for _, scTenant := range cfg.GetSCTenants() {
			referencedTenants[scTenant.Tenant] = true
		}
		for _, tenantCfg := range cfg.GetTenantsOrdered() {
			if !referencedTenants[tenantCfg.Tenant] {
				s.defaultConfig = tenantCfg
				break
			}
		}
	}

	// Priority 3: absolute fallback — first declared tenant
	if s.defaultConfig == nil && len(cfg.GetTenantsOrdered()) > 0 {
		s.defaultConfig = cfg.GetTenantsOrdered()[0]
	}

	// Priority 4: last-resort fallback — any tenant from the map (handles configs
	// built without TenantsOrdered, e.g. in tests or minimal programmatic configs)
	if s.defaultConfig == nil {
		for _, tenantCfg := range cfg.GetTenants() {
			s.defaultConfig = tenantCfg
			break
		}
	}

	s.initializeResolvers(cfg)
}

// initializeResolvers sets up the default resolvers
func (s *Service) initializeResolvers(cfg *config.Config) {
	// Create resolvers for each SC tenant entry, looking up the full TenantConfig by name
	for _, scTenant := range cfg.GetSCTenants() {
		tenantCfg, ok := cfg.GetTenants()[scTenant.Tenant]
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
	s.mu.RLock()
	defer s.mu.RUnlock()

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

// ipInSubnet reports whether clientIP falls within the given subnet string (CIDR or plain IP).
// It mirrors the logic in IPResolver.Resolve, using the ipaddress-go library.
func ipInSubnet(clientIP, subnet string) bool {
	ipSubnet := ipaddr.NewIPAddressString(subnet)
	if ipSubnet.GetAddress() == nil {
		return false
	}
	clientIPAddr := ipaddr.NewIPAddressString(clientIP)
	if clientIPAddr.GetAddress() == nil {
		return false
	}
	return ipSubnet.Contains(clientIPAddr)
}

// hasAnyPrefix reports whether s starts with any of the given prefixes.
// It mirrors the matching logic in UsernamePrefixResolver.Resolve.
func hasAnyPrefix(s string, prefixes []string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}

// containsStr reports whether target is present in the slice.
// It mirrors the matching logic in LocationCodeResolver.Resolve.
func containsStr(slice []string, target string) bool {
	for _, v := range slice {
		if v == target {
			return true
		}
	}
	return false
}

// ResolveComplete performs a single holistic tenant resolution at login time.
// It walks s.scTenants in declaration order and returns the first entry where
// every rule that is present matches. Rules absent from an entry are treated as
// "don't care" (wildcard). If no entry matches, s.defaultConfig is returned.
func (s *Service) ResolveComplete(
	ctx context.Context,
	serverPort int,
	clientIP string,
	username string,
	locationCode string,
) (*config.TenantConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, sc := range s.scTenants {
		// Port rule
		if sc.Port > 0 && sc.Port != serverPort {
			continue
		}

		// Subnet rule
		if sc.SCSubnet != "" && !ipInSubnet(clientIP, sc.SCSubnet) {
			continue
		}

		// Username prefix rule
		if len(sc.UsernamePrefixes) > 0 && !hasAnyPrefix(username, sc.UsernamePrefixes) {
			continue
		}

		// Location code rule
		if len(sc.LocationCodes) > 0 && !containsStr(sc.LocationCodes, locationCode) {
			continue
		}

		// All present rules matched — look up the full TenantConfig
		tenantCfg, ok := s.tenantConfigs[sc.Tenant]
		if !ok {
			// Misconfigured entry: tenant name not in the tenant map — skip
			continue
		}
		return tenantCfg, nil
	}

	// Nothing matched
	if s.defaultConfig == nil {
		return nil, fmt.Errorf("no tenant configuration available")
	}
	return s.defaultConfig, nil
}

// GetDefaultTenant returns the default tenant configuration
func (s *Service) GetDefaultTenant() *config.TenantConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.defaultConfig
}

// GetTenantByName retrieves a tenant configuration by name
func (s *Service) GetTenantByName(tenantName string) (*config.TenantConfig, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	tenant, ok := s.tenantConfigs[tenantName]
	return tenant, ok
}

// GetAllTenants returns all tenant configurations
func (s *Service) GetAllTenants() map[string]*config.TenantConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.tenantConfigs
}

// GetResolverCount returns the number of resolvers by phase
func (s *Service) GetResolverCount(phase ResolutionPhase) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	switch phase {
	case PhaseConnect:
		return len(s.connectResolvers)
	case PhaseLogin:
		return len(s.loginResolvers)
	default:
		return 0
	}
}
