package behavior

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// runIDs is a per-run deterministic ID generator: IDs derive from the
// triggering event and the emission sequence, so re-executing the same
// event produces byte-identical outputs — the "what-if reproducible"
// property by construction (BEHAVIOR_RUNTIME.md §Determinismo + the M6
// exit criterion).
type runIDs struct {
	base string
	n    int
}

// newRunIDs seeds a deterministic generator for one pipeline run.
func newRunIDs(eventID string) *runIDs {
	return &runIDs{base: eventID}
}

// NewID derives the next deterministic ID for this run.
func (r *runIDs) NewID() string {
	r.n++
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s:%d", r.base, r.n)))
	return hex.EncodeToString(sum[:16])
}
