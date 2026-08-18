package tck_test

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"testing"

	graphmem "github.com/Rubentxu/golem/adapters/graph/memstore"
	journalmem "github.com/Rubentxu/golem/adapters/journal/memstore"
	"github.com/Rubentxu/golem/internal/canonical"
	"github.com/Rubentxu/golem/internal/ports"
)

// --- Signed manifest TCK tests (W5.23) ---

// TestCanonicalExportV2Manifest checks that a v2 export produces FormatVersion "2".
func TestCanonicalExportV2Manifest(t *testing.T) {
	ctx := context.Background()
	tenant := ports.TenantID("t-canonical-v2")

	graph := graphmem.NewGraph()
	journal := journalmem.NewJournal()

	// Upsert a node so the export has content.
	_, _ = graph.Apply(ctx, ports.GraphMutation{
		TenantID: tenant,
		Ops: []ports.GraphOp{{
			Kind:   ports.OpUpsertNode,
			Target: "n1",
			Data:   map[string]any{"id": "n1", "kind": "TestNode", "revision": 1, "attributes": map[string]any{"name": "test"}},
		}},
	})

	var buf bytes.Buffer
	exporter := canonical.Exporter{
		TenantID: tenant,
		Graph:    graph,
		Journal:  journal,
		Out:      &buf,
	}
	manifest, err := exporter.Export(ctx)
	if err != nil {
		t.Fatalf("export: %v", err)
	}

	if manifest.FormatVersion != "2" {
		t.Errorf("FormatVersion = %q, want \"2\"", manifest.FormatVersion)
	}
	// SignedBlock is nil on the raw export — signing is done by the worker cron.
	if manifest.SignedBlock != nil {
		t.Errorf("manifest.SignedBlock = %+v, want nil", manifest.SignedBlock)
	}
}

// buildTestTar creates a tar archive with a manifest and empty content files.
func buildTestTar(tenant ports.TenantID, m canonical.Manifest, sig string) bytes.Buffer {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	mData, _ := json.Marshal(m)
	tw.WriteHeader(&tar.Header{Name: "manifest.json", Mode: 0644, Size: int64(len(mData))})
	tw.Write(mData)
	tw.WriteHeader(&tar.Header{Name: "nodes.jsonl", Mode: 0644, Size: 0})
	tw.WriteHeader(&tar.Header{Name: "edges.jsonl", Mode: 0644, Size: 0})
	tw.WriteHeader(&tar.Header{Name: "journal-position.json", Mode: 0644, Size: 0})
	tw.WriteHeader(&tar.Header{Name: "ontology.schema.json", Mode: 0644, Size: 0})
	tw.Close()
	return buf
}

// TestCanonicalImportV1Manifest verifies that a v1 manifest (unsigned) is accepted
// with VerifySignature=false.
func TestCanonicalImportV1Manifest(t *testing.T) {
	ctx := context.Background()
	tenant := ports.TenantID("t-import-v1")

	graph := graphmem.NewGraph()
	journal := journalmem.NewJournal()

	m := canonical.Manifest{
		FormatVersion: "1",
		TenantID:      string(tenant),
		CreatedAt:     "2026-08-19T00:00:00Z",
		Counts:        canonical.Counts{Nodes: 0, Edges: 0},
		Files:         map[string]canonical.FileDigest{},
	}
	buf := buildTestTar(tenant, m, "")

	importer := canonical.Importer{
		TenantID: tenant,
		Graph:    graph,
		Journal:  journal,
	}
	if err := importer.Import(ctx, &buf, canonical.ImporterOpts{VerifySignature: false}); err != nil {
		t.Fatalf("import v1: %v", err)
	}
}

// TestCanonicalImportV2ManifestNoSignature verifies that a v2 manifest with no
// SignedBlock is accepted even with VerifySignature=true.
func TestCanonicalImportV2ManifestNoSignature(t *testing.T) {
	ctx := context.Background()
	tenant := ports.TenantID("t-import-v2-nosig")

	graph := graphmem.NewGraph()
	journal := journalmem.NewJournal()

	m := canonical.Manifest{
		FormatVersion: "2",
		TenantID:      string(tenant),
		CreatedAt:     "2026-08-19T00:00:00Z",
		Counts:        canonical.Counts{Nodes: 0, Edges: 0},
		Files:         map[string]canonical.FileDigest{},
		SignedBlock:   nil,
	}
	buf := buildTestTar(tenant, m, "")

	importer := canonical.Importer{
		TenantID: tenant,
		Graph:    graph,
		Journal:  journal,
	}
	if err := importer.Import(ctx, &buf, canonical.ImporterOpts{VerifySignature: true}); err != nil {
		t.Fatalf("import v2 no-sig: %v", err)
	}
}

