package memstore

import (
	"testing"

	"github.com/Rubentxu/golem/internal/ports"
	"github.com/Rubentxu/golem/tck"
)

// The in-memory search index must conform to the SearchIndex TCK: it is
// the baseline for the OpenSearch reference adapter (ADR-015/046).
func TestSearchConformance(t *testing.T) {
	tck.RunSearchIndexTCK(t, func() ports.SearchIndex { return NewSearch() })
}
