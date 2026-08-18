// Package packs activator: journal-first pack activation.
//
// The activator owns the activation lifecycle of a capability pack for a
// tenant. State lives exclusively in the journal (ADR-021): activation is
// one AppendIf with expected stream version 0, so idempotency falls out of
// optimistic concurrency instead of a read-then-write race. There is no
// graph node and no checkpoint store — the journal is the source of truth,
// queryable via Replay/ReadStream.
package packs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Rubentxu/golem/internal/ports"
)

// Activator activates capability packs for tenants, journalling exactly one
// extension.pack.activated.v1 event per (tenant, name).
type Activator struct {
	journal ports.JournalStore
	ids     ports.IDGenerator
	clock   ports.Clock
	catalog PermissionCatalog
}

// NewActivator wires the activator. catalog is the closed permission
// catalogue the manifests are validated against (NewPermissionCatalogV1).
func NewActivator(journal ports.JournalStore, ids ports.IDGenerator, clock ports.Clock, catalog PermissionCatalog) *Activator {
	return &Activator{journal: journal, ids: ids, clock: clock, catalog: catalog}
}

// activatedPayload is the Q7 event payload: identifying data only — actor,
// tenant and occurred_at travel in the envelope, never duplicated here.
type activatedPayload struct {
	Name                 string   `json:"name"`
	Version              string   `json:"version"`
	IntegrityDigest      string   `json:"integrity_digest"`
	CapabilitiesRequired []string `json:"capabilities_required"`
	Permissions          []string `json:"permissions"`
}

// ActivationResult reports a successful activation.
type ActivationResult struct {
	EventID  string
	Position ports.StreamPosition
}

// Activate validates the manifest, verifies the digest against the raw
// manifest bytes, and journals the activation event.
//
// Order of checks (fail-fast, no journal mutation on failure):
//  1. Manifest.Validate (format_version, digest shape, golem_api range, permission catalog)
//  2. migrations/ui declared → ErrUnsupportedInM51 (M5.1 gate)
//  3. digest mismatch → ErrPackIntegrityFailed
//  4. journal.AppendIf(expected=0) — ErrVersionConflict → ErrPackAlreadyActivated
//
// rawManifestBytes are the bytes read from the pack's manifest.json; they are
// re-canonicalised for digest verification so key order and whitespace do
// not matter (Q3 determinism).
func (a *Activator) Activate(ctx context.Context, tenant ports.TenantID, m *Manifest, rawManifestBytes []byte) (ActivationResult, error) {
	if err := m.Validate(a.catalog); err != nil {
		return ActivationResult{}, err
	}
	if len(m.Migrations) > 0 {
		return ActivationResult{}, fmt.Errorf("%w: manifest declares %d migration(s)", ports.ErrUnsupportedInM51, len(m.Migrations))
	}
	if len(m.UI) > 0 {
		return ActivationResult{}, fmt.Errorf("%w: manifest declares %d ui contribution(s)", ports.ErrUnsupportedInM51, len(m.UI))
	}
	if !m.DigestMatchesCanonical(rawManifestBytes) {
		return ActivationResult{}, fmt.Errorf("%w: declared %s", ports.ErrPackIntegrityFailed, m.IntegrityDigest)
	}

	payload, err := json.Marshal(activatedPayload{
		Name:                 m.Name,
		Version:              m.Version,
		IntegrityDigest:      m.IntegrityDigest,
		CapabilitiesRequired: m.Capabilities,
		Permissions:          m.Permissions,
	})
	if err != nil {
		return ActivationResult{}, fmt.Errorf("packs: payload marshal: %w", err)
	}

	streamID := PackStreamID(tenant, m.Name)
	env := ports.RawEvent{
		EventID:       a.ids.NewID(),
		TenantID:      string(tenant),
		StreamID:      streamID,
		EventType:     ports.EventExtensionPackActivated,
		SchemaVersion: 1,
		OccurredAt:    a.clock.Now(),
		Actor: ports.Actor{
			Type: "service",
			ID:   "pack-activator",
		},
		Payload: payload,
	}

	results, err := a.journal.AppendIf(ctx, ports.StreamVersion{
		TenantID: tenant,
		StreamID: streamID,
		Version:  0,
	}, []ports.RawEvent{env})
	if err != nil {
		if errors.Is(err, ports.ErrVersionConflict) {
			return ActivationResult{}, fmt.Errorf("%w: tenant=%s name=%s", ports.ErrPackAlreadyActivated, tenant, m.Name)
		}
		return ActivationResult{}, fmt.Errorf("packs: append activation: %w", err)
	}
	if len(results) != 1 {
		return ActivationResult{}, fmt.Errorf("packs: expected 1 append result, got %d", len(results))
	}
	return ActivationResult{
		EventID:  results[0].EventID,
		Position: results[0].Position,
	}, nil
}

// Status returns the activated pack for (tenant, name), or (nil, nil) when
// the pack was never activated. The recovery path for callers that receive
// ErrPackAlreadyActivated.
func (a *Activator) Status(ctx context.Context, tenant ports.TenantID, name string) (*ports.PackStatus, error) {
	events, err := a.journal.ReadStream(ctx, tenant, PackStreamID(tenant, name), 0)
	if err != nil {
		return nil, fmt.Errorf("packs: status read: %w", err)
	}
	for i := len(events) - 1; i >= 0; i-- {
		if events[i].EventType != ports.EventExtensionPackActivated {
			continue
		}
		var p activatedPayload
		if err := json.Unmarshal(events[i].Payload, &p); err != nil {
			return nil, fmt.Errorf("packs: status payload decode: %w", err)
		}
		return &ports.PackStatus{
			Name:             p.Name,
			Version:          p.Version,
			IntegrityDigest:  p.IntegrityDigest,
			ActivatedEventID: events[i].EventID,
		}, nil
	}
	return nil, nil
}

// PackStreamID derives the journal stream for (tenant, name). Every
// activation for the same tenant+name lands on the same stream, which is
// what makes AppendIf(expected=0) the dedupe mechanism.
func PackStreamID(tenant ports.TenantID, name string) string {
	return fmt.Sprintf("extension.pack.%s.%s", tenant, name)
}
