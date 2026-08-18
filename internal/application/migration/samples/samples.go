// Package samples provides deterministic sampling utilities for the migration
// harness. It generates reproducible node ID and root samples from a fixed seed
// so that shadow reads are deterministic across source and target graphs.
package samples

import (
	"hash/fnv"
	"sort"

	"github.com/Rubentxu/golem/internal/ports"
)

// Sampler generates deterministic samples for migration diff.
type Sampler struct {
	seed uint64
}

// NewSampler creates a Sampler with the given seed (from manifest).
func NewSampler(seed uint64) *Sampler {
	return &Sampler{seed: seed}
}

// SampleNodeIDs returns up to n node IDs sampled deterministically from the graph.
// Uses FNV hash of (seed + index) to select from sorted node IDs.
func (s *Sampler) SampleNodeIDs(nodes []ports.Node, n int) []string {
	if len(nodes) == 0 {
		return nil
	}
	if n <= 0 {
		return nil
	}
	// Sort nodes by ID for deterministic ordering.
	sorted := make([]ports.Node, len(nodes))
	copy(sorted, nodes)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].ID < sorted[j].ID })

	count := n
	if count > len(sorted) {
		count = len(sorted)
	}
	result := make([]string, 0, count)
	// Use FNV to deterministically pick indices.
	h := fnv.New64a()
	h.Write(uint64ToBytes(s.seed))
	for i := 0; i < count; i++ {
		h.Reset()
		h.Write(uint64ToBytes(s.seed + uint64(i)))
		hash := h.Sum64()
		idx := int(hash % uint64(len(sorted)))
		result = append(result, sorted[idx].ID)
	}
	// Deduplicate while preserving first-seen order.
	seen := map[string]bool{}
	deduped := make([]string, 0, len(result))
	for _, id := range result {
		if !seen[id] {
			seen[id] = true
			deduped = append(deduped, id)
		}
	}
	return deduped
}

// SampleTraversalRoots returns up to n root IDs for traversal queries.
func (s *Sampler) SampleTraversalRoots(nodes []ports.Node, n int) []string {
	if len(nodes) == 0 {
		return nil
	}
	if n <= 0 {
		return nil
	}
	count := n
	if count > len(nodes) {
		count = len(nodes)
	}
	result := make([]string, 0, count)
	h := fnv.New64a()
	for i := 0; i < count; i++ {
		h.Reset()
		h.Write(uint64ToBytes(s.seed + uint64(i+1000))) // offset to differentiate from node samples
		hash := h.Sum64()
		idx := int(hash % uint64(len(nodes)))
		result = append(result, nodes[idx].ID)
	}
	seen := map[string]bool{}
	deduped := make([]string, 0, len(result))
	for _, id := range result {
		if !seen[id] {
			seen[id] = true
			deduped = append(deduped, id)
		}
	}
	return deduped
}

func uint64ToBytes(v uint64) []byte {
	b := make([]byte, 8)
	b[0] = byte(v >> 56)
	b[1] = byte(v >> 48)
	b[2] = byte(v >> 40)
	b[3] = byte(v >> 32)
	b[4] = byte(v >> 24)
	b[5] = byte(v >> 16)
	b[6] = byte(v >> 8)
	b[7] = byte(v)
	return b
}
