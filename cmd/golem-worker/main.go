// Command golem-worker runs background projections and behaviors.
//
// Skeleton for Fase 0 (bootstrap). From M1 it consumes accepted events and
// maintains the derived stores (graph projection, search, analytics),
// which must remain rebuildable from the Journal (ADR-049).
package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "golem-worker: skeleton — projection workers land in M1 (Journal → graph projection → replay)")
	os.Exit(0)
}
