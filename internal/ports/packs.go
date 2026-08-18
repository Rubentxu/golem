package ports

import (
	"context"
	"errors"
)

// Pack sentinel errors. Shared across the pack activation subsystem so that
// failures are asserted by identity (errors.Is), not message.
var (
	// ErrPackNotFound is returned when a pack cannot be loaded from the
	// given source path (file not found, directory empty, etc.).
	ErrPackNotFound = errors.New("pack: not found at source path")

	// ErrPackIntegrityFailed is returned when the computed integrity digest
	// does not match the digest declared in the manifest.
	ErrPackIntegrityFailed = errors.New("pack: integrity_digest mismatch")

	// ErrPackManifestInvalid is returned when the manifest fails structural
	// or semantic validation (unknown format_version, bad digest hex, etc.).
	ErrPackManifestInvalid = errors.New("pack: manifest validation failed")

	// ErrPackUnknownPermission is returned when a permission declared in the
	// manifest is not in the closed permission catalog v1.
	ErrPackUnknownPermission = errors.New("pack: permission not in closed catalog")

	// ErrPackAlreadyActivated is returned when attempting to activate a pack
	// that is already active for the given tenant. The journal already
	// contains an activation event for (tenant, name).
	ErrPackAlreadyActivated = errors.New("pack: already activated for tenant")

	// ErrUnsupportedInM51 is returned when the manifest declares migrations or
	// UI contributions. Those features are reserved for GOLEM 0.6+.
	ErrUnsupportedInM51 = errors.New("pack: migrations and ui contributions are not supported in GOLEM 0.5.1; reserved for M6")
)

// Pack describes a capability pack loaded from a registry adapter.
type Pack struct {
	TenantID        TenantID `json:"tenant_id,omitempty"`
	Name            string   `json:"name"`
	Version         string   `json:"version"`
	IntegrityDigest string   `json:"integrity_digest"` // sha256 hex (64 chars) of the pack's canonical manifest
	FormatVersion   string   `json:"format_version"`   // always "1"
	Capabilities    []string `json:"capabilities,omitempty"`
	Permissions     []string `json:"permissions,omitempty"`
}

// PackRegistry is the activation registry for capability packs. Adapters decide
// WHERE packs come from (filesystem, OCI, etc.); the registry owns the
// activation lifecycle (Load → Verify → Activate → Status → Deactivate).
//
// All methods are pure domain operations; state is stored in the journal.
type PackRegistry interface {
	// Load reads a pack from the given source path (adapter-specific resolution:
	// filesystem = relative to adapter root, OCI = reference string, etc.).
	// Load populates IntegrityDigest and FormatVersion from the manifest.
	// Returns ErrPackNotFound if the source cannot be resolved.
	Load(ctx context.Context, sourcePath string) (*Pack, error)

	// Verify computes the digest independently from the source and compares it
	// to the manifest's integrity_digest field. Exposed separately for callers
	// that want to check without loading the full pack.
	// Returns ErrPackIntegrityFailed on mismatch; ErrPackManifestInvalid if
	// the manifest itself is unreadable.
	Verify(ctx context.Context, sourcePath string) error

	// Activate idempotently activates the pack for the tenant.
	// Idempotency: calling Activate again with the same (tenant, name, digest)
	// returns ErrPackAlreadyActivated without duplicating the journal event.
	// Uses journal.AppendIf(expected=StreamVersion{Version:0}) so that the
	// first call with expected=0 succeeds; a second call finds Version≥1
	// and gets ErrVersionConflict → wrapped as ErrPackAlreadyActivated.
	Activate(ctx context.Context, tenant TenantID, p *Pack) (StreamPosition, error)

	// Status returns the activation state for (tenant, name). If the pack
	// is not activated, returns (nil, nil). This is the recovery path for
	// callers that receive ErrPackAlreadyActivated.
	Status(ctx context.Context, tenant TenantID, name string) (*Pack, error)

	// Deactivate is a no-op in M5.1 (reserved for M6). It exists to keep
	// the Port stable across releases.
	Deactivate(ctx context.Context, tenant TenantID, name string) error
}
