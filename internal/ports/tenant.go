package ports

import "context"

// TenantID identifies the mandatory end-to-end tenant scope (ADR-008).
type TenantID string

type tenantCtxKey struct{}

// WithTenant returns a context carrying the tenant scope. Every command,
// query and journal append must run under a tenant context.
func WithTenant(ctx context.Context, id TenantID) context.Context {
	return context.WithValue(ctx, tenantCtxKey{}, id)
}

// TenantFrom extracts the tenant scope; ok is false when absent, which
// callers must treat as a programming error at the edge (reject request).
func TenantFrom(ctx context.Context) (id TenantID, ok bool) {
	id, ok = ctx.Value(tenantCtxKey{}).(TenantID)
	return id, ok
}
