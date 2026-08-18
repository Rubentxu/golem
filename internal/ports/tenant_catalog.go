package ports

import "context"

// TenantTier defines the tier level for a tenant.
type TenantTier string

const (
	TenantTierStandard   TenantTier = "standard"
	TenantTierRegulated  TenantTier = "regulated"
)

// TenantRecord is the full record for a tenant (REQ-CAT-001).
type TenantRecord struct {
	ID         string      `json:"id"`
	CellID     CellID      `json:"cell_id"`
	Tier       TenantTier  `json:"tier"`
	Region     string      `json:"region"`
	Quotas     map[string]int64 `json:"quotas,omitempty"`
	Policies   []string    `json:"policies,omitempty"`
	CreatedAt  int64       `json:"created_at"`
}

// TenantFilter provides filter criteria for listing tenants.
type TenantFilter struct {
	CellID   CellID    `json:"cell_id,omitempty"`
	Tier     TenantTier `json:"tier,omitempty"`
	PageSize int       `json:"page_size,omitempty"`
}

// TenantCatalog is the control plane for tenant management (REQ-CAT-001..002).
type TenantCatalog interface {
	// Get returns a tenant record by ID.
	Get(ctx context.Context, tenantID string) (TenantRecord, error)
	// Assign assigns a tenant to a cell.
	Assign(ctx context.Context, tenantID string, cellID CellID) error
	// List returns tenants matching the filter.
	List(ctx context.Context, filter TenantFilter) ([]TenantRecord, error)
	// Export exports tenant data as a manifest.
	Export(ctx context.Context, tenantID string) ([]byte, error)
}
