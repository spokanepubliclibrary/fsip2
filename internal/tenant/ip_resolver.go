package tenant

import (
	"context"
	"fmt"
	"net"

	"github.com/seancfoley/ipaddress-go/ipaddr"
	"github.com/spokanepubliclibrary/fsip2/internal/config"
)

// IPResolver resolves tenants based on client IP address (CIDR matching)
type IPResolver struct {
	*BaseResolver
	subnet       string
	tenantConfig *config.TenantConfig
}

// NewIPResolver creates a new IP/subnet resolver
func NewIPResolver(subnet string, tenantConfig *config.TenantConfig) *IPResolver {
	return &IPResolver{
		BaseResolver: NewBaseResolver("IP", PhaseConnect, 100), // High priority
		subnet:       subnet,
		tenantConfig: tenantConfig,
	}
}

// Resolve resolves a tenant based on client IP
func (r *IPResolver) Resolve(ctx context.Context, data *ResolverData) (*config.TenantConfig, error) {
	if data.ClientIP == "" {
		return nil, fmt.Errorf("no client IP provided")
	}

	// Parse the subnet
	ipSubnet := ipaddr.NewIPAddressString(r.subnet)
	if ipSubnet.GetAddress() == nil {
		return nil, fmt.Errorf("invalid subnet: %s", r.subnet)
	}

	// Parse the client IP
	clientIPAddr := ipaddr.NewIPAddressString(data.ClientIP)
	if clientIPAddr.GetAddress() == nil {
		return nil, fmt.Errorf("invalid client IP: %s", data.ClientIP)
	}

	// Check if client IP is in the subnet
	if ipSubnet.Contains(clientIPAddr) {
		return r.tenantConfig, nil
	}

	return nil, nil
}

// SimpleIPResolver is a simpler IP resolver using net package (fallback)
type SimpleIPResolver struct {
	*BaseResolver
	cidr         *net.IPNet
	tenantConfig *config.TenantConfig
}

// NewSimpleIPResolver creates a simple IP resolver
func NewSimpleIPResolver(cidr string, tenantConfig *config.TenantConfig) (*SimpleIPResolver, error) {
	_, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, fmt.Errorf("invalid CIDR: %w", err)
	}

	return &SimpleIPResolver{
		BaseResolver: NewBaseResolver("SimpleIP", PhaseConnect, 100),
		cidr:         ipnet,
		tenantConfig: tenantConfig,
	}, nil
}

// Resolve resolves a tenant based on client IP
func (r *SimpleIPResolver) Resolve(ctx context.Context, data *ResolverData) (*config.TenantConfig, error) {
	if data.ClientIP == "" {
		return nil, fmt.Errorf("no client IP provided")
	}

	clientIP := net.ParseIP(data.ClientIP)
	if clientIP == nil {
		return nil, fmt.Errorf("invalid client IP: %s", data.ClientIP)
	}

	if r.cidr.Contains(clientIP) {
		return r.tenantConfig, nil
	}

	return nil, nil
}
