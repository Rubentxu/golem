package canonical

import (
	"archive/tar"
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/Rubentxu/golem/internal/ports"
)

// Exporter produces a canonical export tar archive from a GraphStore and JournalStore.
type Exporter struct {
	TenantID ports.TenantID
	Graph    ports.GraphStore
	Journal  ports.JournalStore
	Out      io.Writer
}

// canonicalNode is the JSON representation of a node in the canonical export.
// Uses lowercase keys per ports.GraphOp canonical data specification.
type canonicalNode struct {
	ID         string         `json:"id"`
	Kind       string         `json:"kind"`
	Revision   uint64         `json:"revision"`
	Attributes map[string]any `json:"attributes"`
}

// canonicalEdge is the JSON representation of an edge in the canonical export.
type canonicalEdge struct {
	ID         string         `json:"id"`
	Type       string         `json:"type"`
	SourceID   string         `json:"source"`
	TargetID   string         `json:"target"`
	Revision   uint64         `json:"revision"`
	Attributes map[string]any `json:"attributes"`
}

// Export writes the canonical export to e.Out.
// It iterates over the graph (nodes and edges) and the journal head position,
// produces a tar archive with JSONL files and a manifest.
func (e *Exporter) Export(ctx context.Context) (Manifest, error) {
	manifest := NewManifest(string(e.TenantID), 0)

	// Get journal head.
	if e.Journal != nil {
		head, err := e.Journal.Head(ctx)
		if err != nil {
			return Manifest{}, fmt.Errorf("journal.Head: %w", err)
		}
		manifest.JournalPosition.Head = uint64(head)
	}

	// Collect nodes and edges.
	nodes, err := e.collectNodes(ctx)
	if err != nil {
		return Manifest{}, fmt.Errorf("collect nodes: %w", err)
	}
	edges, err := e.collectEdges(ctx)
	if err != nil {
		return Manifest{}, fmt.Errorf("collect edges: %w", err)
	}
	manifest.SetCounts(uint64(len(nodes)), uint64(len(edges)))

	// Pre-compute all digests before writing any files.
	nodesData, nodesDigest, err := encodeJSONL(nodes)
	if err != nil {
		return Manifest{}, fmt.Errorf("nodes jsonl: %w", err)
	}
	manifest.SetFileDigest("nodes.jsonl", nodesDigest)

	edgesData, edgesDigest, err := encodeJSONL(edges)
	if err != nil {
		return Manifest{}, fmt.Errorf("edges jsonl: %w", err)
	}
	manifest.SetFileDigest("edges.jsonl", edgesDigest)

	jpData, err := json.Marshal(manifest.JournalPosition)
	if err != nil {
		return Manifest{}, fmt.Errorf("marshal journal position: %w", err)
	}
	jpDigest, err := ComputeSHA256(bytesReader(jpData))
	if err != nil {
		return Manifest{}, fmt.Errorf("journal-position sha256: %w", err)
	}
	manifest.SetFileDigest("journal-position.json", jpDigest)

	ontologyData := []byte(OntologySchemaJSON)
	ontologyDigest, err := ComputeSHA256(bytesReader(ontologyData))
	if err != nil {
		return Manifest{}, fmt.Errorf("ontology sha256: %w", err)
	}
	manifest.SetFileDigest("ontology.schema.json", ontologyDigest)

	tw := tar.NewWriter(e.Out)
	defer tw.Close()

	// Write manifest.json first (so ReadFromReader can read it before other files).
	manifestData, err := json.Marshal(manifest)
	if err != nil {
		return Manifest{}, fmt.Errorf("marshal manifest: %w", err)
	}
	if err := writeTarFile(tw, "manifest.json", bytesReader(manifestData)); err != nil {
		return Manifest{}, fmt.Errorf("tar manifest: %w", err)
	}

	// Write nodes.jsonl.
	if err := writeTarFile(tw, "nodes.jsonl", bytesReader(nodesData)); err != nil {
		return Manifest{}, fmt.Errorf("tar nodes: %w", err)
	}

	// Write edges.jsonl.
	if err := writeTarFile(tw, "edges.jsonl", bytesReader(edgesData)); err != nil {
		return Manifest{}, fmt.Errorf("tar edges: %w", err)
	}

	// Write journal-position.json.
	if err := writeTarFile(tw, "journal-position.json", bytesReader(jpData)); err != nil {
		return Manifest{}, fmt.Errorf("tar journal-position: %w", err)
	}

	// Write ontology.schema.json.
	if err := writeTarFile(tw, "ontology.schema.json", bytesReader(ontologyData)); err != nil {
		return Manifest{}, fmt.Errorf("tar ontology: %w", err)
	}

	return manifest, nil
}

// collectNodes enumerates all nodes for the tenant using ListNodes and converts to canonical format.
func (e *Exporter) collectNodes(ctx context.Context) ([]canonicalNode, error) {
	nodes, err := e.Graph.ListNodes(ctx, e.TenantID)
	if err != nil {
		return nil, err
	}
	result := make([]canonicalNode, 0, len(nodes))
	for _, n := range nodes {
		result = append(result, canonicalNode{
			ID:         n.ID,
			Kind:       n.Kind,
			Revision:   uint64(n.Revision),
			Attributes: n.Attributes,
		})
	}
	return result, nil
}

// collectEdges enumerates all edges for the tenant using ListEdges and converts to canonical format.
func (e *Exporter) collectEdges(ctx context.Context) ([]canonicalEdge, error) {
	edges, err := e.Graph.ListEdges(ctx, e.TenantID)
	if err != nil {
		return nil, err
	}
	result := make([]canonicalEdge, 0, len(edges))
	for _, e := range edges {
		result = append(result, canonicalEdge{
			ID:         e.ID,
			Type:       e.Type,
			SourceID:   e.SourceID,
			TargetID:   e.TargetID,
			Revision:   uint64(e.Revision),
			Attributes: e.Attributes,
		})
	}
	return result, nil
}

// encodeJSONL encodes items as newline-delimited JSON.
func encodeJSONL[T any](items []T) ([]byte, string, error) {
	buf := make([]byte, 0, len(items)*128)
	enc := json.NewEncoder(bytesWriter{buf: &buf})
	for _, item := range items {
		if err := enc.Encode(item); err != nil {
			return nil, "", err
		}
	}
	digest, err := ComputeSHA256(bytesReader(buf))
	if err != nil {
		return nil, "", err
	}
	return buf, digest, nil
}

// writeTarFile writes data to a tar file entry.
func writeTarFile(tw *tar.Writer, name string, r io.Reader) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	hdr := &tar.Header{
		Name: name,
		Mode: 0644,
		Size: int64(len(data)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	_, err = tw.Write(data)
	return err
}

// bytesReader returns an io.Reader from a byte slice.
func bytesReader(b []byte) io.Reader {
	return &bytesReaderImpl{b: b, i: 0}
}

type bytesReaderImpl struct {
	b []byte
	i int
}

func (r *bytesReaderImpl) Read(p []byte) (n int, err error) {
	if r.i >= len(r.b) {
		return 0, io.EOF
	}
	n = copy(p, r.b[r.i:])
	r.i += n
	return n, nil
}

// bytesWriter collects bytes.
type bytesWriter struct {
	buf *[]byte
}

func (w bytesWriter) Write(p []byte) (n int, err error) {
	*w.buf = append(*w.buf, p...)
	return len(p), nil
}
