package canonical

import (
	"archive/tar"
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/Rubentxu/golem/internal/ports"
)

const maxOpsPerMutation = 500 // Same chunking as internal/application/projection/projector.go

// Reader deserializes a canonical export tar archive and applies its
// contents to a GraphStore via GraphStore.Apply.
type Reader struct {
	TenantID ports.TenantID
	Graph    ports.GraphStore
}

// ReadFromReader reads the canonical export from an io.Reader (already a tar).
// Caller provides the tar.Reader positioned at the start.
func (r *Reader) ReadFromReader(ctx context.Context, tr *tar.Reader) error {
	// Read manifest first.
	manifest, err := r.readManifest(tr)
	if err != nil {
		return fmt.Errorf("read manifest: %w", err)
	}
	if err := manifest.Validate(); err != nil {
		return err
	}

	// Reset tar to start (tr doesn't support seeking, but we already read manifest).
	// Actually we can't reset. We need to track files as we read them.

	// Read and apply nodes.
	nodes, err := r.extractJSONL(tr, "nodes.jsonl", manifest.Counts.Nodes)
	if err != nil {
		return fmt.Errorf("extract nodes: %w", err)
	}

	// Apply nodes in batches.
	if err := r.applyNodes(ctx, nodes); err != nil {
		return fmt.Errorf("apply nodes: %w", err)
	}

	// Read and apply edges.
	edges, err := r.extractJSONL(tr, "edges.jsonl", manifest.Counts.Edges)
	if err != nil {
		return fmt.Errorf("extract edges: %w", err)
	}

	if err := r.applyEdges(ctx, edges); err != nil {
		return fmt.Errorf("apply edges: %w", err)
	}

	return nil
}

// readManifest reads and parses the manifest.json from the tar.
func (r *Reader) readManifest(tr *tar.Reader) (*Manifest, error) {
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil, fmt.Errorf("manifest.json not found in archive")
		}
		if err != nil {
			return nil, fmt.Errorf("tar.Next: %w", err)
		}
		if hdr.Name == "manifest.json" {
			data, err := io.ReadAll(tr)
			if err != nil {
				return nil, fmt.Errorf("read manifest: %w", err)
			}
			var m Manifest
			if err := json.Unmarshal(data, &m); err != nil {
				return nil, fmt.Errorf("unmarshal manifest: %w", err)
			}
			return &m, nil
		}
	}
}

// extractJSONL reads a JSONL file from the tar and decodes it.
func (r *Reader) extractJSONL(tr *tar.Reader, name string, count uint64) ([]map[string]any, error) {
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil, fmt.Errorf("%s not found in archive", name)
		}
		if err != nil {
			return nil, fmt.Errorf("tar.Next: %w", err)
		}
		if hdr.Name == name {
			data, err := io.ReadAll(tr)
			if err != nil {
				return nil, fmt.Errorf("read %s: %w", name, err)
			}

			// Decode JSONL: each line is a JSON object.
			lines := splitJSONLines(data)
			results := make([]map[string]any, 0, len(lines))
			for _, line := range lines {
				if len(line) == 0 {
					continue
				}
				var obj map[string]any
				if err := json.Unmarshal(line, &obj); err != nil {
					return nil, fmt.Errorf("parse %s line: %w", name, err)
				}
				results = append(results, obj)
			}
			return results, nil
		}
	}
}

// splitJSONLines splits JSONL data into lines.
func splitJSONLines(data []byte) [][]byte {
	var lines [][]byte
	start := 0
	for i := 0; i < len(data); i++ {
		if data[i] == '\n' {
			if start < i {
				lines = append(lines, data[start:i])
			}
			start = i + 1
		}
	}
	if start < len(data) {
		lines = append(lines, data[start:])
	}
	return lines
}

// applyNodes applies node mutations to the graph in batches.
func (r *Reader) applyNodes(ctx context.Context, nodes []map[string]any) error {
	for i := 0; i < len(nodes); i += maxOpsPerMutation {
		end := i + maxOpsPerMutation
		if end > len(nodes) {
			end = len(nodes)
		}
		batch := nodes[i:end]
		ops := make([]ports.GraphOp, 0, len(batch))
		for _, n := range batch {
			ops = append(ops, ports.GraphOp{
				Kind:   ports.OpUpsertNode,
				Target: n["id"].(string),
				Data:   n,
			})
		}
		_, err := r.Graph.Apply(ctx, ports.GraphMutation{
			TenantID: r.TenantID,
			Ops:      ops,
		})
		if err != nil {
			return fmt.Errorf("apply nodes batch: %w", err)
		}
	}
	return nil
}

// applyEdges applies edge mutations to the graph in batches.
func (r *Reader) applyEdges(ctx context.Context, edges []map[string]any) error {
	for i := 0; i < len(edges); i += maxOpsPerMutation {
		end := i + maxOpsPerMutation
		if end > len(edges) {
			end = len(edges)
		}
		batch := edges[i:end]
		ops := make([]ports.GraphOp, 0, len(batch))
		for _, e := range batch {
			ops = append(ops, ports.GraphOp{
				Kind:   ports.OpUpsertEdge,
				Target: e["id"].(string),
				Data:   e,
			})
		}
		_, err := r.Graph.Apply(ctx, ports.GraphMutation{
			TenantID: r.TenantID,
			Ops:      ops,
		})
		if err != nil {
			return fmt.Errorf("apply edges batch: %w", err)
		}
	}
	return nil
}

// ParseCanonicalNodes parses JSONL canonical nodes from raw bytes.
func ParseCanonicalNodes(data []byte) ([]CanonicalNode, error) {
	var result []CanonicalNode
	lines := splitJSONLines(data)
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		var n CanonicalNode
		if err := json.Unmarshal(line, &n); err != nil {
			return nil, err
		}
		result = append(result, n)
	}
	return result, nil
}

// ParseCanonicalEdges parses JSONL canonical edges from raw bytes.
func ParseCanonicalEdges(data []byte) ([]CanonicalEdge, error) {
	var result []CanonicalEdge
	lines := splitJSONLines(data)
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		var e CanonicalEdge
		if err := json.Unmarshal(line, &e); err != nil {
			return nil, err
		}
		result = append(result, e)
	}
	return result, nil
}
