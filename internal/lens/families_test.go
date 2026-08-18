package lens

import (
	"context"
	"testing"

	graphmem "github.com/Rubentxu/golem/adapters/graph/memstore"
	"github.com/Rubentxu/golem/internal/ports"
)

// seedSupplyChain builds the S14 walk shape:
// component-1 →(CONTAINS) sbom-1 →(HAS_SBOM) artifact-1 →(RELEASED_AS) release-1
// and vulnerability-1 →(AFFECTED_BY) component-1.
func seedSupplyChain(t *testing.T) ports.GraphStore {
	t.Helper()
	g := graphmem.NewGraph()
	ops := []ports.GraphOp{
		{Kind: ports.OpUpsertNode, Target: "component-1", Data: map[string]any{"kind": "PackageComponent", "attributes": map[string]any{"purl": "pkg:npm/foo@1.0.0"}}},
		{Kind: ports.OpUpsertNode, Target: "sbom-1", Data: map[string]any{"kind": "SBOM", "attributes": map[string]any{"format": "spdx"}}},
		{Kind: ports.OpUpsertNode, Target: "artifact-1", Data: map[string]any{"kind": "Artifact", "attributes": map[string]any{"sha256": "a"}}},
		{Kind: ports.OpUpsertNode, Target: "release-1", Data: map[string]any{"kind": "Release", "attributes": map[string]any{"name": "v1.0"}}},
		{Kind: ports.OpUpsertNode, Target: "cve-1", Data: map[string]any{"kind": "Vulnerability", "attributes": map[string]any{"cve": "CVE-2026-1"}}},
		{Kind: ports.OpUpsertEdge, Target: "e-contains", Data: map[string]any{"type": "CONTAINS", "source": "sbom-1", "target": "component-1"}},
		{Kind: ports.OpUpsertEdge, Target: "e-has-sbom", Data: map[string]any{"type": "HAS_SBOM", "source": "artifact-1", "target": "sbom-1"}},
		{Kind: ports.OpUpsertEdge, Target: "e-released-as", Data: map[string]any{"type": "RELEASED_AS", "source": "artifact-1", "target": "release-1"}},
		{Kind: ports.OpUpsertEdge, Target: "e-affected", Data: map[string]any{"type": "AFFECTED_BY", "source": "component-1", "target": "cve-1"}},
	}
	if _, err := g.Apply(context.Background(), ports.GraphMutation{TenantID: "t-fam", Ops: ops}); err != nil {
		t.Fatal(err)
	}
	return g
}

// S14 — VulnerabilityImpactLens reaches the affected release through the
// M4 blast-radius walk.
func TestVulnerabilityImpactLens(t *testing.T) {
	g := seedSupplyChain(t)
	spec := VulnerabilityImpactLens([]string{"component-1"}, 5, 50, 50)
	res, err := Execute(context.Background(), g, "t-fam", spec)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	ids := map[string]bool{}
	for _, n := range res.Nodes {
		ids[n.ID] = true
	}
	for _, want := range []string{"component-1", "sbom-1", "artifact-1", "release-1", "cve-1"} {
		if !ids[want] {
			t.Errorf("VulnerabilityImpactLens missing %s (got %v)", want, ids)
		}
	}
}

// S15 — ReleaseEvidenceLens from the release root returns the full chain.
func TestReleaseEvidenceLens(t *testing.T) {
	g := seedSupplyChain(t)
	spec := ReleaseEvidenceLens([]string{"release-1"}, 5, 50, 50)
	res, err := Execute(context.Background(), g, "t-fam", spec)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	kinds := map[string]int{}
	for _, n := range res.Nodes {
		kinds[n.Kind]++
	}
	if kinds["Release"] != 1 {
		t.Errorf("ReleaseEvidenceLens must include the Release root, got %v", kinds)
	}
	if kinds["SBOM"] == 0 || kinds["PackageComponent"] == 0 {
		t.Errorf("evidence chain incomplete: %v", kinds)
	}
}
