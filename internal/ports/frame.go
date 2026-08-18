package ports

import "fmt"

// Frame identifies the execution context for an agent (ADR-064).
// It bounds goals, permissions and budgets for agent execution.
type Frame struct {
	ID          string            `json:"id"`
	TenantID    string            `json:"tenant_id"`
	Goal        string            `json:"goal,omitempty"`
	Constraints map[string]string `json:"constraints,omitempty"`
	Permissions []Permission      `json:"permissions,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	// Budget constrains resource usage for this frame (ADR-069, M8 I-1).
	Budget BudgetLimits `json:"budget,omitempty"`
}

// Validate checks that all permissions are in the closed catalog (ADR-058)
// and that the embedded budget is valid.
func (f Frame) Validate() error {
	if f.ID == "" {
		return fmt.Errorf("frame: id is mandatory")
	}
	if f.TenantID == "" {
		return fmt.Errorf("frame: tenant is mandatory")
	}
	if f.Goal == "" {
		return fmt.Errorf("frame: goal is mandatory")
	}
	known := map[Permission]bool{
		PermissionGraphRead:     true,
		PermissionGraphReadLens: true,
		PermissionProposalWrite: true,
		PermissionProposalApply: true,
		PermissionEvidenceWrite: true,
	}
	for _, perm := range f.Permissions {
		if !known[perm] {
			return fmt.Errorf("%w: %q not in closed catalog v1", ErrUnknownPermission, perm)
		}
	}
	if err := f.Budget.Validate(); err != nil {
		return err
	}
	return nil
}
