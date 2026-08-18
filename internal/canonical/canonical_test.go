package canonical

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"sort"
	"sync"
	"testing"

	"github.com/Rubentxu/golem/internal/ports"
)

// --- Test graph store stub ---

type testGraphStore struct {
	mu      sync.RWMutex
	tenants map[ports.TenantID]map[string]*ports.Node
	edges   map[ports.TenantID]map[string]*ports.Edge
}

func newTestGraphStore() *testGraphStore {
	return &testGraphStore{
		tenants: make(map[ports.TenantID]map[string]*ports.Node),
		edges:   make(map[ports.TenantID]map[string]*ports.Edge),
	}
}

func (s *testGraphStore) Apply(ctx context.Context, tx ports.GraphMutation) (ports.Revision, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.tenants[tx.TenantID]; !ok {
		s.tenants[tx.TenantID] = make(map[string]*ports.Node)
		s.edges[tx.TenantID] = make(map[string]*ports.Edge)
	}
	g := s.tenants[tx.TenantID]
	e := s.edges[tx.TenantID]
	for _, op := range tx.Ops {
		switch op.Kind {
		case ports.OpUpsertNode:
			n := &ports.Node{
				ID:         op.Target,
				Kind:       op.Data["kind"].(string),
				Attributes: op.Data["attributes"].(map[string]any),
			}
			g[op.Target] = n
		case ports.OpUpsertEdge:
			ed := &ports.Edge{
				ID:         op.Target,
				Type:       op.Data["type"].(string),
				SourceID:   op.Data["source"].(string),
				TargetID:   op.Data["target"].(string),
				Attributes: op.Data["attributes"].(map[string]any),
			}
			e[op.Target] = ed
		}
	}
	return 1, nil
}

func (s *testGraphStore) GetNode(_ context.Context, tenant ports.TenantID, nodeID string) (ports.Node, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if g, ok := s.tenants[tenant]; ok {
		if n, ok := g[nodeID]; ok {
			return *n, nil
		}
	}
	return ports.Node{}, ports.ErrNodeNotFound
}

