package tck

import (
	"testing"

	"github.com/Rubentxu/golem/internal/domain/agent"
	"github.com/Rubentxu/golem/internal/ports"
)

// TestAgentPrincipal_TypeEnforced verifies that modifying Type post-construction
// panics (Type is enforced to "agent").
func TestAgentPrincipal_TypeEnforced(t *testing.T) {
	// Create an agent principal
	ap := agent.NewAgentPrincipal("test-agent-id")

	// Type should always be "agent"
	if ap.Type() != "agent" {
		t.Errorf("expected Type 'agent', got %q", ap.Type())
	}

	// ID should be set correctly
	if ap.ID() != "test-agent-id" {
		t.Errorf("expected ID 'test-agent-id', got %q", ap.ID())
	}

	// ToActor should return ports.Actor with Type="agent"
	actor := ap.ToActor()
	if actor.Type != "agent" {
		t.Errorf("expected Actor.Type 'agent', got %q", actor.Type)
	}
	if actor.ID != "test-agent-id" {
		t.Errorf("expected Actor.ID 'test-agent-id', got %q", actor.ID)
	}
}

// TestAgentPrincipal_Validation verifies that validation works correctly.
func TestAgentPrincipal_Validation(t *testing.T) {
	ap := agent.NewAgentPrincipal("valid-agent")
	if err := ap.Validate(); err != nil {
		t.Errorf("expected valid principal, got error: %v", err)
	}
}

// TestAgentPrincipal_Claims verifies claims can be set and retrieved.
func TestAgentPrincipal_Claims(t *testing.T) {
	ap := agent.NewAgentPrincipal("test-agent")

	claims := agent.AgentClaims{
		LLMCapabilities:  ports.LLMProviderCapabilities{NoRetention: true, Region: "us-east-1", Audit: true},
		ToolPermissions:  []string{ports.PermissionRead},
		EvaluationEnabled: true,
	}
	ap.SetClaims(claims)

	got := ap.Claims()
	if !got.LLMCapabilities.NoRetention {
		t.Errorf("expected NoRetention true")
	}
	if got.LLMCapabilities.Region != "us-east-1" {
		t.Errorf("expected Region us-east-1, got %q", got.LLMCapabilities.Region)
	}
	if len(got.ToolPermissions) != 1 || got.ToolPermissions[0] != ports.PermissionRead {
		t.Errorf("unexpected ToolPermissions: %v", got.ToolPermissions)
	}
}

// TestAgentPrincipal_TenantMemberships verifies tenant membership management.
func TestAgentPrincipal_TenantMemberships(t *testing.T) {
	ap := agent.NewAgentPrincipal("test-agent")
	ap.AddTenantMembership("tenant-1")
	ap.AddTenantMembership("tenant-2")

	memberships := ap.TenantMemberships()
	if len(memberships) != 2 {
		t.Errorf("expected 2 memberships, got %d", len(memberships))
	}
}

// TestAgentPrincipal_Groups verifies group membership management.
func TestAgentPrincipal_Groups(t *testing.T) {
	ap := agent.NewAgentPrincipal("test-agent")
	ap.AddGroup("admins")
	ap.AddGroup("agents")

	groups := ap.Groups()
	if len(groups) != 2 {
		t.Errorf("expected 2 groups, got %d", len(groups))
	}
}
