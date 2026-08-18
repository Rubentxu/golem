// Package agent provides the agent domain model for GOLEM.
package agent

import (
	"fmt"
	"sync"

	"github.com/Rubentxu/golem/internal/ports"
)

// AgentPrincipal wraps ports.Actor with agent-specific claims and assurance.
// It enforces Type="agent" and provides access to Frame, TenantMemberships,
// and Groups (ADR-005 envelope compliance).
type AgentPrincipal struct {
	mu                sync.RWMutex
	type_             string // enforced to "agent"
	id                string
	claims            AgentClaims
	assurance         AgentAssurance
	frame             *ports.Frame
	tenantMemberships []string
	groups            []string
}

// AgentClaims holds agent-specific claims (e.g., capabilities, permissions).
type AgentClaims struct {
	LLMCapabilities   ports.LLMProviderCapabilities
	ToolPermissions   []ports.Permission
	EvaluationEnabled bool
}

// AgentAssurance provides trust level for the agent.
type AgentAssurance struct {
	Level       string // "trusted", "untrusted", "restricted"
	AuditEvents bool
}

// NewAgentPrincipal creates a new AgentPrincipal with the given ID.
// The Type is enforced to "agent" and cannot be changed after construction.
func NewAgentPrincipal(id string) *AgentPrincipal {
	return &AgentPrincipal{
		type_:     "agent",
		id:        id,
		assurance: AgentAssurance{Level: "untrusted", AuditEvents: true},
	}
}

// Type returns the actor type, always "agent".
func (a *AgentPrincipal) Type() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.type_
}

// ID returns the agent ID.
func (a *AgentPrincipal) ID() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.id
}

// SetClaims sets the agent claims. Not thread-safe after first call.
func (a *AgentPrincipal) SetClaims(c AgentClaims) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.claims = c
}

// Claims returns a copy of the agent claims.
func (a *AgentPrincipal) Claims() AgentClaims {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.claims
}

// SetAssurance sets the agent assurance level. Not thread-safe after first call.
func (a *AgentPrincipal) SetAssurance(as AgentAssurance) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.assurance = as
}

// Assurance returns a copy of the agent assurance.
func (a *AgentPrincipal) Assurance() AgentAssurance {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.assurance
}

// SetFrame sets the agent's execution frame. Not thread-safe after first call.
func (a *AgentPrincipal) SetFrame(f *ports.Frame) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.frame = f
}

// Frame returns the agent's execution frame or nil.
func (a *AgentPrincipal) Frame() *ports.Frame {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.frame
}

// AddTenantMembership adds a tenant membership. Not thread-safe after construction.
func (a *AgentPrincipal) AddTenantMembership(tenantID string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.tenantMemberships = append(a.tenantMemberships, tenantID)
}

// TenantMemberships returns a copy of tenant memberships.
func (a *AgentPrincipal) TenantMemberships() []string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	out := make([]string, len(a.tenantMemberships))
	copy(out, a.tenantMemberships)
	return out
}

// AddGroup adds a group membership. Not thread-safe after construction.
func (a *AgentPrincipal) AddGroup(group string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.groups = append(a.groups, group)
}

// Groups returns a copy of group memberships.
func (a *AgentPrincipal) Groups() []string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	out := make([]string, len(a.groups))
	copy(out, a.groups)
	return out
}

// ToActor converts the AgentPrincipal to a ports.Actor.
func (a *AgentPrincipal) ToActor() ports.Actor {
	return ports.Actor{
		Type: a.Type(),
		ID:   a.ID(),
	}
}

// Validate validates the agent principal invariants.
func (a *AgentPrincipal) Validate() error {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.type_ != "agent" {
		return fmt.Errorf("agent: Type must be 'agent', got %q", a.type_)
	}
	if a.id == "" {
		return fmt.Errorf("agent: ID is mandatory")
	}
	return nil
}
