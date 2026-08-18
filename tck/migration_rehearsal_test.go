package tck

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"testing"
	"time"

	checkpointmem "github.com/Rubentxu/golem/adapters/checkpoint/memstore"
	graphmem "github.com/Rubentxu/golem/adapters/graph/memstore"
	journalmem "github.com/Rubentxu/golem/adapters/journal/memstore"
	registrymem "github.com/Rubentxu/golem/adapters/registry/memstore"
	transportmem "github.com/Rubentxu/golem/adapters/transport/memstore"
	harnesspkg "github.com/Rubentxu/golem/internal/application/migration/harness"
	app_runtime "github.com/Rubentxu/golem/internal/application/runtime"
	"github.com/Rubentxu/golem/internal/clock"
	idgen "github.com/Rubentxu/golem/internal/ids"
	"github.com/Rubentxu/golem/internal/ports"
)

// testStack holds an isolated wiring per scenario (per-scenario isolated stack, M4 convention).
type testStack struct {
	Journal    *journalmem.Store
	Checkpoint *checkpointmem.Store
	Registry   *registrymem.Registry
	Transport  *transportmem.Transport
	Source     *graphmem.Store
	Target     *graphmem.Store
	Runtime    *app_runtime.Runtime
	clk        ports.Clock
	idgen      ports.IDGenerator
	opts       harnesspkg.HarnessOptions
	harnessID  string
}

// newTestStack creates an isolated wiring per scenario.
func newTestStack(t *testing.T, observeWindow time.Duration) *testStack {
	journal := journalmem.NewJournal()
	cp := checkpointmem.NewCheckpoints()
	source := graphmem.NewGraph()
	target := graphmem.NewGraph()
	clk := clock.Fixed(time.Now())
	idg := idgen.NewGenerator(clk)

	tenantID := ports.TenantID("test-tenant")
	opts := harnesspkg.DefaultHarnessOptions(tenantID)
	opts.ObserveWindow = observeWindow

	return &testStack{
		Journal:    journal,
		Checkpoint: cp,
		Source:     source,
		Target:     target,
		clk:        clk,
		idgen:      idg,
		opts:       opts,
		harnessID:  t.Name(),
	}
}

// buildRuntime wires a minimal runtime with source as the initial graph.
// After the cutover event, the host calls rt.SwapGraph(target).
func (s *testStack) buildRuntime(t *testing.T) {
	rt, err := app_runtime.New(app_runtime.Options{
		Journal:    s.Journal,
		Graph:      s.Source,
		Registry:   registrymem.NewRegistry(),
		Transport:  transportmem.NewTransport(),
		Checkpoint: s.Checkpoint,
		Clock:      s.clk,
		IDs:        s.idgen,
	})
	if err != nil {
		t.Fatalf("runtime.New error = %v", err)
	}
	s.Runtime = rt
}

// buildHarness builds the migration harness for this stack.
func (s *testStack) buildHarness() *harnesspkg.Harness {
	return harnesspkg.NewHarness(
		s.harnessID,
		s.Journal,
		s.Checkpoint,
		harnesspkg.HarnessEndpoint{Graph: s.Source, Journal: s.Journal},
		harnesspkg.HarnessEndpoint{Graph: s.Target, Journal: s.Journal},
		s.opts,
		s.clk,
		s.idgen,
	)
}

// SeedGraph seeds a graph with numNodes and numEdges deterministically.
func SeedGraph(ctx context.Context, g *graphmem.Store, tenant ports.TenantID, numNodes, numEdges int, src rand.Source) {
	rng := rand.New(src)
	for i := 0; i < numNodes; i++ {
		nodeID := fmt.Sprintf("n%d", i)
		_, err := g.Apply(ctx, ports.GraphMutation{
			TenantID: tenant,
			Ops: []ports.GraphOp{
				{
					Kind:   ports.OpUpsertNode,
					Target: nodeID,
					Data: map[string]any{
						"kind": "test:module",
						"attributes": map[string]any{
							"name":        fmt.Sprintf("module-%d", i),
							"version":     fmt.Sprintf("v1.0.%d", i),
							"status":      "active",
							"color":       "blue",
							"priority":    i % 5,
							"weight":      float64(i%100) / 100.0,
							"description": fmt.Sprintf("deterministic node %d", i),
						},
					},
				},
			},
		})
		if err != nil {
			panic(fmt.Sprintf("seed node %s: %v", nodeID, err))
		}
	}

	edgeCount := 0
	for edgeCount < numEdges {
		srcIdx := rng.Intn(numNodes)
		tgtIdx := rng.Intn(numNodes)
		if srcIdx == tgtIdx {
			continue
		}
		edgeID := fmt.Sprintf("e%d", edgeCount)
		color := "blue"
		if edgeCount%2 == 0 {
			color = "green"
		}
		_, err := g.Apply(ctx, ports.GraphMutation{
			TenantID: tenant,
			Ops: []ports.GraphOp{
				{
					Kind:   ports.OpUpsertEdge,
					Target: edgeID,
					Data: map[string]any{
						"type":   "DEPENDS_ON",
						"source": fmt.Sprintf("n%d", srcIdx),
						"target": fmt.Sprintf("n%d", tgtIdx),
						"attributes": map[string]any{
							"color":       color,
							"weight":      float64(edgeCount%100) / 100.0,
							"description": fmt.Sprintf("deterministic edge %d", edgeCount),
						},
					},
				},
			},
		})
		if err != nil {
			continue
		}
		edgeCount++
	}
}

