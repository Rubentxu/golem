package ports

import (
	"testing"
	"time"
)

// TestMeterEvent_HasCorrelationID verifies MeteringEvent has correlation ID.
func TestMeterEvent_HasCorrelationID(t *testing.T) {
	t.Parallel()
	event := MeteringEvent{
		TenantID:      "t-123",
		Capability:    "llm",
		Units:         1000,
		CostUSD:       0.05,
		Timestamp:     time.Now(),
		CorrelationID: "corr-456",
	}

	if event.CorrelationID == "" {
		t.Error("expected CorrelationID to be set")
	}
	if event.TenantID == "" {
		t.Error("expected TenantID to be set")
	}
	if event.Capability == "" {
		t.Error("expected Capability to be set")
	}
}
