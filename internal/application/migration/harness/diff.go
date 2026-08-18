package harness

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Rubentxu/golem/internal/application/migration/samples"
	"github.com/Rubentxu/golem/internal/ports"
)

// DiffResult holds the outcome of a migration diff.
type DiffResult struct {
	SamplesChecked int      `json:"samples_checked"`
	NodeDiffs      int      `json:"node_diffs"`
	EdgeDiffs      int      `json:"edge_diffs"`
	DiffDetails    []string `json:"diff_details,omitempty"`
	CountsMatch    bool     `json:"counts_match"`
	CutoverSafe    bool     `json:"cutover_safe"` // true iff Diffs == 0 && CountsMatch
}

// Diff compares source and target graphs using deterministic sampling.
// It performs 10 GetNode samples and 3 Traversal queries, plus structural
// counts. Diffs >= 1 → CutoverSafe=false.
func Diff(ctx context.Context, source, target ports.GraphStore, tenant ports.TenantID, sampleSeed uint64) (DiffResult, error) {
	sampler := samples.NewSampler(sampleSeed)
	res := DiffResult{CountsMatch: true}

	// Collect all nodes from both graphs for sampling.
	sourceNodes, err := source.ListNodes(ctx, tenant)
	if err != nil {
		return DiffResult{}, fmt.Errorf("list source nodes: %w", err)
	}
	targetNodes, err := target.ListNodes(ctx, tenant)
	if err != nil {
		return DiffResult{}, fmt.Errorf("list target nodes: %w", err)
	}

	// Count nodes by kind.
	sourceNodeCounts := countByKind(sourceNodes)
	targetNodeCounts := countByKind(targetNodes)
	if !mapsEqual(sourceNodeCounts, targetNodeCounts) {
		res.CountsMatch = false
		res.DiffDetails = append(res.DiffDetails, "node counts by kind differ")
	}

	// Collect all edges for count comparison.
	sourceEdges, err := source.ListEdges(ctx, tenant)
	if err != nil {
		return DiffResult{}, fmt.Errorf("list source edges: %w", err)
	}
	targetEdges, err := target.ListEdges(ctx, tenant)
	if err != nil {
		return DiffResult{}, fmt.Errorf("list target edges: %w", err)
	}

	sourceEdgeCounts := countByType(sourceEdges)
	targetEdgeCounts := countByType(targetEdges)
	if !mapsEqual(sourceEdgeCounts, targetEdgeCounts) {
		res.CountsMatch = false
		res.DiffDetails = append(res.DiffDetails, "edge counts by type differ")
	}

	// 10 GetNode samples.
	sampledIDs := sampler.SampleNodeIDs(sourceNodes, 10)
	for _, id := range sampledIDs {
		res.SamplesChecked++
		sn, err1 := source.GetNode(ctx, tenant, id)
		tn, err2 := target.GetNode(ctx, tenant, id)
		if err1 != nil || err2 != nil {
			if err1 != err2 {
				res.NodeDiffs++
				res.DiffDetails = append(res.DiffDetails, fmt.Sprintf("node %s: error mismatch", id))
			}
			continue
		}
		if !nodesEqual(sn, tn) {
			res.NodeDiffs++
			res.DiffDetails = append(res.DiffDetails, fmt.Sprintf("node %s: content differs", id))
		}
	}

	// 3 Traversal samples (MaxDepth=3, MaxNodes=100, MaxEdges=200).
	roots := sampler.SampleTraversalRoots(sourceNodes, 3)
	for _, root := range roots {
		srcSub, err1 := source.Traversal(ctx, ports.TraversalQuery{
			TenantID: tenant,
			Roots:    []string{root},
			MaxDepth: 3,
			MaxNodes: 100,
			MaxEdges: 200,
		})
		tgtSub, err2 := target.Traversal(ctx, ports.TraversalQuery{
			TenantID: tenant,
			Roots:    []string{root},
			MaxDepth: 3,
			MaxNodes: 100,
			MaxEdges: 200,
		})
		if err1 != nil || err2 != nil {
			if err1 != err2 {
				res.EdgeDiffs++
				res.DiffDetails = append(res.DiffDetails, fmt.Sprintf("traversal from %s: error mismatch", root))
			}
			continue
		}
		if !subgraphsEqual(srcSub, tgtSub) {
			res.EdgeDiffs++
			res.DiffDetails = append(res.DiffDetails, fmt.Sprintf("traversal from %s: subgraph differs", root))
		}
	}

	res.CutoverSafe = res.NodeDiffs == 0 && res.EdgeDiffs == 0 && res.CountsMatch
	return res, nil
}

// countByKind returns a map of kind → count.
func countByKind(nodes []ports.Node) map[string]int {
	m := map[string]int{}
	for _, n := range nodes {
		m[n.Kind]++
	}
	return m
}

// countByType returns a map of edge type → count.
func countByType(edges []ports.Edge) map[string]int {
	m := map[string]int{}
	for _, e := range edges {
		m[e.Type]++
	}
	return m
}

// mapsEqual compares two string→int maps.
func mapsEqual(a, b map[string]int) bool {
	if len(a) != len(b) {
		return false
	}
	for k, va := range a {
		if vb, ok := b[k]; !ok || va != vb {
			return false
		}
	}
	return true
}

// nodesEqual returns true if two nodes are JSON-equal.
func nodesEqual(a, b ports.Node) bool {
	aj, _ := json.Marshal(a)
	bj, _ := json.Marshal(b)
	return string(aj) == string(bj)
}

// subgraphsEqual returns true if two subgraphs are JSON-equal.
func subgraphsEqual(a, b ports.Subgraph) bool {
	aj, _ := json.Marshal(a)
	bj, _ := json.Marshal(b)
	return string(aj) == string(bj)
}
