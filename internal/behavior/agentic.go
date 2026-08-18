// Package behavior implements the GOLEM behavior engine v1
// (BEHAVIOR_RUNTIME.md): deterministic and relation behaviors with native
// Go handlers, subscription-indexed and executed over the typed traversal
// substrate. Workflow and agentic kinds arrive in M7.
package behavior

import (
	"context"

	"github.com/Rubentxu/golem/internal/agent/observability"
	"github.com/Rubentxu/golem/internal/ports"
)

// Kind values for Behavior.Kind().
const (
	// KindDeterministic is the v1 behavior kind (native Go handlers).
	KindDeterministic = "deterministic"
	// KindAgentic is the M7 agentic behavior kind (LLM-driven).
	KindAgentic = "agentic"
)

// AgenticContext is the execution context for agentic behaviors (ADR-070).
// It provides access to the LLM provider, tools, frame, and budget.
// Nil for non-agentic behaviors — always check Kind() before accessing.
type AgenticContext struct {
	// LLM is the LLM provider for this agent run.
	LLM ports.LLMProvider
	// Tools are the tools available to this agent.
	Tools []ports.Tool
	// Frame is the execution frame (tenant, goal, permissions, budget).
	Frame ports.Frame
	// Budget is the budget for this run (enforced at port boundary).
	Budget ports.Budget
	// Redactor redacts PII from prompts and responses before they enter
	// any log or journal (ADR-066).
	Redactor *observability.Redactor
	// Tracer is the OTel tracer for genai.* spans (ADR-068).
	Tracer ports.Tracer
	// Journal is the journal store for agent.* events.
	Journal ports.JournalStore
	// Clock for timestamps.
	Clock ports.Clock
	// IDGenerator for event IDs.
	IDGenerator ports.IDGenerator
	// TenantID is the tenant scope for this run.
	TenantID ports.TenantID
}

// AgenticHandler is the handler signature for agentic behaviors.
// It receives the event and the AgenticContext, and returns HandlerOutput
// (which may contain proposal notes for privileged mutations).
type AgenticHandler func(ctx context.Context, event ports.RawEvent, agent *AgenticContext) (HandlerOutput, error)

// Kind returns the behavior kind string.
// For agentic behaviors it returns KindAgentic; for v1 behaviors it returns
// KindDeterministic.
func (b Behavior) Kind() string {
	if b.Kind_ == "" {
		return KindDeterministic
	}
	return b.Kind_
}

// IsAgentic returns true if this behavior is an agentic (LLM-driven) behavior.
func (b Behavior) IsAgentic() bool {
	return b.Kind() == KindAgentic
}

// AgenticHandler returns the behavior's agentic handler if it is agentic.
// Returns nil if the behavior is not agentic.
func (b Behavior) AgenticHandler() AgenticHandler {
	if !b.IsAgentic() {
		return nil
	}
	return b.AgenticH
}
