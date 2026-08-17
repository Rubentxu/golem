package projection

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"

	"github.com/Rubentxu/golem/internal/ports"
)

// Digest computes the canonical SHA-256 digest of a subgraph: nodes and
// edges sorted by ID, serialized as deterministic JSON (encoding/json
// orders map keys), hashed. The M1 exit criterion is that replaying the
// journal into a fresh store yields the same digest — projections are
// disposable and reproducible (ADR-049).
func Digest(s ports.Subgraph) (string, error) {
	nodes := append([]ports.Node(nil), s.Nodes...)
	edges := append([]ports.Edge(nil), s.Edges...)
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].ID < nodes[j].ID })
	sort.Slice(edges, func(i, j int) bool { return edges[i].ID < edges[j].ID })

	b, err := json.Marshal(struct {
		Nodes []ports.Node `json:"nodes"`
		Edges []ports.Edge `json:"edges"`
	}{Nodes: nodes, Edges: edges})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}
