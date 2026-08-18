package s3sink

import (
	"context"
	"testing"
	"time"

	"github.com/Rubentxu/golem/internal/ports"
)

// TestS3Sink_Export verifies export to S3.
func TestS3Sink_Export(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	sink := NewSink("test-bucket")

	rollups := []ports.MeteringRollup{
		{
			TenantID:     "tenant-1",
			Hour:         time.Now().Truncate(time.Hour),
			Capability:   "events",
			TotalUnits:   100,
			TotalCostUSD: 0.10,
			Digest:       "abc123",
		},
	}

	err := sink.Export(ctx, rollups)
	if err != nil {
		t.Fatalf("Export error: %v", err)
	}
}

// TestS3Sink_ExportEmpty verifies empty rollups don't cause error.
func TestS3Sink_ExportEmpty(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	sink := NewSink("test-bucket")

	err := sink.Export(ctx, []ports.MeteringRollup{})
	if err != nil {
		t.Fatalf("Export error: %v", err)
	}
}
