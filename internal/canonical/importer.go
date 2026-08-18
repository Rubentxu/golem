package canonical

import (
	"archive/tar"
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/Rubentxu/golem/internal/ports"
)

// Importer replays a canonical export archive into a GraphStore and JournalStore.
// It verifies the manifest signature before replay (REQ-AUDIT-004, AC-14).
// Importer can operate in two modes:
//   - DryRun: validates the archive without writing to the journal.
//   - Full: replay into the journal (requires empty journal).
type Importer struct {
	TenantID ports.TenantID
	Graph    ports.GraphStore
	Journal  ports.JournalStore
	Signer   SignatureVerifier
}

// SignatureVerifier verifies a KMS signature over the manifest bytes.
type SignatureVerifier interface {
	Verify(ctx context.Context, payload []byte, sig string) error
}

// ImporterOpts controls the import behaviour.
type ImporterOpts struct {
	// DryRun validates the archive without writing to the journal.
	DryRun bool
	// VerifySignature, when true, verifies the KMS signature before replay.
	// When false, the import skips signature verification (for unsigned v1 archives).
	VerifySignature bool
}

// Import reads a canonical export from an io.Reader (already a tar) and
// optionally verifies the manifest signature and replays into the journal.
func (i *Importer) Import(ctx context.Context, r io.Reader, opts ImporterOpts) error {
	tr := tar.NewReader(r)

	// Read manifest.
	m, _, err := readManifestRaw(tr)
	if err != nil {
		return fmt.Errorf("import: read manifest: %w", err)
	}

	// Validate format version.
	if err := m.Validate(); err != nil {
		return fmt.Errorf("import: %w", err)
	}

	// Verify signature if enabled and manifest is v2+.
	if opts.VerifySignature && m.SignedBlock != nil {
		if i.Signer == nil {
			return fmt.Errorf("import: signer required for v2 manifest verification")
		}
		signable, err := m.SignedPayload()
		if err != nil {
			return fmt.Errorf("import: signable payload: %w", err)
		}
		if err := i.Signer.Verify(ctx, signable, m.SignedBlock.Signature); err != nil {
			return fmt.Errorf("import: signature verification failed: %w", err)
		}
	}

	if opts.DryRun {
		return nil // validated successfully
	}

	// Check journal is empty before replay.
	head, err := i.Journal.Head(ctx)
	if err != nil {
		return fmt.Errorf("import: journal.Head: %w", err)
	}
	if head > 0 {
		return fmt.Errorf("import: journal not empty (head=%d); restore requires empty journal", head)
	}

	// Replay nodes and edges.
	if err := i.replayNodes(ctx, tr, m.Counts.Nodes); err != nil {
		return fmt.Errorf("import: replay nodes: %w", err)
	}
	if err := i.replayEdges(ctx, tr, m.Counts.Edges); err != nil {
		return fmt.Errorf("import: replay edges: %w", err)
	}

	return nil
}

// readManifestRaw reads the manifest.json from the tar and returns it along
// with the raw bytes for signature verification.
func readManifestRaw(tr *tar.Reader) (*Manifest, []byte, error) {
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil, nil, fmt.Errorf("manifest.json not found in archive")
		}
		if err != nil {
			return nil, nil, fmt.Errorf("tar.Next: %w", err)
		}
		if hdr.Name == "manifest.json" {
			data, err := io.ReadAll(tr)
			if err != nil {
				return nil, nil, fmt.Errorf("read manifest: %w", err)
			}
			var m Manifest
			if err := json.Unmarshal(data, &m); err != nil {
				return nil, nil, fmt.Errorf("unmarshal manifest: %w", err)
			}
			return &m, data, nil
		}
	}
}

// replayNodes reads nodes.jsonl from the tar and applies them to the graph.
func (i *Importer) replayNodes(ctx context.Context, tr *tar.Reader, count uint64) error {
	nodesData, err := extractJSONL(tr, "nodes.jsonl", count)
	if err != nil {
		return err
	}
	for _, n := range nodesData {
		ops := []ports.GraphOp{{
			Kind:   ports.OpUpsertNode,
			Target: n["id"].(string),
			Data:   n,
		}}
		if _, err := i.Graph.Apply(ctx, ports.GraphMutation{
			TenantID: i.TenantID,
			Ops:      ops,
		}); err != nil {
			return fmt.Errorf("apply node %v: %w", n["id"], err)
		}
	}
	return nil
}

// replayEdges reads edges.jsonl from the tar and applies them to the graph.
func (i *Importer) replayEdges(ctx context.Context, tr *tar.Reader, count uint64) error {
	edgesData, err := extractJSONL(tr, "edges.jsonl", count)
	if err != nil {
		return err
	}
	for _, e := range edgesData {
		ops := []ports.GraphOp{{
			Kind:   ports.OpUpsertEdge,
			Target: e["id"].(string),
			Data:   e,
		}}
		if _, err := i.Graph.Apply(ctx, ports.GraphMutation{
			TenantID: i.TenantID,
			Ops:      ops,
		}); err != nil {
			return fmt.Errorf("apply edge %v: %w", e["id"], err)
		}
	}
	return nil
}

// extractJSONL reads a JSONL file from the tar and returns the decoded objects.
func extractJSONL(tr *tar.Reader, name string, count uint64) ([]map[string]any, error) {
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
			var result []map[string]any
			lines := splitJSONLines(data)
			for _, line := range lines {
				if len(line) == 0 {
					continue
				}
				var obj map[string]any
				if err := json.Unmarshal(line, &obj); err != nil {
					return nil, fmt.Errorf("parse %s line: %w", name, err)
				}
				result = append(result, obj)
			}
			return result, nil
		}
	}
}
