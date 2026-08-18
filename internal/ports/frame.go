package ports

// Frame identifies the execution context for an agent (ADR-064).
// Budget is optional here to break import cycles; full Budget type is in budgets.go.
type Frame struct {
	ID          string            `json:"id"`
	TenantID    string            `json:"tenant_id"`
	Goal        string            `json:"goal,omitempty"`
	Constraints map[string]string `json:"constraints,omitempty"`
	Permissions []string          `json:"permissions,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}
