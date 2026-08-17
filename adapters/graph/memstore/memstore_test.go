package memstore

import (
	"testing"

	"github.com/Rubentxu/golem/internal/ports"
	"github.com/Rubentxu/golem/tck"
)

// The in-memory graph must conform to the GraphStore TCK (ADR-046): it is
// the baseline semantics for SP-001 graph database candidates.
func TestGraphConformance(t *testing.T) {
	tck.RunGraphStoreTCK(t, func() ports.GraphStore { return NewGraph() })
}
