package webhook

import (
	"context"
	"testing"

	"github.com/Rubentxu/golem/internal/ports"
)

func TestClient_Page_NoOp(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// No keys configured - should be no-op
	client := NewClient("", "")
	err := client.Page(ctx, ports.Alert{
		Severity: ports.AlertSeverityCritical,
		Message:  "test",
	})
	if err != nil {
		t.Fatalf("Page with no keys should be no-op, got error: %v", err)
	}
}

func TestPagerWebhook_Config(t *testing.T) {
	t.Parallel()

	client := NewClient("my-routing-key", "my-service-key")
	if client.PDRoutingKey() != "my-routing-key" {
		t.Errorf("expected routing key 'my-routing-key', got '%s'", client.PDRoutingKey())
	}
	if client.PDServiceKey() != "my-service-key" {
		t.Errorf("expected service key 'my-service-key', got '%s'", client.PDServiceKey())
	}
}

func TestPDSeverity(t *testing.T) {
	t.Parallel()

	tests := []struct {
		severity ports.AlertSeverity
		want     string
	}{
		{ports.AlertSeverityCritical, "critical"},
		{ports.AlertSeverityHigh, "error"},
		{ports.AlertSeverityMedium, "warning"},
		{ports.AlertSeverityLow, "info"},
	}

	for _, tt := range tests {
		got := pdSeverity(tt.severity)
		if got != tt.want {
			t.Errorf("pdSeverity(%v) = %q, want %q", tt.severity, got, tt.want)
		}
	}
}
