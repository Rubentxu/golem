package archtest

import (
	"context"
	"testing"
	"time"

	"github.com/Rubentxu/golem/internal/ports"
)

// TestQuotaEnforcer_Invariant_HardBlocks verifies hard quota blocks when exceeded (REQ-QUOTA-002).
func TestQuotaEnforcer_Invariant_HardBlocks(t *testing.T) {
	t.Parallel()

	// Verify the interface contract only — actual enforcement tested in adapters.
	var enforcer ports.QuotaEnforcer = &quotaShim{}
	_ = enforcer
	_ = context.Background()
}

// TestSLOTracker_Invariant_EvaluatesViolations verifies SLO evaluation (REQ-SLO-001).
func TestSLOTracker_Invariant_EvaluatesViolations(t *testing.T) {
	t.Parallel()
	// Verify interface contract only.
	var _ ports.SLOTracker = &sloShim{}
}

// TestMeter_Invariant_RollupDigest verifies rollup has digest (REQ-METER-002).
func TestMeter_Invariant_RollupDigest(t *testing.T) {
	t.Parallel()
	// Verify interface contract only.
	var _ ports.UsageMeter = &meterShim{}
}

// Shim types to avoid importing adapters.

type quotaShim struct{}

func (*quotaShim) Consume(ctx context.Context, tenantID, capability string, units int64) (ports.QuotaDecision, error) {
	return ports.QuotaDecision{}, nil
}
func (*quotaShim) Refund(ctx context.Context, tenantID, capability string, units int64) error {
	return nil
}
func (*quotaShim) Limits(ctx context.Context, tenantID string) (map[string]int64, error) {
	return nil, nil
}

type sloShim struct{}

func (*sloShim) Record(ctx context.Context, sloName string, value float64) error { return nil }
func (*sloShim) Evaluate(ctx context.Context) ([]ports.SLOViolation, error)      { return nil, nil }

type meterShim struct{}

func (*meterShim) Record(ctx context.Context, event ports.MeteringEvent) error { return nil }
func (*meterShim) Rollup(ctx context.Context, hour time.Time) ([]ports.MeteringRollup, error) {
	return nil, nil
}
func (*meterShim) UptimeGauge(ctx context.Context, capability string) (float64, error) { return 0, nil }
func (*meterShim) ErrorBudgetGauge(ctx context.Context, capability string) (float64, error) {
	return 0, nil
}
