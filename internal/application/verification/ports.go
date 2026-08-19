// Package verification hosts the application handlers of the Verification bounded context.
package verification

import "context"

// TestRunReader is the narrow port for reading test run data.
type TestRunReader interface {
	// GetTestRun returns the test run for a given run ID.
	// Returns ErrTestRunNotFound if the test run does not exist.
	GetTestRun(ctx context.Context, tenant, runID string) (*TestRun, error)
}

// TestRun represents a verification test run.
type TestRun struct {
	RunID    string
	TestCase string
	Status   string
	Verifies string // requirement ID this test run verifies
}
