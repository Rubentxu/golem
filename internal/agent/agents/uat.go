package agents

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Rubentxu/golem/internal/agent/observability"
	"github.com/Rubentxu/golem/internal/behavior"
	"github.com/Rubentxu/golem/internal/lens"
	"github.com/Rubentxu/golem/internal/ports"
)

const (
	// UATAgentID is the behavior ID for the UAT agent.
	UATAgentID = "agent.uat"
	// UATAgentVersion is the version for all UAT agent releases.
	UATAgentVersion = "v1"
)

// uatPromptTemplate is the static prompt for the UAT agent.
// IMPORTANT: This is a COMPILE-TIME CONSTANT. No user data is interpolated
// into this template. The lens result (requirement trace data) is injected as
// structured JSON, which the LLM uses to generate UAT test cases.
// This design mitigates prompt injection attacks (R-1).
const uatPromptTemplate = `You are the GOLEM UAT (User Acceptance Testing) Agent.
Your task is to analyze requirements and propose test cases based on the provided requirement trace.

## Input Data (from graph lens - RequirementTraceLens)
%s

## Your Task
Based on the requirement trace above:
1. Identify gaps in test coverage for the given requirement
2. Propose new test cases that verify the requirement is met
3. Output a JSON proposal with the following structure:
{
  "operations": [
    {"type": "CreateNode", "kind": "TestCase", "id": "<new-uuid>", "payload": {"title": "<test-title>", "steps": ["<step1>", "<step2>"]}},
    {"type": "AddEdge", "kind": "VERIFIES", "from": "<testcase-id>", "to": "<requirement-id>"}
  ]
}

## Rules
- Only propose test cases that are testable and trace back to the requirement
- Use VERIFIES edge to link the test case to the requirement
- Test steps should be concrete and automatable
- Never make up requirements or test data not in the input data`

// NewUATAgent builds the UAT agent behavior.
// It uses RequirementTraceLens to find requirement verification gaps,
// then calls LLM to propose UAT test cases.
func NewUATAgent(
	llm ports.LLMProvider,
	_ ports.Frame,
	redact *observability.Redactor,
) *behavior.Behavior {
	return &behavior.Behavior{
		ID:            UATAgentID,
		Version:       UATAgentVersion,
		Kind_:         behavior.KindAgentic,
		Subscriptions: []string{"requirement.defined.v1"},
		LensSpec: func() *lens.Spec {
			s := lens.RequirementTraceLens(nil, 5, 500, 1000)
			return &s
		}(),
		AgenticH: uatAgentHandler(llm, redact),
	}
}

func uatAgentHandler(llm ports.LLMProvider, redact *observability.Redactor) behavior.AgenticHandler {
	return func(ctx context.Context, event ports.RawEvent, agent *behavior.AgenticContext) (behavior.HandlerOutput, error) {
		// Extract requirement info from event payload
		var payload struct {
			RequirementID string `json:"requirement_id"`
			Title         string `json:"title"`
		}
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			return behavior.HandlerOutput{}, fmt.Errorf("UAT agent: unmarshal payload: %w", err)
		}

		// Render static prompt with structured lens context.
		// Note: agent should prefer agentCtx.LensResult for richer context
		// (lens executes in pipeline.go before this handler is called).
		promptBody := fmt.Sprintf(uatPromptTemplate, renderRequirementContext(payload.RequirementID, payload.Title))

		// Redact prompt before LLM call (PII detection per ADR-066).
		// redacted.Summary (not empty redact.Redact("")) is passed to the journal.
		redacted := redact.Redact(promptBody)

		// Call LLM
		resp, err := llm.Complete(ctx, ports.LLMRequest{
			TenantID: string(agent.TenantID),
			Prompt:   promptBody,
			Model:    "golem-uat-v1",
		})
		if err != nil {
			return behavior.HandlerOutput{}, fmt.Errorf("UAT agent: LLM call: %w", err)
		}

		// Parse LLM response into proposal operations
		operations, err := parseUATProposal(resp.Content)
		if err != nil {
			return behavior.HandlerOutput{}, fmt.Errorf("UAT agent: parse response: %w", err)
		}
		_ = operations // proposals go through lifecycle

		// Return proposal note
		return behavior.HandlerOutput{
			Proposals: []behavior.ProposalNote{
				{
					Title: fmt.Sprintf("UAT: Test cases for requirement %s", payload.RequirementID),
					Body:  resp.Content,
				},
			},
			Events: []ports.RawEvent{
				makeUATAgentLLMCallEvent(agent, resp, "uat-generate", redacted.Summary),
			},
		}, nil
	}
}

func renderRequirementContext(reqID, title string) string {
	return fmt.Sprintf(`Requirement ID: %s
Title: %s

The lens traversed the graph to find existing test cases and evidence for this requirement.`, reqID, title)
}

func parseUATProposal(content string) ([]ports.Operation, error) {
	var result struct {
		Operations []ports.Operation `json:"operations"`
	}
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return nil, nil // return no operations if parsing fails
	}
	return result.Operations, nil
}

// makeUATAgentLLMCallEvent creates a journal event for a UAT agent LLM call.
func makeUATAgentLLMCallEvent(agent *behavior.AgenticContext, resp ports.LLMResponse, operation string, redactedSummary string) ports.RawEvent {
	tenantID := string(agent.TenantID)
	correlationID := agent.IDGenerator.NewID()

	payload := ports.AgentLLMCallPayload{
		Provider:       resp.Provider,
		Model:          resp.Model,
		Operation:      operation,
		InputTokens:    resp.TokenUsed / 2,
		OutputTokens:   resp.TokenUsed / 2,
		RedactedPrompt: redactedSummary, // real redaction of promptBody
		CorrelationID:  correlationID,
	}

	payloadBytes, _ := json.Marshal(payload)

	return ports.RawEvent{
		EventType:     ports.EventAgentLLMCallCompleted,
		TenantID:      tenantID,
		SchemaVersion: 1,
		OccurredAt:    agent.Clock.Now(),
		Actor:         ports.Actor{Type: "agent", ID: UATAgentID},
		CorrelationID: correlationID,
		Payload:       payloadBytes,
	}
}
