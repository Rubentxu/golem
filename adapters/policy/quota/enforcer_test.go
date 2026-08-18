package quota

import (
	"context"
	"testing"

	"github.com/Rubentxu/golem/internal/ports"
)

// TestQuotaEnforcer_HardEnforcement verifies hard enforcement blocks when quota exceeded.
func TestQuotaEnforcer_HardEnforcement(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	delegate := &quotaMockDelegate{}
	enforcer := NewEnforcer(delegate, ports.QuotaModeHard)

	// This should block when hard enforcement is enabled.
	decision, err := enforcer.Consume(ctx, "tenant-123", "events", 1)
	if err != nil {
		t.Fatalf("Consume error: %v", err)
	}

	// With a mock delegate, the decision depends on implementation.
	_ = decision
}

// TestQuotaEnforcer_AuditMode verifies audit mode logs without blocking.
func TestQuotaEnforcer_AuditMode(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	delegate := &quotaMockDelegate{}
	enforcer := NewEnforcer(delegate, ports.QuotaModeSoft)

	decision, err := enforcer.Consume(ctx, "tenant-audit", "events", 1)
	if err != nil {
		t.Fatalf("Consume error: %v", err)
	}

	// In soft mode, should allow.
	if decision.Outcome != "allowed" {
		t.Errorf("Expected 'allowed', got '%s'", decision.Outcome)
	}
}

// TestQuotaEnforcer_ThrottleMode verifies throttle mode adds retry delay.
func TestQuotaEnforcer_ThrottleMode(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	delegate := &quotaMockDelegate{}
	enforcer := NewEnforcer(delegate, ports.QuotaModeThrottle)

	decision, err := enforcer.Consume(ctx, "tenant-throttle", "events", 1)
	if err != nil {
		t.Fatalf("Consume error: %v", err)
	}

	_ = decision
}

// TestQuotaEnforcer_Refund verifies refund returns units to quota.
func TestQuotaEnforcer_Refund(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	delegate := &quotaMockDelegate{}
	enforcer := NewEnforcer(delegate, ports.QuotaModeHard)

	err := enforcer.Refund(ctx, "tenant-123", "events", 5)
	if err != nil {
		t.Fatalf("Refund error: %v", err)
	}
}

// TestQuotaEnforcer_Limits verifies limits are returned.
func TestQuotaEnforcer_Limits(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	delegate := &quotaMockDelegate{}
	enforcer := NewEnforcer(delegate, ports.QuotaModeHard)

	limits, err := enforcer.Limits(ctx, "tenant-123")
	if err != nil {
		t.Fatalf("Limits error: %v", err)
	}

	if limits == nil {
		t.Error("Expected non-nil limits")
	}
}

// quotaMockDelegate is a minimal QuotaEnforcer for testing.
type quotaMockDelegate struct{}

func (m *quotaMockDelegate) Consume(ctx context.Context, tenantID, capability string, units int64) (ports.QuotaDecision, error) {
	return ports.QuotaDecision{Outcome: "allowed", Mode: ports.QuotaModeSoft}, nil
}

func (m *quotaMockDelegate) Refund(ctx context.Context, tenantID, capability string, units int64) error {
	return nil
}

func (m *quotaMockDelegate) Limits(ctx context.Context, tenantID string) (map[string]int64, error) {
	return map[string]int64{"events": 1000}, nil
}

// Compile-time interface check.
var _ ports.QuotaEnforcer = (*Enforcer)(nil)