// harnessEventTypes returns the ordered list of event types from the harness stream.
func harnessEventTypes(ctx context.Context, journal *journalmem.Store, harnessID string) ([]string, error) {
	streamID := fmt.Sprintf("harness:%s", harnessID)
	events, err := journal.ReadStream(ctx, "system", streamID, 0)
	if err != nil {
		return nil, err
	}
	types := make([]string, 0, len(events))
	for _, e := range events {
		types = append(types, e.EventType)
	}
	return types, nil
}

// readHarnessEvents returns all events from the harness stream.
func readHarnessEvents(ctx context.Context, journal *journalmem.Store, harnessID string) ([]ports.RawEvent, error) {
	streamID := fmt.Sprintf("harness:%s", harnessID)
	return journal.ReadStream(ctx, "system", streamID, 0)
}

// auditPayload parses the JSON payload of a raw event.
func auditPayload(t *testing.T, e ports.RawEvent) map[string]any {
	var payload map[string]any
	if err := json.Unmarshal(e.Payload, &payload); err != nil {
		t.Fatalf("unmarshal payload of event %s: %v", e.EventType, err)
	}
	return payload
}

// TestR4HappyPath verifies the full R4 rehearsal with identical source and target.
// Source = memstore with 100 nodes / 200 edges (deterministic seed 42).
// Target = memstore fresh (empty, loaded from source snapshot during harness run).
// GOLEM_MIGRATION_OBSERVE_WINDOW=100ms.
//
// Acceptance:
//   - harness.Run() completes without rollback (emits completed event)
//   - 4 audit events emitted in order: started, diffed, cutover, completed
//   - diffed event has diff_counts = {nodes:0, edges:0}
//   - After host-side SwapGraph, runtime.Graph points to target
func TestR4HappyPath(t *testing.T) {
	ctx := context.Background()

	orig := os.Getenv("GOLEM_MIGRATION_OBSERVE_WINDOW")
	os.Setenv("GOLEM_MIGRATION_OBSERVE_WINDOW", "100ms")
	defer func() { _ = os.Setenv("GOLEM_MIGRATION_OBSERVE_WINDOW", orig) }()

	stack := newTestStack(t, 100*time.Millisecond)
	tenant := ports.TenantID("test-tenant")
	stack.opts.SampleSeed = 42

	// Seed source; target starts empty — harness loading populates it identically.
	SeedGraph(ctx, stack.Source, tenant, 100, 200, rand.NewSource(42))

	stack.buildRuntime(t)
	h := stack.buildHarness()

	if err := h.Run(ctx); err != nil {
		t.Fatalf("harness.Run error = %v", err)
	}

	// Verify 4 audit events in order (no rollback).
	types, err := harnessEventTypes(ctx, stack.Journal, h.ID)
	if err != nil {
		t.Fatalf("harnessEventTypes error = %v", err)
	}
	wantEvents := []string{
		ports.EventMigrationHarnessStarted,
		ports.EventMigrationHarnessDiffed,
		ports.EventMigrationHarnessCutover,
		ports.EventMigrationHarnessCompleted,
	}
	if len(types) != len(wantEvents) {
		t.Fatalf("got %d audit events, want %d: %v", len(types), len(wantEvents), types)
	}
	for i, want := range wantEvents {
		if types[i] != want {
			t.Errorf("audit event[%d] = %q, want %q", i, types[i], want)
		}
	}

	// Verify diffed event has diff_counts = 0 (identical graphs after loading).
	events, err := readHarnessEvents(ctx, stack.Journal, h.ID)
	if err != nil {
		t.Fatalf("readHarnessEvents error = %v", err)
	}
	for _, e := range events {
		if e.EventType == ports.EventMigrationHarnessDiffed {
			payload := auditPayload(t, e)
			diffs, ok := payload["diff_counts"].(map[string]any)
			if !ok {
				t.Fatal("diffed event missing diff_counts")
			}
			if diffs["nodes"].(float64) != 0 || diffs["edges"].(float64) != 0 {
				t.Errorf("diff_counts = %v, want nodes=0, edges=0", diffs)
			}
		}
	}

	// Verify cutover event was emitted.
	cutoverSeen := false
	for _, e := range events {
		if e.EventType == ports.EventMigrationHarnessCutover {
			cutoverSeen = true
			break
		}
	}
	if !cutoverSeen {
		t.Fatal("cutover event not found — host-side SwapGraph cannot be triggered")
	}

	// Simulate host-side cutover: call SwapGraph(target) after cutover event.
	if err := stack.Runtime.SwapGraph(ctx, stack.Target); err != nil {
		t.Fatalf("SwapGraph error = %v", err)
	}

	// Verify runtime.Graph now points to target.
	if stack.Runtime.Graph != stack.Target {
		t.Error("runtime.Graph should point to target after SwapGraph")
	}
}

