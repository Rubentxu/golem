package ports

import "testing"

// TestPrincipal_FieldsPopulated verifies Principal has all required fields.
func TestPrincipal_FieldsPopulated(t *testing.T) {
	t.Parallel()
	p := Principal{
		Subject: "user-123",
		Type:    "human",
		TenantMemberships: []TenantMembership{
			{TenantID: "t-1", Role: "admin"},
		},
		Groups: []string{"golem-operators"},
		Claims: map[string]any{"iss": "https://auth.example.com"},
	}

	if p.Subject == "" {
		t.Error("expected Subject to be set")
	}
	if p.Type == "" {
		t.Error("expected Type to be set")
	}
	if len(p.TenantMemberships) == 0 {
		t.Error("expected TenantMemberships to be set")
	}
	if p.TenantMemberships[0].TenantID != "t-1" {
		t.Errorf("expected TenantID t-1, got %s", p.TenantMemberships[0].TenantID)
	}
}
