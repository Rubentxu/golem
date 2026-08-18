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
	EventScenarioPromoted = "scenario.delta.promoted.v1"

	// Proposal lifecycle events (M7, ADR-065).
	EventProposalProposed   = "proposal.proposed.v1"
	EventProposalApproved   = "proposal.approved.v1"
	EventProposalRejected   = "proposal.rejected.v1"
	EventProposalConflicted = "proposal.conflicted.v1"
	EventProposalApplied    = "proposal.applied.v1"

	// Agent eval events (M7, ADR-067).
	EventAgentEvalCompleted = "agent.eval.completed.v1"

	// Agent lifecycle events (M7, ADR-066, ADR-068).
	// agent.llm.call.completed.v1: emitted after each LLM call with a
	// redacted summary (ADR-066) and correlation_id linking to OTel span.
	EventAgentLLMCallCompleted = "agent.llm.call.completed.v1"
	// agent.tool.invoked.v1: emitted after each tool call with a redacted
	// summary and correlation_id.
	EventAgentToolInvoked = "agent.tool.invoked.v1"
	// agent.budget.exceeded.v1: emitted when a budget limit is hit (AC-8).
	EventAgentBudgetExceeded = "agent.budget.exceeded.v1"
	// agent.principal.authenticated.v1: emitted when an agent principal is
	// constructed and verified (AC-1).
	EventAgentPrincipalAuthenticated = "agent.principal.authenticated.v1"
	// agent.injection.detected.v1: emitted when a prompt injection attempt is
	// detected by the static template + Redactor (R-1).
	EventAgentInjectionDetected = "agent.injection.detected.v1"
)

// --- Agent event payloads (M7) ---

// AgentLLMCallPayload is the payload for agent.llm.call.completed.v1.
// Prompt and content are redacted by the Redactor before journal entry
// (ADR-066). The correlation_id links to the OTel span (ADR-068).
type AgentLLMCallPayload struct {
	Provider        string `json:"provider"`         // e.g. "openai-compatible", "memstore"
	Model          string `json:"model"`           // model used
	Operation      string `json:"operation"`        // e.g. "complete", "embed"
	InputTokens    int    `json:"input_tokens"`    // usage.input_tokens
	OutputTokens   int    `json:"output_tokens"`   // usage.output_tokens
	LatencyMs      int64  `json:"latency_ms"`      // wall clock ms
	RedactedPrompt string `json:"redacted_prompt"` // Redactor output, never raw
	CorrelationID  string `json:"correlation_id"`  // links to OTel span (ADR-019)
}

// AgentToolInvokePayload is the payload for agent.tool.invoked.v1.
type AgentToolInvokePayload struct {
	ToolID         string `json:"tool_id"`          // Tool.ID()
	ToolVersion    string `json:"tool_version"`     // Tool.Version()
	RedactedArgs   string `json:"redacted_args"`    // Redacted tool arguments (ADR-066)
	CorrelationID  string `json:"correlation_id"`   // links to OTel span
}

// AgentBudgetExceededPayload is the payload for agent.budget.exceeded.v1.
type AgentBudgetExceededPayload struct {
	BudgetKind string `json:"budget_kind"` // e.g. "token_cost", "wall_clock", "tool_calls"
	Limit      string `json:"limit"`        // string representation of the limit
	Actual     string `json:"actual"`       // string representation of actual consumption
}

// AgentPrincipalAuthenticatedPayload is the payload for agent.principal.authenticated.v1.
type AgentPrincipalAuthenticatedPayload struct {
	PrincipalType string `json:"principal_type"` // always "agent"
	PrincipalID   string `json:"principal_id"`
	FrameID      string `json:"frame_id,omitempty"`
}

// AgentInjectionDetectedPayload is the payload for agent.injection.detected.v1.
type AgentInjectionDetectedPayload struct {
	AttemptedContent string `json:"attempted_content"` // snippet of the injection attempt (redacted)
	CorrelationID  string `json:"correlation_id"`
}

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
//
// Frame (M7) replaces FrameID for agentic behaviors. MarshalJSON writes
// frame_id for backwards compatibility with v1 readers (ADR-064).
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
	FrameID       string    `json:"frame_id,omitempty"` // kept for v1 backwards compat
	Frame         *Frame    `json:"frame,omitempty"`    // M7: replaces FrameID
	Payload       T         `json:"payload"`
	EvidenceRefs  []string  `json:"evidence_refs,omitempty"`
}
