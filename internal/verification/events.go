// Package verification defines the Test bounded context (category
// "Verification" in GRAPH_MODEL): test cases, runs and evidence. UAT
// sessions and campaigns arrive later in M3.
package verification

// TestRunReported is the payload of verification.testrun.reported.v1.
// Verifies points at the entity the run proves — an artifact digest, a
// build id or a commit sha — through a canonical VERIFIES edge.
type TestRunReported struct {
	RunID    string `json:"run_id"`
	TestCase string `json:"case"`
	Status   string `json:"status"` // passed | failed | skipped
	Verifies string `json:"verifies"`
}

// Event type names of this context.
const (
	EventTestRunReported = "verification.testrun.reported.v1"
)
