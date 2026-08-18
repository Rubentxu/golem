package ports

import "fmt"

// permissionCatalog is the closed catalog of valid permissions (ADR-062).
var permissionCatalog = map[string]bool{
	PermissionRead:    true,
	PermissionWrite:   true,
	PermissionDelete:  true,
	PermissionExecute: true,
	PermissionAdmin:   true,
}

// Frame identifies the execution context for an agent (ADR-064).
// It bounds goals, permissions and budgets for agent execution.
type Frame struct {
	ID          string            `json:"id"`
	TenantID    string            `json:"tenant_id"`
	Goal        string            `json:"goal,omitempty"`
	Constraints map[string]string `json:"constraints,omitempty"`
	Permissions []string          `json:"permissions,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// Validate checks that all permissions are in the closed catalog.
// Returns ErrUnknownPermission if any permission is invalid.
func (f Frame) Validate() error {
	for _, perm := range f.Permissions {
		if !permissionCatalog[perm] {
			return fmt.Errorf("%w: %q", ErrUnknownPermission, perm)
		}
	}
	return nil
}
