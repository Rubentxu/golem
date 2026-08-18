package canonical

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/Rubentxu/golem/internal/ports"
)

// AgentEvalNodeKind is the canonical node kind for agent evaluations.
const AgentEvalNodeKind = "AgentEval"

// AgentEvalReportNodeKind is the canonical node kind for agent eval reports.
const AgentEvalReportNodeKind = "AgentEvalReport"

// AgentRunNodeKind is the canonical node kind for agent runs.
const AgentRunNodeKind = "AgentRun"

// EdgeTypeEVALUATED is the canonical edge type from AgentEval to Behavior.
const EdgeTypeEVALUATED = "EVALUATED"

// EdgeTypeOBSERVED is the canonical edge type from AgentEval to Proposal.
const EdgeTypeOBSERVED = "OBSERVED"

// AgentEvalPayload is the payload of agent.eval.completed.v1 events.
type AgentEvalPayload struct {
	EvalID           string  `json:"eval_id"`
	TenantID         string  `json:"tenant_id"`
	BehaviorID       string  `json:"behavior_id"`
	RunSeq           uint64  `json:"run_seq"`
	Outcome          string  `json:"outcome"` // "pass", "fail", "error"
	Rationale        string  `json:"rationale"`
	CostUSD          float64 `json:"cost_usd"`
	LatencyMs        uint64  `json:"latency_ms"`
	ProposalID       string  `json:"proposal_id,omitempty"`
	PolicyViolations int     `json:"policy_violations"`
}

// AgentEvalProjector projects agent.eval.completed.v1 journal events into
// AgentEval graph nodes and edges (ADR-067). ID derivation:
// sha256(behavior_id + tenant_id + run_seq)[:8].
type AgentEvalProjector struct {
	graph ports.GraphStore
}

// NewAgentEvalProjector creates a projector for AgentEval nodes and edges.
func NewAgentEvalProjector(graph ports.GraphStore) *AgentEvalProjector {
	return &AgentEvalProjector{graph: graph}
}

// Project applies an agent.eval.completed.v1 event to the graph.
// It creates an AgentEval node and edges: EVALUATED → Behavior,
// OBSERVED → Proposal (if proposal_id is present).
// Duplicate events (same eval_id) are idempotent — they do not create
// duplicate nodes (ESC-031).
func (p *AgentEvalProjector) Project(ctx context.Context, env ports.RawEvent) error {
	if env.EventType != ports.EventAgentEvalCompleted {
		return nil // not handled by this projector
	}

	var payload AgentEvalPayload
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		return fmt.Errorf("agent eval projector: unmarshal payload: %w", err)
	}

	nodeID := deriveAgentEvalID(payload.BehaviorID, payload.TenantID, payload.RunSeq)

	// Build node attributes
	nodeAttrs := map[string]any{
		"eval_id":           payload.EvalID,
		"behavior_id":       payload.BehaviorID,
		"tenant_id":         payload.TenantID,
		"run_seq":           payload.RunSeq,
		"outcome":           payload.Outcome,
		"cost_usd":          payload.CostUSD,
		"latency_ms":        payload.LatencyMs,
		"policy_violations": payload.PolicyViolations,
		"proposal_id":       payload.ProposalID,
	}

	var ops []ports.GraphOp

	// Upsert the AgentEval node
	ops = append(ops, ports.GraphOp{
		Kind:   ports.OpUpsertNode,
		Target: nodeID,
		Data: map[string]any{
			"kind":       AgentEvalNodeKind,
			"attributes": nodeAttrs,
		},
	})

	// Edge: AgentEval → Behavior (EVALUATED)
	behaviorEdgeID := deriveEdgeID(nodeID, payload.BehaviorID, EdgeTypeEVALUATED)
	ops = append(ops, ports.GraphOp{
		Kind:   ports.OpUpsertEdge,
		Target: behaviorEdgeID,
		Data: map[string]any{
			"type":       EdgeTypeEVALUATED,
			"source":     nodeID,
			"target":     payload.BehaviorID,
			"attributes": map[string]any{"eval_id": payload.EvalID},
		},
	})

	// Edge: AgentEval → Proposal (OBSERVED) — only if proposal_id is present
	if payload.ProposalID != "" {
		proposalEdgeID := deriveEdgeID(nodeID, payload.ProposalID, EdgeTypeOBSERVED)
		ops = append(ops, ports.GraphOp{
			Kind:   ports.OpUpsertEdge,
			Target: proposalEdgeID,
			Data: map[string]any{
				"type":       EdgeTypeOBSERVED,
				"source":     nodeID,
				"target":     payload.ProposalID,
				"attributes": map[string]any{"eval_id": payload.EvalID},
			},
		})
	}

	tx := ports.GraphMutation{
		TenantID: ports.TenantID(payload.TenantID),
		Ops:      ops,
	}

	if _, err := p.graph.Apply(ctx, tx); err != nil {
		return fmt.Errorf("agent eval projector: apply: %w", err)
	}

	return nil
}

// deriveAgentEvalID computes the deterministic ID for an AgentEval node.
func deriveAgentEvalID(behaviorID, tenantID string, runSeq uint64) string {
	h := sha256.New()
	h.Write([]byte(behaviorID + tenantID + fmt.Sprintf("%d", runSeq)))
	return hex.EncodeToString(h.Sum(nil))[:8]
}

// deriveEdgeID computes the deterministic ID for an edge.
func deriveEdgeID(sourceID, targetID, edgeType string) string {
	h := sha256.New()
	h.Write([]byte(sourceID + targetID + edgeType))
	return hex.EncodeToString(h.Sum(nil))[:16]
}
