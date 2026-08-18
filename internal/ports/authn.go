package ports

import "context"

// Principal represents an authenticated identity (REQ-OIDC-001).
type Principal struct {
	Subject           string             `json:"subject"`            // unique identifier
	Type              string             `json:"type"`               // "human", "service", "agent"
	TenantMemberships []TenantMembership `json:"tenant_memberships"` // tenants the principal belongs to
	Groups            []string           `json:"groups"`             // groups the principal belongs to
	Claims            map[string]any     `json:"claims"`             // raw JWT claims
}

// TenantMembership describes a principal's membership in a tenant.
type TenantMembership struct {
	TenantID string `json:"tenant_id"`
	Role     string `json:"role"` // e.g., "admin", "member"
}

// AuthN verifies bearer tokens and provides principal information (REQ-OIDC-001..004).
type AuthN interface {
	// VerifyBearer verifies a bearer JWT token and returns the principal.
	VerifyBearer(ctx context.Context, token string) (Principal, error)
	// Discover returns the OIDC issuer URL for JWKS discovery.
	Discover(ctx context.Context) (string, error)
}
