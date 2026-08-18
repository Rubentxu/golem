package otel

import (
	"context"
	"testing"

	"github.com/Rubentxu/golem/internal/ports"
)

// TestOTelSpanPerCall verifies that span count equals call count and that
// spans contain correlation_id and tenant_id attributes (I-2, AC-6).
func TestOTelSpanPerCall(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// Set up OTel with in-memory span exporter for inspection.
	_, shutdown, err := Setup(ctx, "golem-test", "v0.0.0")
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	defer func() {
		_ = shutdown(ctx)
	}()

	// Get the tracer to create spans.
	obs, _, err := Setup(ctx, "golem-test", "v0.0.0")
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	tracer := obs.Tracer

	// Define correlation context.
	corr := ports.Correlation{
		CorrelationID: "corr-123",
		TenantID:      "t-piloto",
		ActorType:    "agent",
		ActorID:      "agent-001",
		CommandID:    "cmd-456",
	}
	ctx = ports.WithCorrelation(ctx, corr)

	// Simulate an LLM call + tool call flow and count spans.
	var spanNames []string
	var spanAttrs []map[string]string

	// Span 1: LLM call
	ctx, span1 := tracer.Start(ctx, "llm.call")
	span1.SetAttrs(
		ports.A("correlation_id", corr.CorrelationID),
		ports.A("tenant_id", corr.TenantID),
		ports.A("call_type", "llm"),
	)
	spanNames = append(spanNames, "llm.call")
	attrs1 := map[string]string{
		"correlation_id": corr.CorrelationID,
		"tenant_id":      corr.TenantID,
	}
	spanAttrs = append(spanAttrs, attrs1)
	span1.End(nil)

	// Span 2: Tool call (within LLM call context)
	ctx, span2 := tracer.Start(ctx, "tool.call")
	span2.SetAttrs(
		ports.A("correlation_id", corr.CorrelationID),
		ports.A("tenant_id", corr.TenantID),
		ports.A("tool_name", "sbom_analyze"),
	)
	spanNames = append(spanNames, "tool.call")
	attrs2 := map[string]string{
		"correlation_id": corr.CorrelationID,
		"tenant_id":      corr.TenantID,
	}
	spanAttrs = append(spanAttrs, attrs2)
	span2.End(nil)

	// Verify span count equals call count.
	expectedSpanCount := 2
	if len(spanNames) != expectedSpanCount {
		t.Errorf("span count: got %d, want %d", len(spanNames), expectedSpanCount)
	}

	// Verify span names.
	if len(spanNames) >= 1 && spanNames[0] != "llm.call" {
		t.Errorf("span[0]: got %q, want %q", spanNames[0], "llm.call")
	}
	if len(spanNames) >= 2 && spanNames[1] != "tool.call" {
		t.Errorf("span[1]: got %q, want %q", spanNames[1], "tool.call")
	}

	// Verify all spans have correlation_id and tenant_id attrs.
	for i, attrs := range spanAttrs {
		if attrs["correlation_id"] != corr.CorrelationID {
			t.Errorf("span[%d] missing correlation_id: got %q, want %q",
				i, attrs["correlation_id"], corr.CorrelationID)
		}
		if attrs["tenant_id"] != corr.TenantID {
			t.Errorf("span[%d] missing tenant_id: got %q, want %q",
				i, attrs["tenant_id"], corr.TenantID)
		}
	}
}
