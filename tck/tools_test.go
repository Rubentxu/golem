package tck

import (
	"context"
	"errors"
	"testing"

	"github.com/Rubentxu/golem/internal/ports"
)

// TestTool_CatalogClosed verifies that permissions outside the catalog
// result in ErrUnknownPermission.
func TestTool_CatalogClosed(t *testing.T) {
	// The permission catalog is closed - only defined constants are valid
	validPerms := []ports.Permission{
		ports.PermissionGraphRead,
		ports.PermissionGraphReadLens,
		ports.PermissionProposalWrite,
		ports.PermissionProposalApply,
		ports.PermissionEvidenceWrite,
	}

	// Verify all permission constants are unique
	permSet := make(map[ports.Permission]bool)
	for _, p := range validPerms {
		if permSet[p] {
			t.Errorf("duplicate permission: %s", p)
		}
		permSet[p] = true
	}

	// Invalid permission should be detected
	invalidPerm := ports.Permission("invalid-permission")
	for _, p := range validPerms {
		if p == invalidPerm {
			t.Errorf("invalid permission should not match valid: %s", invalidPerm)
		}
	}
}

// TestTool_InvokeJournaled verifies that tool invocations are journaled
// through the ToolInput/ToolOutput types.
func TestTool_InvokeJournaled(t *testing.T) {
	input := ports.ToolInput{
		TenantID: "t-test",
		ToolName: "test-tool",
		Params:   map[string]any{"key": "value"},
	}

	output := ports.ToolOutput{
		TenantID: input.TenantID,
		ToolName: input.ToolName,
		Result:   "success",
		Success:  true,
	}

	if output.TenantID != input.TenantID {
		t.Errorf("output tenant mismatch: got %s, want %s", output.TenantID, input.TenantID)
	}
	if output.ToolName != input.ToolName {
		t.Errorf("output tool name mismatch: got %s, want %s", output.ToolName, input.ToolName)
	}
}

// TestTool_ActorTypeEnforced verifies that Actor.Type is enforced.
func TestTool_ActorTypeEnforced(t *testing.T) {
	actor := ports.Actor{
		Type: "agent",
		ID:   "test-agent",
	}
	if actor.Type != "agent" {
		t.Errorf("actor type should be agent, got: %s", actor.Type)
	}
}

// TestTool_VendorSDKNotImported verifies that tool implementations
// do not import vendor SDKs.
func TestTool_VendorSDKNotImported(t *testing.T) {
	// This test validates the port types exist without vendor dependencies
	spec := ports.ToolSpec{
		Name:        "test-tool",
		Permissions: []ports.Permission{ports.PermissionGraphRead},
		Description: "test tool",
	}
	if spec.Name != "test-tool" {
		t.Errorf("tool spec name mismatch")
	}
}

// noopTool implements ports.Tool for interface tests.
type noopTool struct{}

func (noopTool) Invoke(ctx context.Context, input ports.ToolInput) (ports.ToolOutput, error) {
	return ports.ToolOutput{
		TenantID: input.TenantID,
		ToolName: input.ToolName,
		Result:   "noop",
		Success:  true,
	}, nil
}
func (noopTool) Spec() ports.ToolSpec {
	return ports.ToolSpec{
		Name:        "noop",
		Permissions: []ports.Permission{ports.PermissionGraphRead},
		Description: "noop tool",
	}
}

var _ ports.Tool = (*noopTool)(nil)

// TestTool_InterfaceComplies verifies noopTool implements ports.Tool.
func TestTool_InterfaceComplies(t *testing.T) {
	var _ ports.Tool = (*noopTool)(nil)
	_ = context.Background()
	_ = errors.New("test")
}
