// Package supplychain hosts the application handlers of the Supply Chain
// bounded context: commands validated by domain rules and expressed as event
// drafts for the command bus.
package supplychain

import (
	"context"
	"errors"
	"net/url"
	"strings"

	"github.com/Rubentxu/golem/internal/application/projection"
	"github.com/Rubentxu/golem/internal/ports"
	domainsupplychain "github.com/Rubentxu/golem/internal/supplychain"
)

// contextKey is a custom type to avoid collisions in context.WithValue.
type contextKey string

const blastRadiusMaxNodesKey contextKey = "supplychain.blast_radius.max_nodes"
const blastRadiusMaxEdgesKey contextKey = "supplychain.blast_radius.max_edges"

// BlastRadiusResult describes which releases are affected by a given component.
type BlastRadiusResult struct {
	Component string           `json:"component"`
	Releases  []BlastRadiusHit `json:"releases"`
	Truncated bool             `json:"truncated"`
}

// BlastRadiusHit describes one release candidate that is affected by the queried component.
type BlastRadiusHit struct {
	ReleaseID string `json:"release_id"`
	Name      string `json:"name,omitempty"`
}

// MaxBlastRadiusNodes is the safety bound for blast radius traversal.
const MaxBlastRadiusNodes = 500

// MaxBlastRadiusEdges is the safety bound for blast radius traversal.
const MaxBlastRadiusEdges = 2000

// ErrInvalidPurlForBlast is returned when the component purl is not a valid URL-encoded purl.
var ErrInvalidPurlForBlast = errors.New("supplychain: invalid purl for blast radius")

// WithMaxNodes returns a context that overrides the MaxNodes safety bound for
// BlastRadius traversal. This is intended for test use only.
func WithMaxNodes(ctx context.Context, maxNodes int) context.Context {
	return context.WithValue(ctx, blastRadiusMaxNodesKey, maxNodes)
}

// WithMaxEdges returns a context that overrides the MaxEdges safety bound for
// BlastRadius traversal. This is intended for test use only.
func WithMaxEdges(ctx context.Context, maxEdges int) context.Context {
	return context.WithValue(ctx, blastRadiusMaxEdgesKey, maxEdges)
}

// BlastRadius computes the set of releases whose artifacts contain the given component.
// It traverses: component → CONTAINS⁻¹ → SBOM → HAS_SBOM⁻¹ → Artifact → RELEASED_AS → Release.
// The traversal is undirected and typed; Subgraph.Truncated is surfaced in the result.
func BlastRadius(ctx context.Context, graph ports.GraphStore, tenant ports.TenantID, purl string) (BlastRadiusResult, error) {
	// Validate purl is non-empty and looks like a URL-encoded purl.
	purl = strings.TrimSpace(purl)
	if purl == "" {
		return BlastRadiusResult{}, ErrInvalidPurlForBlast
	}
	// Try to decode URL encoding - if it fails, use as-is (may be plain).
	if _, err := url.QueryUnescape(purl); err != nil {
		return BlastRadiusResult{}, ErrInvalidPurlForBlast
	}

	// Check the component exists.
	_, err := graph.GetNode(ctx, tenant, purl)
	if err != nil {
		if errors.Is(err, ports.ErrNodeNotFound) {
			return BlastRadiusResult{}, ErrInvalidPurlForBlast
		}
		return BlastRadiusResult{}, err
	}

	// Traverse: component → CONTAINS⁻¹ → SBOM → HAS_SBOM⁻¹ → Artifact → RELEASED_AS → Release.
	// Using undirected traversal with typed filters.
	// We need to walk up: from component through CONTAINS to SBOM, then HAS_SBOM to Artifact,
	// then RELEASED_AS to Release.
	//
	// The memstore Traversal is undirected BFS. Starting from the component purl,
	// we walk through CONTAINS (from SBOM side), HAS_SBOM (from SBOM side), and RELEASED_AS edges.
	// MaxDepth=4: component(0) → SBOM(1) → Artifact(2) → Release(3).
	maxNodes := MaxBlastRadiusNodes
	maxEdges := MaxBlastRadiusEdges
	if v, ok := ctx.Value(blastRadiusMaxNodesKey).(int); ok {
		maxNodes = v
	}
	if v, ok := ctx.Value(blastRadiusMaxEdgesKey).(int); ok {
		maxEdges = v
	}
	sub, err := graph.Traversal(ctx, ports.TraversalQuery{
		TenantID:  tenant,
		Roots:     []string{purl},
		EdgeTypes: []string{domainsupplychain.RelationCONTAINS, domainsupplychain.RelationHAS_SBOM, "RELEASED_AS"},
		Kinds:     []string{domainsupplychain.KindSBOM, projection.KindArtifact, projection.KindRelease},
		MaxDepth:  4,
		MaxNodes:  maxNodes,
		MaxEdges:  maxEdges,
	})
	if err != nil {
		return BlastRadiusResult{}, err
	}

	// Collect unique releases from the result.
	releaseIDs := []string{}
	seenRelease := map[string]bool{}
	for _, n := range sub.Nodes {
		if n.Kind == "Release" && n.ID != purl && !seenRelease[n.ID] {
			seenRelease[n.ID] = true
			releaseIDs = append(releaseIDs, n.ID)
		}
	}

	// Build hits with optional name attribute.
	hits := make([]BlastRadiusHit, 0, len(releaseIDs))
	for _, rid := range releaseIDs {
		hit := BlastRadiusHit{ReleaseID: rid}
		if n, err := graph.GetNode(ctx, tenant, rid); err == nil {
			if name, ok := n.Attributes["name"].(string); ok {
				hit.Name = name
			}
		}
		hits = append(hits, hit)
	}

	return BlastRadiusResult{
		Component: purl,
		Releases:  hits,
		Truncated: sub.Truncated,
	}, nil
}
