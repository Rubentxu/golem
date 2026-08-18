package bbolt

import (
	"testing"
)

// TestSplitEventTypeGolden verifies that splitEventType produces identical output
// to the hand-written implementation for known inputs (S33).
func TestSplitEventTypeGolden(t *testing.T) {
	cases := []struct {
		input    string
		expected []string
	}{
		{
			input:    "migration.harness.started.v1",
			expected: []string{"migration", "harness", "started", "v1"},
		},
		{
			input:    "extension.pack.activated.v1",
			expected: []string{"extension", "pack", "activated", "v1"},
		},
		{
			input:    "single",
			expected: []string{"single"},
		},
		{
			input:    ".leading.dot",
			expected: []string{"leading", "dot"},
		},
		{
			input:    "trailing.dot.",
			expected: []string{"trailing", "dot"},
		},
		{
			input:    "empty..parts",
			expected: []string{"empty", "parts"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			got := splitEventType(tc.input)
			if len(got) != len(tc.expected) {
				t.Fatalf("len=%d, want %d; input=%q", len(got), len(tc.expected), tc.input)
			}
			for i := range got {
				if got[i] != tc.expected[i] {
					t.Errorf("got[%d]=%q, want[%d]=%q; input=%q", i, got[i], i, tc.expected[i], tc.input)
				}
			}
		})
	}
}
