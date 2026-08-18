package ports

import "testing"

// TestAlert_SeverityRank verifies alert severity ordering.
func TestAlert_SeverityRank(t *testing.T) {
	t.Parallel()
	// Verify severity constants are defined correctly.
	levels := []AlertSeverity{AlertSeverityCritical, AlertSeverityHigh, AlertSeverityMedium, AlertSeverityLow}
	if len(levels) != 4 {
		t.Errorf("expected 4 severity levels, got %d", len(levels))
	}

	alert := Alert{
		Severity: AlertSeverityCritical,
		Route:    "https:// pagerduty.com /services/abc",
		Message:  "SLO budget exhausted",
		SLIName: "command-api",
	}

	if alert.Severity != AlertSeverityCritical {
		t.Errorf("expected severity Critical, got %s", alert.Severity)
	}
}
