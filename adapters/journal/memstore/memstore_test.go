package memstore

import (
	"testing"

	"github.com/Rubentxu/golem/internal/ports"
	"github.com/Rubentxu/golem/tck"
)

// The in-memory journal must conform to the JournalStore TCK (ADR-046):
// it is the reference semantics for every future adapter.
func TestJournalConformance(t *testing.T) {
	tck.RunJournalStoreTCK(t, func() ports.JournalStore { return NewJournal() })
}
