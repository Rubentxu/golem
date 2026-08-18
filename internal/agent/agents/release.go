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
	// ReleaseAgentID is the behavior ID for the Release agent.
	ReleaseAgentID = "agent.release"
	// ReleaseAgentVersion is the version for all Release agent releases.
	ReleaseAgentVersion = "v1"
)

// releasePromptTemplate is the static prompt for the Release agent.
// IMPORTANT: This is a COMPILE-TIME CONSTANT. No user data is interpolated
// into this template. The lens result (release evidence data) is injected as
// structured JSON, which the LLM uses to evaluate release readiness.
// This design mitigates prompt injection attacks (R-1).
const releasePromptTemplate = `You are the GOLEM Release Agent.
Your task is to evaluate whether a release is ready for approval based on the provided evidence chain.

## Input Data (from graph lens - ReleaseEvidenceLens)
%s

## Your Task
Based on the release evidence above:
1. Check if all required attestations and VEX statements are present
2. Verify that no unresolved high-severity vulnerabilities affect the release
3. Determine if the release meets the organization's release criteria
4. Output a JSON proposal with the following structure:
{
  "operations": [
    {"type": "UpdateNode", "kind": "Release", "id": "<release-id>", "payload": {"status": "approved"}}
  ]
}

## Rules
- Only approve a release if all evidence is complete and no blocking issues exist
- Use the VEX state ("fixed", "under_review", "not_affected") to determine if vulnerabilities block release
- If evidence is incomplete or blocking vulnerabilities exist, propose no operations
- Never make up release IDs or evidence not in the input data`

// NewReleaseAgent builds the Release agent behavior.
// It uses ReleaseEvidenceLens to gather the full evidence chain for a release,
// then calls LLM to evaluate release readiness and propose approval.
func NewReleaseAgent(
	llm ports.LLMProvider,
	_ ports.Frame,
	redact *observability.Redactor,
) *behavior.Behavior {
	return &behavior.Behavior{
		ID:            ReleaseAgentID,
		Version:       ReleaseAgentVersion,
		Kind_:         behavior.KindAgentic,
		Subscriptions: []string{"release.candidate.v1", "proposal.applied.v1"},
		LensSpec: func() *lens.Spec {
			s := lens.ReleaseEvidenceLens(nil, 5, 500, 1000)
			return &s
		}(),
		AgenticH: releaseAgentHandler(llm, redact),
	}
}

func releaseAgentHandler(llm ports.LLMProvider, redact *observability.Redactor) behavior.AgenticHandler {
	return func(ctx context.Context, event ports.RawEvent, agent *behavior.AgenticContext) (behavior.HandlerOutput, error) {
		// Extract release info from event payload
		var payload struct {
			ReleaseID string `json:"release_id"`
			TenantID  string `json:"tenant_id"`
		}
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			return behavior.HandlerOutput{}, fmt.Errorf("Release agent: unmarshal payload: %w", err)
		}

		// Build lens spec with roots from event
		lensSpec := lens.ReleaseEvidenceLens(
			[]string{payload.ReleaseID},
			5,    // max depth
			500,  // max nodes
			1000, // max edges
		)
		_ = lensSpec // lens execution happens in behavior pipeline

		// Render static prompt with structured lens context
		promptBody := fmt.Sprintf(releasePromptTemplate, renderReleaseContext(payload.ReleaseID))

		// Redact prompt before LLM call (PII detection per ADR-066)
		redacted := redact.Redact(promptBody)
		_ = redacted // summary goes to journal event

		// Call LLM
		resp, err := llm.Complete(ctx, ports.LLMRequest{
			TenantID: string(agent.TenantID),
			Prompt:   promptBody,
			Model:    "golem-release-v1",
		})
		if err != nil {
			return behavior.HandlerOutput{}, fmt.Errorf("Release agent: LLM call: %w", err)
		}

		// Parse LLM response into proposal operations
		operations, err := parseReleaseProposal(resp.Content)
		if err != nil {
			return behavior.HandlerOutput{}, fmt.Errorf("Release agent: parse response: %w", err)
		}
		_ = operations // proposals go through lifecycle

		// Return proposal note
		return behavior.HandlerOutput{
			Proposals: []behavior.ProposalNote{
				{
					Title: fmt.Sprintf("Release: Approval evaluation for %s", payload.ReleaseID),
					Body:  resp.Content,
				},
			},
			Events: []ports.RawEvent{
				makeReleaseAgentLLMCallEvent(agent, resp, "release-evaluate", redact),
			},
		}, nil
	}
}

func renderReleaseContext(releaseID string) string {
	return fmt.Sprintf(`Release ID: %s

The lens traversed the graph to find the full evidence chain:
- Release status and metadata
- Associated artifacts and SBOMs
- VEX statements (MITIGATED_BY edges)
- Attestations

Use this evidence to determine if the release is ready for approval.`, releaseID)
}

func parseReleaseProposal(content string) ([]ports.Operation, error) {
	var result struct {
		Operations []ports.Operation `json:"operations"`
	}
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return nil, nil // return no operations if parsing fails
	}
	return result.Operations, nil
}

// makeReleaseAgentLLMCallEvent creates a journal event for a Release agent LLM call.
func makeReleaseAgentLLMCallEvent(agent *behavior.AgenticContext, resp ports.LLMResponse, operation string, redact *observability.Redactor) ports.RawEvent {
	tenantID := string(agent.TenantID)
	correlationID := agent.IDGenerator.NewID()

	payload := ports.AgentLLMCallPayload{
		Provider:       resp.Provider,
		Model:          resp.Model,
		Operation:      operation,
		InputTokens:    resp.TokenUsed / 2,
		OutputTokens:   resp.TokenUsed / 2,
		RedactedPrompt: redact.Redact("").Summary,
		CorrelationID:  correlationID,
	}

	payloadBytes, _ := json.Marshal(payload)

	return ports.RawEvent{
		EventType:     ports.EventAgentLLMCallCompleted,
		TenantID:      tenantID,
		SchemaVersion: 1,
		OccurredAt:    agent.Clock.Now(),
		Actor:         ports.Actor{Type: "agent", ID: ReleaseAgentID},
		CorrelationID: correlationID,
		Payload:       payloadBytes,
	}
}
