// Package filesystem is the reference PackRegistry adapter: it loads
// capability packs from a directory tree on disk (M5.1 distribution model
// per ADR-058 — OCI arrives in M8). Layout:
//
//	<root>/<source-path>/manifest.json
//
// The adapter is a thin composition: reading/validating the manifest is
// filesystem-specific; the activation lifecycle delegates to
// internal/packs.Activator, whose state lives in the journal.
package filesystem

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Rubentxu/golem/internal/packs"
	"github.com/Rubentxu/golem/internal/ports"
)

// DefaultRoot is the pack directory used when no root is given (Q9:
// ./packs relative to the working directory).
const DefaultRoot = "packs"

// Registry implements ports.PackRegistry over a filesystem root.
type Registry struct {
	root      string
	activator *packs.Activator
	catalog   packs.PermissionCatalog
}

// New builds a filesystem pack registry. root is the directory packs are
// resolved against (DefaultRoot when empty).
func New(root string, journal ports.JournalStore, ids ports.IDGenerator, clock ports.Clock) *Registry {
	if root == "" {
		root = DefaultRoot
	}
	catalog := packs.NewPermissionCatalogV1()
	return &Registry{
		root:      root,
		activator: packs.NewActivator(journal, ids, clock, catalog),
		catalog:   catalog,
	}
}

// manifestPath resolves the manifest location for a source path.
func (r *Registry) manifestPath(sourcePath string) string {
	return filepath.Join(r.root, filepath.FromSlash(sourcePath), "manifest.json")
}

// readManifest loads and validates the manifest at sourcePath, returning
// the parsed manifest and the verbatim bytes (digest contract).
func (r *Registry) readManifest(sourcePath string) (*packs.Manifest, []byte, error) {
	path := r.manifestPath(sourcePath)
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, fmt.Errorf("%w: %s", ports.ErrPackNotFound, path)
		}
		return nil, nil, fmt.Errorf("packs fs: read %s: %w", path, err)
	}
	var m packs.Manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, nil, fmt.Errorf("%w: manifest.json decode: %v", ports.ErrPackManifestInvalid, err)
	}
	if err := m.Validate(r.catalog); err != nil {
		return nil, nil, err
	}
	return &m, raw, nil
}

// Load reads and validates the pack at sourcePath (relative to the root).
func (r *Registry) Load(_ context.Context, sourcePath string) (*ports.LoadedPack, error) {
	m, raw, err := r.readManifest(sourcePath)
	if err != nil {
		return nil, err
	}
	return &ports.LoadedPack{
		Name:            m.Name,
		Version:         m.Version,
		IntegrityDigest: m.IntegrityDigest,
		RawManifest:     raw,
	}, nil
}

// Verify recomputes the digest from the source and compares it to the
// manifest's declared integrity_digest.
func (r *Registry) Verify(_ context.Context, sourcePath string) error {
	m, raw, err := r.readManifest(sourcePath)
	if err != nil {
		return err
	}
	if !m.DigestMatchesCanonical(raw) {
		return fmt.Errorf("%w: %s", ports.ErrPackIntegrityFailed, r.manifestPath(sourcePath))
	}
	return nil
}

// Activate activates the pack for the tenant (delegates to the journal-first
// activator; idempotent per (tenant, name)).
func (r *Registry) Activate(ctx context.Context, tenant ports.TenantID, p *ports.LoadedPack) (ports.StreamPosition, error) {
	var m packs.Manifest
	if err := json.Unmarshal(p.RawManifest, &m); err != nil {
		return 0, fmt.Errorf("%w: manifest decode: %v", ports.ErrPackManifestInvalid, err)
	}
	res, err := r.activator.Activate(ctx, tenant, &m, p.RawManifest)
	if err != nil {
		return 0, err
	}
	return res.Position, nil
}

// Status returns the activation state for (tenant, name).
func (r *Registry) Status(ctx context.Context, tenant ports.TenantID, name string) (*ports.PackStatus, error) {
	return r.activator.Status(ctx, tenant, name)
}

// Deactivate is a no-op in M5.1 (reserved for M6).
func (r *Registry) Deactivate(_ context.Context, _ ports.TenantID, _ string) error {
	return nil
}
