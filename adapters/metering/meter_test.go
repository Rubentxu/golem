package metering

import (
	"context"
	"testing"
	"time"

	"github.com/Rubentxu/golem/internal/ports"
)

// TestMeter_Record verifies metering event recording.
func TestMeter_Record(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	meter := NewMeter()

	event := ports.MeteringEvent{
		TenantID:  "tenant-1",
		Capability: "events",
		Units:     10,
		CostUSD:   0.01,
		Timestamp: time.Now(),
	}

	err := meter.Record(ctx, event)
	if err != nil {
		t.Fatalf("Record error: %v", err)
	}
}

// TestMeter_Rollup verifies hourly rollup.
func TestMeter_Rollup(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	meter := NewMeter()

	now := time.Now().Truncate(time.Hour)

	event := ports.MeteringEvent{
		TenantID:  "tenant-1",
		Capability: "events",
		Units:     100,
		CostUSD:   0.10,
		Timestamp: now,
	}

	_ = meter.Record(ctx, event)

	rollups, err := meter.Rollup(ctx, now)
	if err != nil {
		t.Fatalf("Rollup error: %v", err)
	}

	if len(rollups) == 0 {
		t.Error("Expected at least one rollup")
	}
}

// TestMeter_UptimeGauge verifies uptime gauge.
func TestMeter_UptimeGauge(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	meter := NewMeter()

	uptime, err := meter.UptimeGauge(ctx, "events")
	if err != nil {
		t.Fatalf("UptimeGauge error: %v", err)
	}

	if uptime < 0 || uptime > 1 {
		t.Errorf("Expected uptime 0..1, got %f", uptime)
	}
}
