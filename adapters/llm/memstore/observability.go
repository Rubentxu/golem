// Package memstore provides a deterministic LLM adapter for testing and replay.
package memstore

import (
	"context"
	"encoding/json"
	"time"

	"github.com/Rubentxu/golem/adapters/observability/otel"
	"github.com/Rubentxu/golem/internal/agent/observability"
	"github.com/Rubentxu/golem/internal/ports"
)

// ObservableComplete wraps LLMStore.Complete with OTel GenAI spans (ADR-068)
// and a redacted agent.llm.call.completed.v1 journal event (ADR-066).
//
// The correlation_id links the span to the journal event (ADR-019).
//
// Parameters:
//   - ctx: should carry ports.CorrelationFrom(ctx).CorrelationID for linkability.
//   - tracer: ports.Tracer (e.g. otel.Tracer from adapters/observability/otel).
//   - journal: ports.JournalStore to append the agent.llm.call.completed.v1 event.
//   - clk: ports.Clock for timestamps.
//   - idgen: ports.IDGenerator for event ID derivation.
//   - tenantID, actor: for the journal envelope.
func ObservableComplete(
	ctx context.Context,
	store *LLMStore,
	req ports.LLMRequest,
	tracer ports.Tracer,
	journal ports.JournalStore,
	clk ports.Clock,
	idgen ports.IDGenerator,
	tenantID string,
	actor ports.Actor,
) (ports.LLMResponse, error) {
	// Extract correlation_id from context if present (ADR-019).
	correlationID := ""
	if corr, ok := ports.CorrelationFrom(ctx); ok {
		correlationID = corr.CorrelationID
	}

	start := clk.Now()

	// Start OTel GenAI LLM span (ADR-068).
	ctx, span := otel.StartLLMSpan(ctx, otel.LLMStartInput{
		Tracer:        tracer,
		CorrelationID: correlationID,
		System:        "memstore",
		Model:         req.Model,
		OperationName: "complete",
	})

	// Redact prompt before it enters any log or span attribute (ADR-066).
	promptSummary := observability.NewRedactor().Redact(req.Prompt)

	resp, err := store.Complete(ctx, req)

	latencyMs := int64(time.Since(start).Milliseconds())
	inputTokens := 0
	outputTokens := 0
	if err == nil {
		inputTokens = estimateInputTokens(req.Prompt)
		outputTokens = resp.TokenUsed
	}

	// End OTel span (ADR-068) — never record raw content.
	otel.EndLLMSpan(otel.LLMEndInput{
		Span:          span,
		InputTokens:   inputTokens,
		OutputTokens:  outputTokens,
		FinishReasons: finishReasons(err, resp),
		Err:           err,
	})

	// Emit journal event with redacted summary (ADR-066).
	if journal != nil && idgen != nil {
		payload := ports.AgentLLMCallPayload{
			Provider:       "memstore",
			Model:          req.Model,
			Operation:      "complete",
			InputTokens:    inputTokens,
			OutputTokens:   outputTokens,
			LatencyMs:      latencyMs,
			RedactedPrompt: promptSummary.Summary,
			CorrelationID:  correlationID,
		}
		emitAgentLLMEvent(ctx, journal, clk, idgen, tenantID, actor, correlationID, payload)
	}

	return resp, err
}

// emitAgentLLMEvent appends an agent.llm.call.completed.v1 event to the journal.
// Errors are logged but not returned — journal is best-effort.
func emitAgentLLMEvent(
	ctx context.Context,
	journal ports.JournalStore,
	clk ports.Clock,
	idgen ports.IDGenerator,
	tenantID string,
	actor ports.Actor,
	correlationID string,
	payload ports.AgentLLMCallPayload,
) {
	data, err := json.Marshal(payload)
	if err != nil {
		return // best-effort
	}
	env := ports.RawEvent{
		EventID:       idgen.NewID(),
		TenantID:      tenantID,
		EventType:     ports.EventAgentLLMCallCompleted,
		SchemaVersion: 1,
		OccurredAt:    clk.Now(),
		Actor:         actor,
		CorrelationID: correlationID,
		Payload:       data,
	}
	_, _ = journal.Append(ctx, []ports.RawEvent{env}) // best-effort
}

// estimateInputTokens approximates token count from word count (words * 1.3).
func estimateInputTokens(prompt string) int {
	words := 0
	inWord := false
	for _, r := range prompt {
		if r == ' ' || r == '\n' || r == '\t' {
			inWord = false
		} else if !inWord {
			words++
			inWord = true
		}
	}
	return int(float64(words) * 1.3)
}

// finishReasons returns finish reason strings from error or response.
func finishReasons(err error, resp ports.LLMResponse) []string {
	if err != nil {
		return []string{"error"}
	}
	if resp.Content != "" {
		return []string{"stop"}
	}
	return []string{"tool_calls"}
}
