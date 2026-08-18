package canonical

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"time"
)

// Manifest describes the contents of a canonical export archive.
// It is serialized as manifest.json in the archive root.
// Format version 2 adds a SignedBlock for KMS signature verification (AC-14).
type Manifest struct {
	FormatVersion   string                `json:"format_version"`         // "2" for signed manifests
	TenantID        string                `json:"tenant_id"`              // Tenant scope
	CreatedAt       string                `json:"created_at"`             // ISO-8601
	JournalPosition JournalPosition       `json:"journal_position"`       // Head position at export time
	Files           map[string]FileDigest `json:"files"`                  // path → sha256
	Counts          Counts                `json:"counts"`                 // node/edge line counts
	SignedBlock     *SignedBlock          `json:"signed_block,omitempty"` // KMS signature (v2+)
	Extensions      map[string]any        `json:"extensions,omitempty"`   // Forward-compat
}

// SignedBlock holds the KMS signature over the canonical export bytes (AC-14, REQ-AUDIT-002).
type SignedBlock struct {
	// KeyID is the KMS key alias used for signing (e.g. alias/golem-export).
	KeyID string `json:"key_id"`
	// Signature is the hex-encoded RSASSA-PKCS1-v1.5 signature over the
	// canonical export tar bytes (all files concatenated in deterministic order).
	Signature string `json:"signature"`
	// Regions lists the S3 replication destinations for multi-region durability.
	Regions []string `json:"regions,omitempty"`
}

// FileDigest holds the SHA-256 digest of one file in the archive.
type FileDigest struct {
	SHA256 string `json:"sha256"`
}

// JournalPosition records the journal head at snapshot time.
type JournalPosition struct {
	Head     uint64 `json:"head"`
	TenantID string `json:"tenant_id"`
}

// Counts holds the line counts of the JSONL files.
type Counts struct {
	Nodes uint64 `json:"nodes"`
	Edges uint64 `json:"edges"`
}

// NewManifest creates a manifest with the given metadata.
func NewManifest(tenantID string, journalHead uint64) Manifest {
	return Manifest{
		FormatVersion: FormatVersion,
		TenantID:      tenantID,
		CreatedAt:     time.Now().UTC().Format(time.RFC3339),
		JournalPosition: JournalPosition{
			Head:     journalHead,
			TenantID: tenantID,
		},
		Files:      make(map[string]FileDigest),
		Counts:     Counts{},
		Extensions: make(map[string]any),
	}
}

// SetFileDigest records the SHA-256 digest of one archive file.
func (m *Manifest) SetFileDigest(path string, digest string) {
	m.Files[path] = FileDigest{SHA256: digest}
}

// SetCounts records the node and edge line counts.
func (m *Manifest) SetCounts(nodes, edges uint64) {
	m.Counts = Counts{Nodes: nodes, Edges: edges}
}

// Validate checks the manifest is valid for reading.
// Supports both v1 (unsigned) and v2 (signed) formats.
func (m *Manifest) Validate() error {
	switch m.FormatVersion {
	case "1", "2":
		return nil
	default:
		return fmt.Errorf("%w: %q", ErrUnsupportedFormatVersion, m.FormatVersion)
	}
}

// SignedPayload returns the canonical bytes that should be signed.
// For v2, this is the JSON bytes of the manifest excluding the SignedBlock field.
func (m *Manifest) SignedPayload() ([]byte, error) {
	// Clone manifest without SignedBlock for signing.
	signable := struct {
		FormatVersion   string                `json:"format_version"`
		TenantID        string                `json:"tenant_id"`
		CreatedAt       string                `json:"created_at"`
		JournalPosition JournalPosition       `json:"journal_position"`
		Files           map[string]FileDigest `json:"files"`
		Counts          Counts                `json:"counts"`
		Extensions      map[string]any        `json:"extensions,omitempty"`
	}{
		FormatVersion:   m.FormatVersion,
		TenantID:        m.TenantID,
		CreatedAt:       m.CreatedAt,
		JournalPosition: m.JournalPosition,
		Files:           m.Files,
		Counts:          m.Counts,
		Extensions:      m.Extensions,
	}
	return json.Marshal(signable)
}

// MarshalJSON serializes the manifest to JSON.
func (m Manifest) MarshalJSON() ([]byte, error) {
	type alias Manifest // avoid recursion
	return json.Marshal(alias(m))
}

// ComputeSHA256 reads all content from r and returns the SHA-256 hex digest.
func ComputeSHA256(r io.Reader) (string, error) {
	h := sha256.New()
	if _, err := io.Copy(h, r); err != nil {
		return "", fmt.Errorf("sha256: %w", err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