// TestR4StructuralDivergence verifies that the harness diff correctly detects
// structural divergence between source and target graphs.
func TestR4StructuralDivergence(t *testing.T) {
	ctx := context.Background()
	tenant := ports.TenantID("test-tenant")

	// Test 1: Extra node in target (target has 101 nodes, source has 100).
	t.Run("extra_node_in_target", func(t *testing.T) {
		source := graphmem.NewGraph()
		target := graphmem.NewGraph()
		SeedGraph(ctx, source, tenant, 100, 200, rand.NewSource(42))
		// Target gets source's 100 nodes PLUS one extra divergent node.
		SeedGraph(ctx, target, tenant, 100, 200, rand.NewSource(42))
		_, _ = target.Apply(ctx, ports.GraphMutation{
			TenantID: tenant,
			Ops: []ports.GraphOp{
				{
					Kind:   ports.OpUpsertNode,
					Target: "n-divergent",
					Data: map[string]any{
						"kind": "test:module",
						"attributes": map[string]any{
							"name":  "extra-node",
							"color": "red",
						},
					},
				},
			},
		})

		result, err := harnesspkg.Diff(ctx, source, target, tenant, 42)
		if err != nil {
			t.Fatalf("Diff error = %v", err)
		}
		if result.CutoverSafe {
			t.Error("CutoverSafe should be false with extra node in target")
		}
		if result.CountsMatch {
			t.Error("CountsMatch should be false with extra node in target")
		}
	})

	// Test 2: Different node counts (target has 60 nodes, source has 100).
	t.Run("different_node_counts", func(t *testing.T) {
		source := graphmem.NewGraph()
		target := graphmem.NewGraph()
		SeedGraph(ctx, source, tenant, 100, 200, rand.NewSource(42))
		SeedGraph(ctx, target, tenant, 60, 0, rand.NewSource(99)) // partial, different seed

		result, err := harnesspkg.Diff(ctx, source, target, tenant, 42)
		if err != nil {
			t.Fatalf("Diff error = %v", err)
		}
		if result.CutoverSafe {
			t.Error("CutoverSafe should be false with different node counts")
		}
		if result.CountsMatch {
			t.Error("CountsMatch should be false with 60 vs 100 nodes")
		}
	})

	// Test 3: Identical graphs — CutoverSafe=true.
	t.Run("identical_graphs", func(t *testing.T) {
		source := graphmem.NewGraph()
		target := graphmem.NewGraph()
		SeedGraph(ctx, source, tenant, 100, 200, rand.NewSource(42))
		SeedGraph(ctx, target, tenant, 100, 200, rand.NewSource(42)) // same seed

		result, err := harnesspkg.Diff(ctx, source, target, tenant, 42)
		if err != nil {
			t.Fatalf("Diff error = %v", err)
		}
		if !result.CutoverSafe {
			t.Error("CutoverSafe should be true for identical graphs")
		}
		if !result.CountsMatch {
			t.Error("CountsMatch should be true for identical graphs")
		}
		if result.NodeDiffs != 0 {
			t.Errorf("NodeDiffs = %d, want 0 for identical graphs", result.NodeDiffs)
		}
		if result.EdgeDiffs != 0 {
			t.Errorf("EdgeDiffs = %d, want 0 for identical graphs", result.EdgeDiffs)
		}
	})
}
