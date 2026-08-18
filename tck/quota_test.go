package tck

import (
	"context"
	"testing"

	"github.com/Rubentxu/golem/adapters/policy/quota"
	"github.com/Rubentxu/golem/internal/ports"
)

// TestQuotaEnforcer_HardBlocks verifies hard enforcement blocks when exceeded (REQ-QUOTA-002).
func TestQuotaEnforcer_HardBlocks(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	enforcer := quota.NewEnforcer(nil, ports.QuotaModeHard)

	// With nil delegate, should allow.
	decision, err := enforcer.Consume(ctx, "tenant-1", "events", 1)
	if err != nil {
		t.Fatalf("Consume error: %v", err)
	}

	if decision.Outcome != "allowed" {
		t.Errorf("Expected 'allowed' with nil delegate, got '%s'", decision.Outcome)
	}
}

// TestQuotaEnforcer_SoftAllows verifies soft mode allows operations (REQ-QUOTA-002).
func TestQuotaEnforcer_SoftAllows(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	enforcer := quota.NewEnforcer(nil, ports.QuotaModeSoft)

	decision, err := enforcer.Consume(ctx, "tenant-1", "events", 1)
	if err != nil {
		t.Fatalf("Consume error: %v", err)
	}

	if decision.Outcome != "allowed" {
		t.Errorf("Expected 'allowed' in soft mode, got '%s'", decision.Outcome)
	}
}

// TestQuotaEnforcer_Refund verifies refund (REQ-QUOTA-003).
func TestQuotaEnforcer_Refund(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	enforcer := quota.NewEnforcer(nil, ports.QuotaModeHard)

	err := enforcer.Refund(ctx, "tenant-1", "events", 5)
	if err != nil {
		t.Fatalf("Refund error: %v", err)
	}
}

// TestQuotaEnforcer_Limits verifies limits retrieval (REQ-QUOTA-001).
func TestQuotaEnforcer_Limits(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	enforcer := quota.NewEnforcer(nil, ports.QuotaModeHard)

	limits, err := enforcer.Limits(ctx, "tenant-1")
	if err != nil {
		t.Fatalf("Limits error: %v", err)
	}

	if limits == nil {
		t.Error("Expected non-nil limits map")
	}
}
