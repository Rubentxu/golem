package ports

import (
	"context"
	"testing"
)

// TestQuotaDecision_Outcomes verifies the three quota decision outcomes.
func TestQuotaDecision_Outcomes(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// Create a mock QuotaEnforcer for testing.
	enforcer := &mockQuotaEnforcer{limits: map[string]int64{"token_cost": 1000}}

	// Test: consume within limit
	dec, err := enforcer.Consume(ctx, "tenant-1", "token_cost", 100)
	if err != nil {
		t.Fatalf("Consume error: %v", err)
	}
	if dec.Outcome != "allowed" {
		t.Errorf("expected outcome=allowed, got %s", dec.Outcome)
	}

	// Test: consume exceeding limit (hard mode)
	enforcer2 := &mockQuotaEnforcer{limits: map[string]int64{"token_cost": 50}, mode: QuotaModeHard}
	dec2, err := enforcer2.Consume(ctx, "tenant-1", "token_cost", 100)
	if err != nil {
		t.Fatalf("Consume error: %v", err)
	}
	if dec2.Outcome != "denied" {
		t.Errorf("expected outcome=denied, got %s", dec2.Outcome)
	}

	// Test: throttle mode returns retry-after
	enforcer3 := &mockQuotaEnforcer{limits: map[string]int64{"token_cost": 50}, mode: QuotaModeThrottle}
	dec3, err := enforcer3.Consume(ctx, "tenant-1", "token_cost", 100)
	if err != nil {
		t.Fatalf("Consume error: %v", err)
	}
	if dec3.Outcome != "throttled" {
		t.Errorf("expected outcome=throttled, got %s", dec3.Outcome)
	}
	if dec3.RetryAfterMs <= 0 {
		t.Errorf("expected RetryAfterMs > 0, got %d", dec3.RetryAfterMs)
	}
}

// mockQuotaEnforcer is a minimal implementation for testing.
type mockQuotaEnforcer struct {
	limits map[string]int64
	mode   QuotaMode
	used   map[string]int64
}

func (m *mockQuotaEnforcer) Consume(ctx context.Context, tenantID, capability string, units int64) (QuotaDecision, error) {
	if m.used == nil {
		m.used = make(map[string]int64)
	}
	limit := m.limits[capability]
	used := m.used[tenantID+capability]

	if used+units > limit {
		switch m.mode {
		case QuotaModeHard:
			return QuotaDecision{Outcome: "denied", Mode: m.mode}, nil
		case QuotaModeThrottle:
			return QuotaDecision{Outcome: "throttled", Mode: m.mode, RetryAfterMs: 1000}, nil
		default:
			// Soft mode allows
			return QuotaDecision{Outcome: "allowed", Mode: QuotaModeSoft}, nil
		}
	}
	m.used[tenantID+capability] = used + units
	return QuotaDecision{Outcome: "allowed", Mode: m.mode}, nil
}

func (m *mockQuotaEnforcer) Refund(ctx context.Context, tenantID, capability string, units int64) error {
	if m.used == nil {
		m.used = make(map[string]int64)
	}
	used := m.used[tenantID+capability]
	m.used[tenantID+capability] = used - units
	return nil
}

func (m *mockQuotaEnforcer) Limits(ctx context.Context, tenantID string) (map[string]int64, error) {
	return m.limits, nil
}

// Compile-time interface check.
var _ QuotaEnforcer = (*mockQuotaEnforcer)(nil)
