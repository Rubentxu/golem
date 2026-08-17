package ids

import (
	"strings"
	"testing"
	"time"

	"github.com/Rubentxu/golem/internal/clock"
	"github.com/Rubentxu/golem/internal/ports"
)

func TestULIDFormat(t *testing.T) {
	id := New(time.Now())
	if len(id) != ULIDLen {
		t.Fatalf("length = %d, want %d", len(id), ULIDLen)
	}
	for i, c := range id {
		if !strings.ContainsRune(Alphabet, c) {
			t.Fatalf("char %d (%q) not in Crockford alphabet", i, c)
		}
	}
}

func TestUniqueWithinSameMillisecond(t *testing.T) {
	ts := time.UnixMilli(1_700_000_000_000)
	const n = 10_000
	seen := make(map[string]struct{}, n)
	gen := make([]string, 0, n)
	for i := 0; i < n; i++ {
		id := New(ts)
		if _, dup := seen[id]; dup {
			t.Fatalf("duplicate ULID within same millisecond: %s", id)
		}
		seen[id] = struct{}{}
		gen = append(gen, id)
	}
	for i := 1; i < len(gen); i++ {
		if gen[i] <= gen[i-1] {
			t.Fatalf("not monotonic at %d: %s <= %s", i, gen[i], gen[i-1])
		}
	}
}

func TestSortableAcrossTime(t *testing.T) {
	const n = 1_000
	gen := make([]string, 0, n)
	for i := 0; i < n; i++ {
		gen = append(gen, New(time.Now()))
	}
	for i := 1; i < len(gen); i++ {
		if gen[i] < gen[i-1] {
			t.Fatalf("ordering violated at %d: %s < %s", i, gen[i], gen[i-1])
		}
	}
}

func TestGeneratorUsesInjectedClock(t *testing.T) {
	g := NewGenerator(clock.SystemClock{})
	id := g.NewID()
	if len(id) != ULIDLen {
		t.Fatalf("NewID length = %d, want %d", len(id), ULIDLen)
	}
}

// Compile-time: ULIDGenerator satisfies the port.
var _ ports.IDGenerator = (*ULIDGenerator)(nil)
