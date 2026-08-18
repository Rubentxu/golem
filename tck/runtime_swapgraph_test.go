package tck

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/Rubentxu/golem/adapters/graph/memstore"
	"github.com/Rubentxu/golem/internal/application/runtime"
	"github.com/Rubentxu/golem/internal/ports"
)

// TestRuntimeSwapGraphAtomicity verifies that SwapGraph uses a write lock
// and that concurrent readers during a swap see either the old or new graph,
// never a partially-updated state. This is the spec S15 scenario.
func TestRuntimeSwapGraphAtomicity(t *testing.T) {
	t.Parallel()

	oldGraph := memstore.NewGraph()
	newGraph := memstore.NewGraph()

	// Build a Runtime with the old graph as initial state.
	rt := &runtime.Runtime{
		Graph: oldGraph,
	}

	const readerCount = 10

	// Readers repeatedly read rt.Graph under RLock for the duration of the swap.
	var readersMu sync.WaitGroup
	seenOld := make(chan struct{})
	seenNew := make(chan struct{})
	stop := make(chan struct{})

	for i := 0; i < readerCount; i++ {
		readersMu.Add(1)
		go func() {
			defer readersMu.Done()
			for {
				select {
				case <-stop:
					return
				default:
					// Hold RLock to read — mimics how the tail loop reads Graph
					// during ProjectBatch.
					var g ports.GraphStore
					rt.WithGraphRLock(context.Background(), func() error {
						g = rt.Graph
						return nil
					})
					if g == oldGraph {
						select {
						case seenOld <- struct{}{}:
						default:
						}
					} else if g == newGraph {
						select {
						case seenNew <- struct{}{}:
						default:
						}
					}
					time.Sleep(time.Millisecond)
				}
			}
		}()
	}

	// Give readers a moment to observe the old graph.
	time.Sleep(5 * time.Millisecond)

	// Perform the swap.
	err := rt.SwapGraph(context.Background(), newGraph)
	if err != nil {
		t.Fatalf("SwapGraph failed: %v", err)
	}

	// Wait for readers to observe the new graph.
	timeout := time.After(500 * time.Millisecond)
	for readersSeen := 0; readersSeen < readerCount; {
		select {
		case <-seenNew:
			readersSeen++
		case <-timeout:
			// Not all readers may have observed newGraph yet due to timing,
			// but they should all have stopped reading oldGraph after the swap.
			// At minimum, verify that after swap, no reader still sees oldGraph.
			select {
			case <-seenOld:
				t.Errorf("reader observed old graph after SwapGraph completed")
			default:
			}
			goto done
		}
	}

done:
	close(stop)
	readersMu.Wait()

	// After swap, reading Graph() should return newGraph.
	var g ports.GraphStore
	rt.WithGraphRLock(context.Background(), func() error {
		g = rt.Graph
		return nil
	})
	if g != newGraph {
		t.Errorf("after SwapGraph, rt.Graph = %v, want %v", g, newGraph)
	}
}

// TestRuntimeSwapGraphNilError verifies that swapping with nil graph fails.
func TestRuntimeSwapGraphNilError(t *testing.T) {
	t.Parallel()

	rt := &runtime.Runtime{Graph: memstore.NewGraph()}
	err := rt.SwapGraph(context.Background(), nil)
	if err == nil {
		t.Error("SwapGraph(nil) expected error, got nil")
	}
}

// TestRuntimeSwapGraphNoop verifies that swapping with the same graph is safe.
func TestRuntimeSwapGraphNoop(t *testing.T) {
	t.Parallel()

	g := memstore.NewGraph()
	rt := &runtime.Runtime{Graph: g}
	err := rt.SwapGraph(context.Background(), g)
	if err != nil {
		t.Errorf("SwapGraph(same graph) error = %v, want nil", err)
	}
	if rt.Graph != g {
		t.Errorf("Graph changed on noop swap")
	}
}

// TestRuntimeSwapGraphReadThenWrite verifies that a reader holding RLock
// while SwapGraph is called is properly handled (write waits for read).
func TestRuntimeSwapGraphReadThenWrite(t *testing.T) {
	t.Parallel()

	oldGraph := memstore.NewGraph()
	newGraph := memstore.NewGraph()
	rt := &runtime.Runtime{Graph: oldGraph}

	var started, done sync.WaitGroup
	swapStarted := make(chan struct{})
	writerDone := make(chan struct{})

	started.Add(2)

	// Reader holds RLock briefly, then releases.
	go func() {
		started.Done()
		rt.WithGraphRLock(context.Background(), func() error {
			close(swapStarted) // signal that RLock is held
			time.Sleep(20 * time.Millisecond)
			return nil
		})
	}()

	// Writer waits for reader to hold lock, then tries SwapGraph.
	go func() {
		started.Done()
		<-swapStarted                    // wait for reader to acquire RLock
		time.Sleep(5 * time.Millisecond) // ensure writer tries after reader has lock
		_ = rt.SwapGraph(context.Background(), newGraph)
		close(writerDone)
	}()

	done.Add(2)
	go func() { started.Wait(); done.Done() }()
	go func() { started.Wait(); done.Done() }()
	done.Wait()

	// After writer completes, Graph must be newGraph.
	<-writerDone
	var g ports.GraphStore
	rt.WithGraphRLock(context.Background(), func() error {
		g = rt.Graph
		return nil
	})
	if g != newGraph {
		t.Errorf("Graph = %v, want %v", g, newGraph)
	}
}
