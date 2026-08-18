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
type Manifest struct {
	FormatVersion   string                `json:"format_version"`       // Always "1"
	TenantID        string                `json:"tenant_id"`            // Tenant scope
	CreatedAt       string                `json:"created_at"`           // ISO-8601
	JournalPosition JournalPosition       `json:"journal_position"`     // Head position at export time
	Files           map[string]FileDigest `json:"files"`                // path → sha256
	Counts          Counts                `json:"counts"`               // node/edge line counts
	Extensions      map[string]any        `json:"extensions,omitempty"` // Forward-compat
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
func (m *Manifest) Validate() error {
	if m.FormatVersion != FormatVersion {
		return fmt.Errorf("%w: %q", ErrUnsupportedFormatVersion, m.FormatVersion)
	}
	return nil
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
