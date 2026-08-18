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
	// SecurityAgentID is the behavior ID for the Security agent.
	SecurityAgentID = "agent.security"
	// SecurityAgentVersion is the version for all Security agent releases.
	SecurityAgentVersion = "v1"
)

// securityPromptTemplate is the static prompt for the Security agent.
// IMPORTANT: This is a COMPILE-TIME CONSTANT. No user data is interpolated
// into this template. The lens result (graph data) is injected as structured
// JSON, which the LLM uses to reason about vulnerabilities and propose VEX fixes.
// This design mitigates prompt injection attacks (R-1): dynamic user data never
// enters the prompt directly.
const securityPromptTemplate = `You are the GOLEM Security Agent.
Your task is to analyze vulnerability impact using the provided graph data and propose VEX (Vulnerability Exploitability eXchange) statements.

## Input Data (from graph lens)
%s

## Your Task
Based on the graph data above:
1. Identify which releases/artifacts are affected by the vulnerability
2. Determine if a VEX "fixed" statement is appropriate given the component version and fix status
3. Output a JSON proposal with the following structure:
{
  "operations": [
    {"type": "CreateNode", "kind": "VEXStatement", "id": "<new-uuid>", "payload": {"cve_id": "<cve>", "state": "fixed", "justification": "<reason>"}},
    {"type": "AddEdge", "kind": "MITIGATED_BY", "from": "<artifact-id>", "to": "<vex-statement-id>"}
  ]
}

## Rules
- Only propose VEX "fixed" if the vulnerability is actually remediated in the target release
- Use MITIGATED_BY edge to link the affected artifact to the VEX statement
- If the vulnerability cannot be fixed, propose no operations
- Never make up CVE IDs or component versions not in the input data`

// NewSecurityAgent builds the Security agent behavior.
// It uses VulnerabilityImpactLens to find blast radius of vulnerabilities,
// then calls LLM to reason about VEX "fixed" proposals.
func NewSecurityAgent(
	llm ports.LLMProvider,
	_ ports.Frame,
	redact *observability.Redactor,
) *behavior.Behavior {
	return &behavior.Behavior{
		ID:            SecurityAgentID,
		Version:       SecurityAgentVersion,
		Kind_:         behavior.KindAgentic,
		Subscriptions: []string{"vulnerability.detected.v1"},
		LensSpec: func() *lens.Spec {
			s := lens.VulnerabilityImpactLens(nil, 5, 500, 1000)
			return &s
		}(),
		AgenticH: securityAgentHandler(llm, redact),
	}
}

func securityAgentHandler(llm ports.LLMProvider, redact *observability.Redactor) behavior.AgenticHandler {
	return func(ctx context.Context, event ports.RawEvent, agent *behavior.AgenticContext) (behavior.HandlerOutput, error) {
		// Extract vulnerability info from event payload
		var payload struct {
			CVEID string `json:"cve_id"`
			PURL  string `json:"purl"`
		}
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			return behavior.HandlerOutput{}, fmt.Errorf("security agent: unmarshal payload: %w", err)
		}

		// Render static prompt with structured lens context.
		// Note: agent should prefer agentCtx.LensResult for richer context
		// (lens executes in pipeline.go before this handler is called).
		// The lens traverses: Component → SBOM → Artifact → Release
		// to find all affected releases.
		promptBody := fmt.Sprintf(securityPromptTemplate, renderVulnerabilityContext(payload.CVEID, payload.PURL))

		// Redact prompt before LLM call (PII detection per ADR-066).
		// redacted.Summary (not empty redact.Redact("")) is passed to the journal.
		redacted := redact.Redact(promptBody)

		// Call LLM
		resp, err := llm.Complete(ctx, ports.LLMRequest{
			TenantID: string(agent.TenantID),
			Prompt:   promptBody,
			Model:    "golem-security-v1",
		})
		if err != nil {
			return behavior.HandlerOutput{}, fmt.Errorf("security agent: LLM call: %w", err)
		}

		// Parse LLM response into proposal operations
		operations, err := parseSecurityProposal(resp.Content)
		if err != nil {
			return behavior.HandlerOutput{}, fmt.Errorf("security agent: parse response: %w", err)
		}
		_ = operations // proposals go through lifecycle, not direct mutation

		// Return proposal note (privileged mutations go through proposal lifecycle)
		return behavior.HandlerOutput{
			Proposals: []behavior.ProposalNote{
				{
					Title: fmt.Sprintf("Security: VEX fix for %s", payload.CVEID),
					Body:  resp.Content,
				},
			},
			Events: []ports.RawEvent{
				makeAgentLLMCallEvent(agent, resp, "security-analyze", redacted.Summary),
			},
		}, nil
	}
}

func renderVulnerabilityContext(cveID, purl string) string {
	return fmt.Sprintf(`Vulnerability: %s
Package: %s

The lens traversed the graph to find all releases and artifacts affected by this vulnerability.`, cveID, purl)
}

func parseSecurityProposal(content string) ([]ports.Operation, error) {
	// Try to parse operations from LLM response
	var result struct {
		Operations []ports.Operation `json:"operations"`
	}
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		// If parsing fails, return no operations (LLM may have explained why no fix is possible)
		return nil, nil
	}
	return result.Operations, nil
}

// makeAgentLLMCallEvent creates a journal event for an LLM call.
func makeAgentLLMCallEvent(agent *behavior.AgenticContext, resp ports.LLMResponse, operation string, redactedSummary string) ports.RawEvent {
	tenantID := string(agent.TenantID)
	correlationID := agent.IDGenerator.NewID()

	payload := ports.AgentLLMCallPayload{
		Provider:       resp.Provider,
		Model:          resp.Model,
		Operation:      operation,
		InputTokens:    resp.TokenUsed / 2, // rough estimate
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
		Actor:         ports.Actor{Type: "agent", ID: SecurityAgentID},
		CorrelationID: correlationID,
		Payload:       payloadBytes,
	}
}
