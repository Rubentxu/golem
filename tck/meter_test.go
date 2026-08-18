package tck

import (
	"context"
	"testing"
	"time"

	"github.com/Rubentxu/golem/adapters/metering"
	"github.com/Rubentxu/golem/internal/ports"
)

// TestMeter_Record verifies metering event recording (REQ-METER-001).
func TestMeter_Record(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	meter := metering.NewMeter()

	event := ports.MeteringEvent{
		TenantID:   "tenant-1",
		Capability: "events",
		Units:      10,
		CostUSD:    0.01,
		Timestamp:  time.Now(),
	}

	err := meter.Record(ctx, event)
	if err != nil {
		t.Fatalf("Record error: %v", err)
	}
}

// TestMeter_Rollup verifies hourly rollup with digest (REQ-METER-002).
func TestMeter_Rollup(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	meter := metering.NewMeter()

	now := time.Now().Truncate(time.Hour)

	event := ports.MeteringEvent{
		TenantID:   "tenant-1",
		Capability: "events",
		Units:      100,
		CostUSD:    0.10,
		Timestamp:  now,
	}

	_ = meter.Record(ctx, event)

	rollups, err := meter.Rollup(ctx, now)
	if err != nil {
		t.Fatalf("Rollup error: %v", err)
	}

	if len(rollups) == 0 {
		t.Fatal("Expected at least one rollup")
	}

	// Verify digest is present.
	if rollups[0].Digest == "" {
		t.Error("Expected non-empty digest in rollup")
	}
}

// TestMeter_Gauges verifies uptime and error budget gauges (REQ-METER-001).
func TestMeter_Gauges(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	meter := metering.NewMeter()

	uptime, err := meter.UptimeGauge(ctx, "events")
	if err != nil {
		t.Fatalf("UptimeGauge error: %v", err)
	}

	if uptime < 0 || uptime > 1 {
		t.Errorf("Expected uptime 0..1, got %f", uptime)
	}

	eb, err := meter.ErrorBudgetGauge(ctx, "events")
	if err != nil {
		t.Fatalf("ErrorBudgetGauge error: %v", err)
	}

	if eb < 0 || eb > 1 {
		t.Errorf("Expected error budget 0..1, got %f", eb)
	}
}
