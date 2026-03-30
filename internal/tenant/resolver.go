package tenant

import (
	"context"

	"github.com/spokanepubliclibrary/fsip2/internal/config"
)

// ResolutionPhase represents the phase when tenant resolution occurs
type ResolutionPhase int

const (
	// PhaseConnect is the TCP connection phase (IP, port available)
	PhaseConnect ResolutionPhase = iota
	// PhaseLogin is the SIP2 LOGIN message phase (all message fields available)
	PhaseLogin
)

// String returns the string representation of the resolution phase
func (rp ResolutionPhase) String() string {
	switch rp {
	case PhaseConnect:
		return "CONNECT"
	case PhaseLogin:
		return "LOGIN"
	default:
		return "UNKNOWN"
	}
}

// ResolverData contains data available for tenant resolution
type ResolverData struct {
	// Connection data (available in CONNECT phase)
	ClientIP   string
	ClientPort int
	ServerPort int

	// Login message data (available in LOGIN phase)
	Username     string
	LocationCode string

	// Current tenant config (for context)
	CurrentTenant *config.TenantConfig
}

// Resolver is an interface for resolving tenants based on connection/message data
type Resolver interface {
	// Resolve attempts to resolve a tenant configuration
	Resolve(ctx context.Context, data *ResolverData) (*config.TenantConfig, error)

	// Phase returns the resolution phase this resolver operates in
	Phase() ResolutionPhase

	// Name returns the name of this resolver
	Name() string

	// Priority returns the priority of this resolver (higher = earlier)
	Priority() int
}

// BaseResolver provides common functionality for resolvers
type BaseResolver struct {
	name     string
	phase    ResolutionPhase
	priority int
}

// NewBaseResolver creates a new base resolver
func NewBaseResolver(name string, phase ResolutionPhase, priority int) *BaseResolver {
	return &BaseResolver{
		name:     name,
		phase:    phase,
		priority: priority,
	}
}

// Phase returns the resolution phase
func (br *BaseResolver) Phase() ResolutionPhase {
	return br.phase
}

// Name returns the resolver name
func (br *BaseResolver) Name() string {
	return br.name
}

// Priority returns the resolver priority
func (br *BaseResolver) Priority() int {
	return br.priority
}

// ByPriority implements sort.Interface for []Resolver based on priority
type ByPriority []Resolver

func (a ByPriority) Len() int           { return len(a) }
func (a ByPriority) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByPriority) Less(i, j int) bool { return a[i].Priority() > a[j].Priority() }
