package obs

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"github.com/Rubentxu/golem/internal/ports"
)

func TestSlogLoggerEnrichesCorrelation(t *testing.T) {
	var buf bytes.Buffer
	logger := NewSlogFrom(slog.New(slog.NewJSONHandler(&buf, nil)))

	ctx := ports.WithCorrelation(context.Background(), ports.Correlation{
		CorrelationID: "corr-1",
		TenantID:      "t_42",
		ActorType:     "user",
		ActorID:       "u_9",
		CommandID:     "cmd-7",
	})

	logger.Info(ctx, "command accepted", ports.A("command", "work.create-work-item"))

	var record map[string]any
	if err := json.Unmarshal(buf.Bytes(), &record); err != nil {
		t.Fatalf("not JSON: %v (%s)", err, buf.String())
	}
	for _, key := range []string{"correlation_id", "tenant_id", "actor", "command_id"} {
		if _, ok := record[key]; !ok {
			t.Fatalf("record missing %q: %s", key, buf.String())
		}
	}
	if record["msg"] != "command accepted" {
		t.Fatalf("msg = %v", record["msg"])
	}
}

func TestSlogLoggerFallsBackToTenantContext(t *testing.T) {
	var buf bytes.Buffer
	logger := NewSlogFrom(slog.New(slog.NewJSONHandler(&buf, nil)))

	ctx := ports.WithTenant(context.Background(), "t_fallback")
	logger.Warn(ctx, "no correlation present")

	if !strings.Contains(buf.String(), `"tenant_id":"t_fallback"`) {
		t.Fatalf("tenant fallback missing: %s", buf.String())
	}
}

func TestNoopsAreUsable(t *testing.T) {
	o := Fill(ports.Observability{})
	ctx := context.Background()
	o.Logger.Info(ctx, "discarded")
	_, span := o.Tracer.Start(ctx, "test")
	span.SetAttrs(ports.A("k", "v"))
	span.End(nil)
	span.End(nil) // double End must be safe
	o.Meter.Counter("c").Add(ctx, 1)
	o.Meter.Histogram("h").Record(ctx, 1.5)
}
