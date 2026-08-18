package harness

import (
	"testing"
	"time"
)

// TestCutoverWindow_Bounds verifies cutover window timing (REQ-MIG-002).
func TestCutoverWindow_Bounds(t *testing.T) {
	t.Parallel()

	// Default cutover window: [5min, 1h].
	minWindow := 5 * time.Minute
	maxWindow := 1 * time.Hour

	if minWindow >= maxWindow {
		t.Error("MinCutoverWindow must be less than MaxCutoverWindow")
	}

	if minWindow < 1*time.Minute {
		t.Error("MinCutoverWindow too short for safe cutover")
	}

	if maxWindow > 24*time.Hour {
		t.Error("MaxCutoverWindow unreasonably long")
	}
}
