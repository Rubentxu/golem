package ports

import "time"

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
