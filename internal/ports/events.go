package ports

import "time"

// EventType constants for migration harness audit events (ADR-005 naming convention).
// The migration harness emits these via journal.Append during R4 rehearsal.
// Spec scenario: Replay(0,0) never contains an EventType starting with
// "extension.pack." during M5; ReservedEventPrefixExtensionPack reserves that
// prefix for M5.1 (WASM + OCI capability packs).
const (
	EventMigrationHarnessStarted    = "migration.harness.started.v1"
	EventMigrationHarnessDiffed     = "migration.harness.diffed.v1"
	EventMigrationHarnessCutover    = "migration.harness.cutover.v1"
	EventMigrationHarnessCompleted  = "migration.harness.completed.v1"
	EventMigrationHarnessRolledBack = "migration.harness.rolled_back.v1"

	// ReservedEventPrefixExtensionPack reserves the "extension.pack." prefix
	// for capability packs. In M5.1 the first real event materialises under
	// this prefix: pack activation (EventExtensionPackActivated). WASM
	// execution events will follow in M6.
	ReservedEventPrefixExtensionPack = "extension.pack."

	// EventExtensionPackActivated is emitted when a capability pack is
	// activated for a tenant (M5.1). Payload: {name, version,
	// integrity_digest, capabilities_required, permissions}; actor, tenant
	// and occurred_at travel in the envelope. Emitted via journal.AppendIf
	// on stream "extension.pack.{tenant}.{name}" — exactly once per
	// (tenant, name) by optimistic concurrency (expected version 0).
	EventExtensionPackActivated = "extension.pack.activated.v1"

	// EventScenarioPromoted is emitted when a scenario fork is promoted
	// (M6). Payload: {scenario_id, base_position, events_applied,
	// promoted_by}. Emitted on stream "scenario.{id}" after the overlay
	// batch has been appended atomically.
	EventScenarioPromoted = "scenario.promoted.v1"
)

// Actor identifies who or what performed an action. Actor and tenant are
// mandatory on every journal event.
type Actor struct {
	Type string `json:"type"` // "user", "service", "agent", ...
	ID   string `json:"id"`
}

// Envelope is the canonical Graph Journal event envelope (ADR-005, ADR-031).
//
// Invariants: EventID is unique, the payload is immutable after acceptance,
// schemas are versioned, actor and tenant are mandatory, causation is
// explicit when it exists, and secrets never enter the journal.
type Envelope[T any] struct {
	EventID       string    `json:"event_id"`
	TenantID      string    `json:"tenant_id"`
	StreamID      string    `json:"stream_id,omitempty"`
	EventType     string    `json:"event_type"`
	SchemaVersion int       `json:"schema_version"`
	OccurredAt    time.Time `json:"occurred_at"`
	Actor         Actor     `json:"actor"`
	CorrelationID string    `json:"correlation_id,omitempty"`
	CausationID   string    `json:"causation_id,omitempty"`
	CommandID     string    `json:"command_id,omitempty"`
	FrameID       string    `json:"frame_id,omitempty"`
	Payload       T         `json:"payload"`
	EvidenceRefs  []string  `json:"evidence_refs,omitempty"`
}
