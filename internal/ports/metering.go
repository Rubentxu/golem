package ports

import (
	"context"
	"time"
)

// MeteringEvent is emitted by the metering hook for each capability invocation (REQ-METER-001).
type MeteringEvent struct {
	TenantID      string    `json:"tenant_id"`
	Capability    string    `json:"capability"`
	Units         int64     `json:"units"`
	CostUSD       float64   `json:"cost_usd"`
	Timestamp     time.Time `json:"timestamp"`
	CorrelationID string    `json:"correlation_id"`
}

// MeteringRollup is the hourly aggregated metering data (REQ-METER-002).
type MeteringRollup struct {
	TenantID     string    `json:"tenant_id"`
	Hour         time.Time `json:"hour"` // truncated to hour
	Capability   string    `json:"capability"`
	TotalUnits   int64     `json:"total_units"`
	TotalCostUSD float64   `json:"total_cost_usd"`
	Digest       string    `json:"digest"` // sha256 of rollup content
}

// UsageMeter tracks capability usage for billing (REQ-METER-001..004).
//
// Record emits a MeteringEvent for each capability invocation.
// Rollup generates hourly rollups with sha256 digest.
//
// UptimeGauge and ErrorBudgetGauge are used by SLO tracking.
type UsageMeter interface {
	// Record emits a metering event.
	Record(ctx context.Context, event MeteringEvent) error
	// Rollup generates hourly rollup.
	Rollup(ctx context.Context, hour time.Time) ([]MeteringRollup, error)
	// UptimeGauge returns the uptime metric for a capability.
	UptimeGauge(ctx context.Context, capability string) (float64, error)
	// ErrorBudgetGauge returns the error budget consumption for a capability.
	ErrorBudgetGauge(ctx context.Context, capability string) (float64, error)
}
