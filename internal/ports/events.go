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

	// Cell lifecycle events (M8, ADR-074, REQ-CELL-006).
	// cell.promoted.v1: emitted when a cell becomes newly operative.
	EventCellPromoted = "cell.promoted.v1"
	// cell.demoted.v1: emitted when a cell is drained or taken offline.
	EventCellDemoted = "cell.demoted.v1"
	// cell.routing.conflict_detected.v1: emitted when a routing conflict is detected.
	EventCellRoutingConflict = "cell.routing.conflict_detected.v1"

	// Tenant migration events (M8, ADR-075, REQ-MIG-003).
	// tenant.migration.started.v1: emitted when migration begins.
	EventTenantMigrationStarted = "tenant.migration.started.v1"
	// tenant.migration.shadowed.v1: emitted after each shadow-read completed.
	EventTenantMigrationShadowed = "tenant.migration.shadowed.v1"
	// tenant.migration.cutover.v1: emitted when cutover begins.
	EventTenantMigrationCutover = "tenant.migration.cutover.v1"
	// tenant.migration.completed.v1: emitted on successful migration.
	EventTenantMigrationCompleted = "tenant.migration.completed.v1"
	// tenant.migration.failed.v1: emitted on failed migration.
	EventTenantMigrationFailed = "tenant.migration.failed.v1"

	// SLO events (M8, ADR-080, REQ-SLO-003).
	// slo.budget.burn.v1: emitted when burn rate exceeds 2x threshold.
	EventSLOBudgetBurn = "slo.budget.burn.v1"
	// slo.budget.exhausted.v1: emitted when error budget is > 90% exhausted.
	EventSLOBudgetExhausted = "slo.budget.exhausted.v1"

	// Ops console audit events (M8, ADR-081, REQ-OPS-003).
	// ops.console.action.completed.v1: emitted after each successful admin operation.
	EventOpsConsoleActionCompleted = "ops.console.action.completed.v1"
	// ops.console.action.rejected.v1: emitted when an admin operation fails.
	EventOpsConsoleActionRejected = "ops.console.action.rejected.v1"

	// OIDC events (M8, ADR-082, REQ-OIDC-004).
	// oidc.token.verified.v1: emitted after successful JWT verification.
	EventOIDCTokenVerified = "oidc.token.verified.v1"
	// oidc.token.rejected.v1: emitted when JWT verification fails.
	EventOIDCTokenRejected = "oidc.token.rejected.v1"

	// Audit export events (M8, ADR-078, REQ-AUDIT-005).
	// audit.export.completed.v1: emitted after successful canonical export + S3 upload + KMS sign.
	EventAuditExportCompleted = "audit.export.completed.v1"
)

// --- Agent event payloads (M7) ---

// AgentLLMCallPayload is the payload for agent.llm.call.completed.v1.
// Prompt and content are redacted by the Redactor before journal entry
// (ADR-066). The correlation_id links to the OTel span (ADR-068).
type AgentLLMCallPayload struct {
	Provider       string `json:"provider"`        // e.g. "openai-compatible", "memstore"
	Model          string `json:"model"`           // model used
	Operation      string `json:"operation"`       // e.g. "complete", "embed"
	InputTokens    int    `json:"input_tokens"`    // usage.input_tokens
	OutputTokens   int    `json:"output_tokens"`   // usage.output_tokens
	LatencyMs      int64  `json:"latency_ms"`      // wall clock ms
	RedactedPrompt string `json:"redacted_prompt"` // Redactor output, never raw
	CorrelationID  string `json:"correlation_id"`  // links to OTel span (ADR-019)
}

// AgentToolInvokePayload is the payload for agent.tool.invoked.v1.
type AgentToolInvokePayload struct {
	ToolID        string `json:"tool_id"`        // Tool.ID()
	ToolVersion   string `json:"tool_version"`   // Tool.Version()
	RedactedArgs  string `json:"redacted_args"`  // Redacted tool arguments (ADR-066)
	CorrelationID string `json:"correlation_id"` // links to OTel span
}