func (s *testGraphStore) ListNodes(_ context.Context, tenant ports.TenantID) ([]ports.Node, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	g, ok := s.tenants[tenant]
	if !ok {
		return nil, nil
	}
	ids := make([]string, 0, len(g))
	for id := range g {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	result := make([]ports.Node, 0, len(ids))
	for _, id := range ids {
		result = append(result, *g[id])
	}
	return result, nil
}

func (s *testGraphStore) ListEdges(_ context.Context, tenant ports.TenantID) ([]ports.Edge, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	e, ok := s.edges[tenant]
	if !ok {
		return nil, nil
	}
	ids := make([]string, 0, len(e))
	for id := range e {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	result := make([]ports.Edge, 0, len(ids))
	for _, id := range ids {
		result = append(result, *e[id])
	}
	return result, nil
}

func (s *testGraphStore) Neighborhood(_ context.Context, _ ports.NeighborhoodQuery) (ports.Subgraph, error) {
	return ports.Subgraph{}, nil
}

func (s *testGraphStore) Traversal(_ context.Context, _ ports.TraversalQuery) (ports.Subgraph, error) {
	return ports.Subgraph{}, nil
}

func (s *testGraphStore) Capabilities(_ context.Context) ports.GraphCapabilities {
	return ports.GraphCapabilities{Transactions: true, EdgeProperties: true}
}

// --- Test journal store stub ---

type testJournalStore struct{}

func (t *testJournalStore) Append(_ context.Context, _ []ports.RawEvent) ([]ports.AppendResult, error) {
	return nil, nil
}

func (t *testJournalStore) AppendIf(_ context.Context, _ ports.StreamVersion, _ []ports.RawEvent) ([]ports.AppendResult, error) {
	return nil, nil
}

func (t *testJournalStore) ReadStream(_ context.Context, _ ports.TenantID, _ string, _ uint64) ([]ports.RawEvent, error) {
	return nil, nil
}

func (t *testJournalStore) Replay(_ context.Context, _ ports.StreamPosition, _ int) ([]ports.RawEvent, ports.StreamPosition, error) {
	return nil, 0, nil
}

func (t *testJournalStore) Head(_ context.Context) (ports.StreamPosition, error) {
	return 0, nil
}

func (t *testJournalStore) Backup(_ context.Context) (ports.BackupHandle, error) {
	return ports.BackupHandle{}, nil
}

func (t *testJournalStore) Restore(_ context.Context, _ ports.BackupHandle) error {
	return nil
}

// --- Tests ---

// TestCanonicalRoundTrip verifies that Export → import produces equivalent node results.
func TestCanonicalRoundTrip(t *testing.T) {
	ctx := context.Background()
	tenant := ports.TenantID("t1")

	srcGraph := newTestGraphStore()
	srcJournal := &testJournalStore{}

	// Add 10 nodes.
	nodeIDs := make([]string, 10)
	for i := 0; i < 10; i++ {
		nodeIDs[i] = "node-" + itoa(i)
		_, err := srcGraph.Apply(ctx, ports.GraphMutation{
			TenantID: tenant,
			Ops: []ports.GraphOp{
				{
					Kind:   ports.OpUpsertNode,
					Target: nodeIDs[i],
					Data: map[string]any{
						"id":         nodeIDs[i],
						"tenant_id":  tenant,
						"kind":       "Project",
						"revision":   1,
						"attributes": map[string]any{"index": i},
					},
				},
			},
		})
		if err != nil {
			t.Fatalf("apply node: %v", err)
		}
	}

	// Add 15 edges.
	for i := 0; i < 15; i++ {
		src := i % 10
		tgt := (i + 1) % 10
		_, err := srcGraph.Apply(ctx, ports.GraphMutation{
			TenantID: tenant,
			Ops: []ports.GraphOp{
				{
					Kind:   ports.OpUpsertEdge,
					Target: "edge-" + itoa(i),
					Data: map[string]any{
						"id":         "edge-" + itoa(i),
						"tenant_id":  tenant,
						"type":       "DEPENDS_ON",
						"source":     nodeIDs[src],
						"target":     nodeIDs[tgt],
						"revision":   1,
						"attributes": map[string]any{"index": i},
					},
				},
			},
		})
		if err != nil {
			t.Fatalf("apply edge: %v", err)
		}
	}

	var buf bytes.Buffer
	exporter := Exporter{
		TenantID: tenant,
		Graph:    srcGraph,
		Journal:  srcJournal,
		Out:      &buf,
	}
	manifest, err := exporter.Export(ctx)
	if err != nil {
		t.Fatalf("export: %v", err)
	}

	if manifest.FormatVersion != "1" {
		t.Errorf("manifest.FormatVersion = %q, want %q", manifest.FormatVersion, "1")
	}
	if manifest.TenantID != string(tenant) {
		t.Errorf("manifest.TenantID = %q, want %q", manifest.TenantID, tenant)
	}

	for _, path := range []string{"nodes.jsonl", "edges.jsonl", "journal-position.json", "ontology.schema.json"} {
		if _, ok := manifest.Files[path]; !ok {
			t.Errorf("manifest.Files[%q] missing", path)
		}
	}

	tgtGraph := newTestGraphStore()
	tr := tar.NewReader(&buf)
	reader := Reader{
		TenantID: tenant,
		Graph:    tgtGraph,
	}
	if err := reader.ReadFromReader(ctx, tr, ReaderOpts{}); err != nil {
		t.Fatalf("read from reader: %v", err)
	}

	node, err := tgtGraph.GetNode(ctx, tenant, nodeIDs[0])
	if err != nil {
		t.Errorf("GetNode(%s): %v", nodeIDs[0], err)
	}
	if node.ID != nodeIDs[0] {
		t.Errorf("node.ID = %q, want %q", node.ID, nodeIDs[0])
	}
}

// TestCanonicalEmptyExport verifies that exporting an empty graph produces a valid manifest.
func TestCanonicalEmptyExport(t *testing.T) {
	ctx := context.Background()
	tenant := ports.TenantID("t1")

	graph := newTestGraphStore()
	journal := &testJournalStore{}

	var buf bytes.Buffer
	exporter := Exporter{
		TenantID: tenant,
		Graph:    graph,
		Journal:  journal,
		Out:      &buf,
	}
	manifest, err := exporter.Export(ctx)
	if err != nil {
		t.Fatalf("export: %v", err)
	}

	if manifest.FormatVersion != "1" {
		t.Errorf("FormatVersion = %q, want %q", manifest.FormatVersion, "1")
	}
	if manifest.Counts.Nodes != 0 {
		t.Errorf("Counts.Nodes = %d, want 0", manifest.Counts.Nodes)
	}
	if manifest.Counts.Edges != 0 {
		t.Errorf("Counts.Edges = %d, want 0", manifest.Counts.Edges)
	}

	tr := tar.NewReader(&buf)
	found := map[string]bool{
		"nodes.jsonl":           false,
		"edges.jsonl":           false,
		"journal-position.json": false,
		"ontology.schema.json":  false,
		"manifest.json":         false,
	}
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("tar.Next: %v", err)
		}
		if _, ok := found[hdr.Name]; ok {
			found[hdr.Name] = true
		}
	}
	for name, ok := range found {
		if !ok {
			t.Errorf("archive missing file: %s", name)
		}
	}
}

