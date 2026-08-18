package ports

import (
	"context"
	"encoding/json"
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

// LoadedPack is a pack resolved from a registry source. RawManifest carries
// the verbatim manifest.json bytes: the digest contract is computed over the
// canonical re-serialisation of those bytes, so adapters must hand them
// through untouched instead of re-marshalling.
type LoadedPack struct {
	Name            string
	Version         string
	IntegrityDigest string
	RawManifest     json.RawMessage
}

// PackStatus is the query-side view of one activation.
type PackStatus struct {
	Name             string
	Version          string
	IntegrityDigest  string
	ActivatedEventID string
}

// PackRegistry is the activation registry for capability packs. Adapters
// decide WHERE packs come from (filesystem, OCI, ...); the lifecycle
// (validate → verify → activate exactly once per tenant) is the port
// contract. State lives in the journal — there is no side table.
type PackRegistry interface {
	// Load reads and validates a pack from the given source path
	// (adapter-specific resolution: filesystem = relative to the adapter
	// root). Returns ErrPackNotFound when the source cannot be resolved and
	// ErrPackManifestInvalid when the manifest fails validation.
	Load(ctx context.Context, sourcePath string) (*LoadedPack, error)

	// Verify computes the digest from the source and compares it to the
	// manifest's declared integrity_digest. Returns ErrPackIntegrityFailed
	// on mismatch.
	Verify(ctx context.Context, sourcePath string) error

	// Activate activates the pack for the tenant, journalling exactly one
	// extension.pack.activated.v1 event. Re-activation returns
	// ErrPackAlreadyActivated without duplicating the event.
	Activate(ctx context.Context, tenant TenantID, p *LoadedPack) (StreamPosition, error)

	// Status returns the activation state for (tenant, name), or (nil, nil)
	// when the pack was never activated for that tenant.
	Status(ctx context.Context, tenant TenantID, name string) (*PackStatus, error)

	// Deactivate is a no-op in M5.1 (reserved for M6). Its signature exists
	// to keep the port stable across releases.
	Deactivate(ctx context.Context, tenant TenantID, name string) error
}
