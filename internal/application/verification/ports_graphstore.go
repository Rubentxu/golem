// Package verification provides narrow-port adapters over the graph store.
package verification

import (
	"context"
	"errors"

	"github.com/Rubentxu/golem/internal/ports"
)

// graphStoreTestRunReader implements TestRunReader over a GraphStore.
type graphStoreTestRunReader struct {
	gs ports.GraphStore
}

// NewTestRunReaderOverGraphStore creates a TestRunReader that delegates to gs.
func NewTestRunReaderOverGraphStore(gs ports.GraphStore) TestRunReader {
	return &graphStoreTestRunReader{gs: gs}
}

// ErrTestRunNotFound is returned when the test run does not exist.
var ErrTestRunNotFound = errors.New("test run not found")

// GetTestRun implements TestRunReader by reading the TestRun node and extracting attributes.
func (r *graphStoreTestRunReader) GetTestRun(ctx context.Context, tenant, runID string) (*TestRun, error) {
	n, err := r.gs.GetNode(ctx, ports.TenantID(tenant), runID)
	if err != nil {
		if errors.Is(err, ports.ErrNodeNotFound) {
			return nil, nil // SCN-PORT-08: returns (nil, nil) for not-found
		}
		return nil, err
	}
	if n.Kind != "TestRun" {
		return nil, nil
	}
	return &TestRun{
		RunID:    runID,
		TestCase: stringAttr(n, "test_case"),
		Status:   stringAttr(n, "status"),
		Verifies: stringAttr(n, "verifies"),
	}, nil
}

func stringAttr(n ports.Node, k string) string {
	if s, ok := n.Attributes[k].(string); ok {
		return s
	}
	return ""
}
