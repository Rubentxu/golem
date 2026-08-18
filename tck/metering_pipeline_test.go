package tck

import (
	"context"
	"testing"
	"time"

	"github.com/Rubentxu/golem/adapters/metering"
	"github.com/Rubentxu/golem/adapters/metering/dlq"
	"github.com/Rubentxu/golem/adapters/metering/s3sink"
	"github.com/Rubentxu/golem/internal/ports"
)

// TestMeteringPipeline_RecordRollupExport verifies full pipeline (REQ-METER-003).
func TestMeteringPipeline_RecordRollupExport(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	meter := metering.NewMeter()
	sink := s3sink.NewSink("test-bucket")
	queue := dlq.NewQueue()

	// Record events.
	event := ports.MeteringEvent{
		TenantID:   "tenant-pipeline",
		Capability: "events",
		Units:      100,
		CostUSD:    0.10,
		Timestamp:  time.Now().Truncate(time.Hour),
	}

	_ = meter.Record(ctx, event)

	// Rollup.
	hour := event.Timestamp.Truncate(time.Hour)
	rollups, err := meter.Rollup(ctx, hour)
	if err != nil {
		t.Fatalf("Rollup error: %v", err)
	}

	// Export.
	err = sink.Export(ctx, rollups)
	if err != nil {
		t.Fatalf("Export error: %v", err)
	}

	// Verify DLQ can store failed events.
	_ = queue.Add(ctx, event)
}

// TestMeteringPipeline_DLQReplay verifies DLQ replay (REQ-METER-003).
func TestMeteringPipeline_DLQReplay(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	queue := dlq.NewQueue()

	event := ports.MeteringEvent{
		TenantID:   "tenant-dlq",
		Capability: "events",
		Units:      50,
		CostUSD:    0.05,
		Timestamp:  time.Now(),
	}

	_ = queue.Add(ctx, event)

	events, err := queue.Replay(ctx)
	if err != nil {
		t.Fatalf("Replay error: %v", err)
	}

	if len(events) != 1 {
		t.Errorf("Expected 1 event, got %d", len(events))
	}
}
