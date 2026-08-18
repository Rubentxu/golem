package dlq

import (
	"context"
	"testing"
	"time"

	"github.com/Rubentxu/golem/internal/ports"
)

// TestDLQ_Add verifies adding events to DLQ.
func TestDLQ_Add(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	dlq := NewQueue()

	event := ports.MeteringEvent{
		TenantID:   "tenant-1",
		Capability: "events",
		Units:      10,
		CostUSD:    0.01,
		Timestamp:  time.Now(),
	}

	err := dlq.Add(ctx, event)
	if err != nil {
		t.Fatalf("Add error: %v", err)
	}
}

// TestDLQ_Replay verifies replay returns and clears events.
func TestDLQ_Replay(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	dlq := NewQueue()

	event := ports.MeteringEvent{
		TenantID:   "tenant-1",
		Capability: "events",
		Units:      10,
		CostUSD:    0.01,
		Timestamp:  time.Now(),
	}

	_ = dlq.Add(ctx, event)

	events, err := dlq.Replay(ctx)
	if err != nil {
		t.Fatalf("Replay error: %v", err)
	}

	if len(events) != 1 {
		t.Errorf("Expected 1 event, got %d", len(events))
	}

	// Verify queue is cleared.
	size, _ := dlq.Size(ctx)
	if size != 0 {
		t.Errorf("Expected 0 events after replay, got %d", size)
	}
}

// TestDLQ_Size verifies size tracking.
func TestDLQ_Size(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	dlq := NewQueue()

	event := ports.MeteringEvent{
		TenantID:   "tenant-1",
		Capability: "events",
		Units:      10,
		CostUSD:    0.01,
		Timestamp:  time.Now(),
	}

	_ = dlq.Add(ctx, event)

	size, err := dlq.Size(ctx)
	if err != nil {
		t.Fatalf("Size error: %v", err)
	}

	if size != 1 {
		t.Errorf("Expected size 1, got %d", size)
	}
}