// AgentBudgetExceededPayload is the payload for agent.budget.exceeded.v1.
type AgentBudgetExceededPayload struct {
	BudgetKind string `json:"budget_kind"` // e.g. "token_cost", "wall_clock", "tool_calls"
	Limit      string `json:"limit"`       // string representation of the limit
	Actual     string `json:"actual"`      // string representation of actual consumption
}

// AgentPrincipalAuthenticatedPayload is the payload for agent.principal.authenticated.v1.
type AgentPrincipalAuthenticatedPayload struct {
	PrincipalType string `json:"principal_type"` // always "agent"
	PrincipalID   string `json:"principal_id"`
	FrameID       string `json:"frame_id,omitempty"`
}

// AgentInjectionDetectedPayload is the payload for agent.injection.detected.v1.
type AgentInjectionDetectedPayload struct {
	AttemptedContent string `json:"attempted_content"` // snippet of the injection attempt (redacted)
	CorrelationID    string `json:"correlation_id"`
}

// SLOBudgetBurnPayload is the payload for slo.budget.burn.v1.
type SLOBudgetBurnPayload struct {
	SLOName     string  `json:"slo_name"`
	BurnRate    float64 `json:"burn_rate"`
	WindowHours int     `json:"window_hours"`
	BudgetLeft  float64 `json:"budget_left"`  // fraction of budget remaining
	ErrorRate   float64 `json:"error_rate"`   // observed error rate
	AllowedRate float64 `json:"allowed_rate"` // allowed error rate
}

// SLOBudgetExhaustedPayload is the payload for slo.budget.exhausted.v1.
type SLOBudgetExhaustedPayload struct {
	SLOName         string  `json:"slo_name"`
	BudgetConsumed  float64 `json:"budget_consumed"`  // fraction 0..1
	BudgetRemaining float64 `json:"budget_remaining"` // fraction 0..1
	WindowHours     int     `json:"window_hours"`
}

// OpsConsoleActionPayload is the payload for ops.console.action.{completed,rejected}.v1.
// Emitted by the audit middleware on every admin endpoint (REQ-OPS-003).
type OpsConsoleActionPayload struct {
	Action      string `json:"action"`            // e.g. "cell.migrate", "tenant.assign"
	Target      string `json:"target,omitempty"`  // e.g. tenant_id or cell_id
	Status      string `json:"status"`            // "completed" or "rejected"
	Subject     string `json:"subject,omitempty"` // principal subject
	Correlation string `json:"correlation_id,omitempty"`
	Detail      string `json:"detail,omitempty"` // error message if rejected
}

// OIDCTokenVerifiedPayload is the payload for oidc.token.verified.v1.
type OIDCTokenVerifiedPayload struct {
	Subject     string   `json:"subject"`
	Issuer      string   `json:"issuer"`
	Groups      []string `json:"groups,omitempty"`
	Correlation string   `json:"correlation_id,omitempty"`
}

// OIDCTokenRejectedPayload is the payload for oidc.token.rejected.v1.
type OIDCTokenRejectedPayload struct {
	Error       string `json:"error"` // error message
	Issuer      string `json:"issuer,omitempty"`
	Correlation string `json:"correlation_id,omitempty"`
}

// AuditExportCompletedPayload is the payload for audit.export.completed.v1.
// Emitted after a successful canonical export cycle: snapshot + tar + S3 upload + KMS sign (REQ-AUDIT-005).
type AuditExportCompletedPayload struct {
	TenantID      string   `json:"tenant_id"`
	JournalHead   uint64   `json:"journal_head"`      // journal position at export time
	NodeCount     uint64   `json:"node_count"`        // nodes exported
	EdgeCount     uint64   `json:"edge_count"`        // edges exported
	S3Bucket      string   `json:"s3_bucket"`         // destination S3 bucket
	S3Key         string   `json:"s3_key"`            // destination S3 object key
	KMSKeyAlias   string   `json:"kms_key_alias"`     // KMS key used for signing
	Signature     string   `json:"signature"`         // hex signature over manifest
	FormatVersion string   `json:"format_version"`    // "1" or "2"
	Regions       []string `json:"regions,omitempty"` // S3 replication regions
	DurationMs    int64    `json:"duration_ms"`       // export cycle duration
	CorrelationID string   `json:"correlation_id,omitempty"`
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
