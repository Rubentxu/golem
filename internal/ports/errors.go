package ports

import "errors"

// Kernel contract errors. Shared by ports, adapters and TCKs so that
// semantic failures are asserted by identity (errors.Is), not message.
var (
	// ErrEmptyTenant enforces the mandatory tenant invariant (ADR-008).
	ErrEmptyTenant = errors.New("ports: tenant is mandatory")
	// ErrNoTenant reports a missing TenantContext.
	ErrNoTenant = errors.New("ports: no tenant in context")
	// ErrEmptyEventID, ErrEmptyActor, ErrZeroTimestamp and
	// ErrInvalidEventType enforce the event envelope invariants (ADR-005).
	ErrEmptyEventID     = errors.New("ports: event_id is mandatory")
	ErrEmptyActor       = errors.New("ports: actor is mandatory")
	ErrZeroTimestamp    = errors.New("ports: occurred_at is mandatory")
	ErrInvalidEventType = errors.New("ports: event_type must be <context>.<entity>.<verb>.v<major>")

	// ErrVersionConflict reports a failed optimistic-concurrency check:
	// the stream moved since the caller read it (ADR-021).
	ErrVersionConflict = errors.New("ports: stream version conflict")

	// Graph mutation errors.
	ErrEmptyMutation    = errors.New("ports: empty graph mutation")
	ErrInvalidOp        = errors.New("ports: invalid graph op")
	ErrNodeNotFound     = errors.New("ports: node not found")
	ErrEdgeNotFound     = errors.New("ports: edge not found")
	ErrKindMismatch     = errors.New("ports: node kind is immutable")
	ErrTypeMismatch     = errors.New("ports: edge type is immutable")
	ErrEndpointNotFound = errors.New("ports: edge endpoint not found")
	ErrUnboundedQuery   = errors.New("ports: query must be bounded (max depth/nodes/edges > 0)")
)