// TestCanonicalUnsupportedFormatVersion verifies that a manifest with format_version "99" is rejected.
func TestCanonicalUnsupportedFormatVersion(t *testing.T) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)

	manifest := Manifest{
		FormatVersion: "99",
		TenantID:      "t1",
		CreatedAt:     "2026-08-18T10:00:00Z",
		Counts:        Counts{Nodes: 0, Edges: 0},
		Files:         map[string]FileDigest{},
	}
	data, _ := json.Marshal(manifest)
	hdr := &tar.Header{
		Name: "manifest.json",
		Mode: 0644,
		Size: int64(len(data)),
	}
	tw.WriteHeader(hdr)
	tw.Write(data)
	tw.Close()

	// Extract manifest.json from tar using tr.Next().
	tr := tar.NewReader(&buf)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			t.Fatalf("manifest.json not found in tar")
		}
		if err != nil {
			t.Fatalf("tar.Next: %v", err)
		}
		if hdr.Name == "manifest.json" {
			extracted, err := io.ReadAll(tr)
			if err != nil {
				t.Fatalf("io.ReadAll: %v", err)
			}
			var manifestFromTar Manifest
			if err := json.Unmarshal(extracted, &manifestFromTar); err != nil {
				t.Fatalf("json.Unmarshal: %v", err)
			}

			err = manifestFromTar.Validate()
			if err == nil {
				t.Fatalf("Validate() = nil, want ErrUnsupportedFormatVersion")
			}
			return
		}
	}
}

// TestCanonicalSHA256Matches verifies that the SHA-256 of nodes.jsonl matches.
func TestCanonicalSHA256Matches(t *testing.T) {
	ctx := context.Background()
	tenant := ports.TenantID("t1")

	graph := newTestGraphStore()
	journal := &testJournalStore{}

	_, err := graph.Apply(ctx, ports.GraphMutation{
		TenantID: tenant,
		Ops: []ports.GraphOp{
			{
				Kind:   ports.OpUpsertNode,
				Target: "n1",
				Data: map[string]any{
					"id": "n1", "tenant_id": tenant, "kind": "Project", "revision": 1, "attributes": map[string]any{},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}

	var buf bytes.Buffer
	exporter := Exporter{
		TenantID: tenant,
		Graph:    graph,
		Journal:  journal,
		Out:      &buf,
	}
	manifest, err := exporter.Export(ctx)
	if err != nil {
		t.Fatalf("export: %v", err)
	}

	tr := tar.NewReader(&buf)
	var nodesData []byte
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if hdr.Name == "nodes.jsonl" {
			nodesData, _ = io.ReadAll(tr)
			break
		}
	}

	if len(nodesData) == 0 {
		t.Fatal("nodes.jsonl not found in archive")
	}

	digest, err := ComputeSHA256(bytes.NewReader(nodesData))
	if err != nil {
		t.Fatalf("ComputeSHA256: %v", err)
	}

	if digest != manifest.Files["nodes.jsonl"].SHA256 {
		t.Errorf("SHA256 mismatch: computed=%s, manifest=%s", digest, manifest.Files["nodes.jsonl"].SHA256)
	}
}

// itoa converts an integer to a decimal string.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