// localSigner is a test Signer that uses RSA-PKCS1v15 to mimic KMS signing.
// It pre-hashes the payload before signing (matching AWS SDK v2 behavior).
type localSigner struct {
	key *rsa.PrivateKey
}

func newLocalSigner() (*localSigner, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}
	return &localSigner{key: key}, nil
}

func (s *localSigner) Sign(ctx context.Context, payload []byte) (string, error) {
	h := crypto.SHA256.New()
	h.Write(payload)
	hashed := h.Sum(nil)
	sig, err := rsa.SignPKCS1v15(rand.Reader, s.key, crypto.SHA256, hashed)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(sig), nil
}

func (s *localSigner) Verify(ctx context.Context, payload []byte, sigHex string) error {
	sigBytes, err := hex.DecodeString(sigHex)
	if err != nil {
		return fmt.Errorf("hex decode: %w", err)
	}
	h := crypto.SHA256.New()
	h.Write(payload)
	return rsa.VerifyPKCS1v15(&s.key.PublicKey, crypto.SHA256, h.Sum(nil), sigBytes)
}

// TestCanonicalImportSignedV2Manifest verifies that a v2 manifest with a valid
// signature is accepted.
func TestCanonicalImportSignedV2Manifest(t *testing.T) {
	ctx := context.Background()
	tenant := ports.TenantID("t-import-signed")

	graph := graphmem.NewGraph()
	journal := journalmem.NewJournal()

	signer, err := newLocalSigner()
	if err != nil {
		t.Fatalf("newLocalSigner: %v", err)
	}

	// Create manifest and sign its exact SignedPayload bytes.
	m := canonical.Manifest{
		FormatVersion: "2",
		TenantID:      string(tenant),
		CreatedAt:     "2026-08-19T00:00:00Z",
		Counts:        canonical.Counts{Nodes: 0, Edges: 0},
		Files:         map[string]canonical.FileDigest{},
	}
	signable, err := (&m).SignedPayload()
	if err != nil {
		t.Fatalf("SignedPayload: %v", err)
	}
	sig, err := signer.Sign(ctx, signable)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	m.SignedBlock = &canonical.SignedBlock{KeyID: "alias/test-key", Signature: sig}

	buf := buildTestTar(tenant, m, sig)

	importer := canonical.Importer{
		TenantID: tenant,
		Graph:    graph,
		Journal:  journal,
		Signer:   signer,
	}
	if err := importer.Import(ctx, &buf, canonical.ImporterOpts{VerifySignature: true}); err != nil {
		t.Fatalf("import signed v2: %v", err)
	}
}

// TestCanonicalImportSignedV2ManifestTampered verifies that a tampered v2
// manifest is rejected by signature verification.
func TestCanonicalImportSignedV2ManifestTampered(t *testing.T) {
	ctx := context.Background()
	tenant := ports.TenantID("t-import-tampered")

	graph := graphmem.NewGraph()
	journal := journalmem.NewJournal()

	signer, err := newLocalSigner()
	if err != nil {
		t.Fatalf("newLocalSigner: %v", err)
	}

	// Sign a manifest with the correct tenant.
	m := canonical.Manifest{
		FormatVersion: "2",
		TenantID:      string(tenant),
		CreatedAt:     "2026-08-19T00:00:00Z",
		Counts:        canonical.Counts{Nodes: 0, Edges: 0},
		Files:         map[string]canonical.FileDigest{},
	}
	signable, _ := (&m).SignedPayload()
	sig, _ := signer.Sign(ctx, signable)

	// Tamper: change TenantID after signing.
	m.SignedBlock = &canonical.SignedBlock{KeyID: "alias/test-key", Signature: sig}
	(&m).TenantID = string(tenant) + "-hacked" // mutate after signing

	buf := buildTestTar(tenant, m, sig)

	importer := canonical.Importer{
		TenantID: tenant,
		Graph:    graph,
		Journal:  journal,
		Signer:   signer,
	}
	if err := importer.Import(ctx, &buf, canonical.ImporterOpts{VerifySignature: true}); err == nil {
		t.Fatalf("import tampered: expected error, got nil")
	}
}
