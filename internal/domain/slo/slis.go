// Package slo implements the SLO bounded context (REQ-SLO-001).
package slo

import (
	"fmt"
	"time"
)

// SLIName identifies a specific Service Level Indicator.
type SLIName string

// SLI definitions for the 13 tracked indicators (ADR-080).
const (
	// Command API SLIs
	SLICommandLatencyP99    SLIName = "command.latency.p99"
	SLICommandErrorRate     SLIName = "command.error_rate"

	// System SLIs
	SLISystemAvailability   SLIName = "system.availability"

	// Journal SLIs
	SLIJournalReplayTimeP99 SLIName = "journal.replay_time.p99"

	// Agent SLIs
	SLIAgentEvalPassRate    SLIName = "agent.eval_pass_rate"

	// OIDC SLIs
	SLIOIDCVerifyLatencyP99 SLIName = "oidc.verify_latency.p99"

	// Ops console SLIs
	SLIOpsConsoleActionLatencyP99 SLIName = "ops.console.action_latency.p99"

	// Audit SLIs
	SLIAuditExportSuccessRate SLIName = "audit.export.success_rate"

	// Metering SLIs
	SLIMeteringRollupSuccessRate SLIName = "metering.rollup.success_rate"

	// Additional internal SLIs
	SLIQuotaCheckLatencyP99  SLIName = "quota.check_latency.p99"
	SLICellMigrateDurationP99 SLIName = "cell.migrate.duration.p99"
	SLISnapshotDurationP99   SLIName = "snapshot.duration.p99"
	SLIMeterQueryLatencyP99  SLIName = "meter.query_latency.p99"
)

// SLIDefinition defines an SLI with its target and window.
type SLIDefinition struct {
	Name        SLIName
	Description string
	Unit        string // e.g., "ms", "percent", "ratio"
	Target      float64 // e.g., 0.999 for 99.9%
	Window      time.Duration
}

// AllSLIs returns all 13 SLI definitions.
func AllSLIs() []SLIDefinition {
	return []SLIDefinition{
		{
			Name:        SLICommandLatencyP99,
			Description: "Command API p99 latency",
			Unit:        "ms",
			Target:      0.999, // p99 < 250ms
			Window:      1 * time.Hour,
		},
		{
			Name:        SLICommandErrorRate,
			Description: "Command API error rate",
			Unit:        "percent",
			Target:      0.999, // error rate < 0.1%
			Window:      1 * time.Hour,
		},
		{
			Name:        SLISystemAvailability,
			Description: "System availability",
			Unit:        "percent",
			Target:      0.999, // 99.9%
			Window:      30 * 24 * time.Hour,
		},
		{
			Name:        SLIJournalReplayTimeP99,
			Description: "Journal replay p99 time",
			Unit:        "ms",
			Target:      0.99995, // p99 < 50ms
			Window:      1 * time.Hour,
		},
		{
			Name:        SLIAgentEvalPassRate,
			Description: "Agent evaluation pass rate",
			Unit:        "percent",
			Target:      0.80, // ≥ 80%
			Window:      1 * time.Hour,
		},
		{
			Name:        SLIOIDCVerifyLatencyP99,
			Description: "OIDC token verify p99 latency",
			Unit:        "ms",
			Target:      0.999, // p99 < 100ms
			Window:      1 * time.Hour,
		},
		{
			Name:        SLIOpsConsoleActionLatencyP99,
			Description: "Ops console action p99 latency",
			Unit:        "ms",
			Target:      0.9995, // p99 < 500ms
			Window:      1 * time.Hour,
		},
		{
			Name:        SLIAuditExportSuccessRate,
			Description: "Audit export success rate",
			Unit:        "percent",
			Target:      0.99, // ≥ 99%
			Window:      1 * time.Hour,
		},
		{
			Name:        SLIMeteringRollupSuccessRate,
			Description: "Metering rollup success rate",
			Unit:        "percent",
			Target:      0.995, // ≥ 99.5%
			Window:      1 * time.Hour,
		},
		{
			Name:        SLIQuotaCheckLatencyP99,
			Description: "Quota check p99 latency",
			Unit:        "ms",
			Target:      0.999, // p99 < 100ms
			Window:      1 * time.Hour,
		},
		{
			Name:        SLICellMigrateDurationP99,
			Description: "Cell migration p99 duration",
			Unit:        "ms",
			Target:      0.999, // p99 < 500ms
			Window:      24 * time.Hour,
		},
		{
			Name:        SLISnapshotDurationP99,
			Description: "DR snapshot p99 duration",
			Unit:        "ms",
			Target:      0.999, // p99 < 5min
			Window:      24 * time.Hour,
		},
		{
			Name:        SLIMeterQueryLatencyP99,
			Description: "Metering query p99 latency",
			Unit:        "ms",
			Target:      0.999, // p99 < 200ms
			Window:      1 * time.Hour,
		},
	}
}

// GetSLI returns the SLI definition by name.
func GetSLI(name SLIName) (SLIDefinition, error) {
	for _, sli := range AllSLIs() {
		if sli.Name == name {
			return sli, nil
		}
	}
	return SLIDefinition{}, fmt.Errorf("unknown SLI: %s", name)
}

// ValidateValue validates a metric value for an SLI.
func ValidateValue(name SLIName, value float64) error {
	def, err := GetSLI(name)
	if err != nil {
		return err
	}
	// Values must be non-negative for latency/error metrics
	if value < 0 {
		return fmt.Errorf("negative value %f for SLI %s", value, name)
	}
	// For percentage-based SLIs, value should be 0..1 or 0..100
	// Latency values are in milliseconds
	switch def.Unit {
	case "ms":
		// Latency values in ms, reasonable upper bound 60000 (1 minute)
		if value > 60000 {
			return fmt.Errorf("latency value %f exceeds maximum for %s", value, name)
		}
	case "percent":
		// Accept both 0..1 and 0..100
		if value > 100 {
			return fmt.Errorf("percent value %f exceeds 100 for %s", value, name)
		}
	case "ratio":
		if value > 1 {
			return fmt.Errorf("ratio value %f exceeds 1 for %s", value, name)
		}
	}
	return nil
}
