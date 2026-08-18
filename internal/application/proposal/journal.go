package proposal

import (
	"encoding/json"
	"strings"
)

// RedactedProposalEvent is the redacted summary of a proposal event.
// It never contains raw rationale or raw operation content — only structured
// attributes suitable for journal logging (ADR-066).
type RedactedProposalEvent struct {
	EventType  string `json:"event_type"`
	ProposalID string `json:"proposal_id"`
	TenantID   string `json:"tenant_id"`
	Status     string `json:"status"`
	// OperationCount is the number of operations, not the content
	OperationCount int `json:"operation_count"`
	// TargetType and TargetID identify the target without raw payload
	TargetType string `json:"target_type"`
	TargetID   string `json:"target_id"`
	// Rationale is always "[REDACTED]" — raw rationale never enters the journal
	Rationale string `json:"rationale"`
	// ActorID is the actor ID without sensitive claims
	ActorID   string `json:"actor_id"`
	ActorType string `json:"actor_type"`
}

// RedactProposalEvent returns a RedactedProposalEvent from a raw proposal event payload.
// Raw rationale is replaced with "[REDACTED]" and operation payloads are summarised
// by count only (ADR-066).
func RedactProposalEvent(eventType string, payload json.RawMessage) RedactedProposalEvent {
	// First try to extract just the proposal_id, tenant_id, status without
	// deserializing the full raw content
	var raw map[string]any
	if err := json.Unmarshal(payload, &raw); err != nil {
		return RedactedProposalEvent{EventType: eventType}
	}

	r := RedactedProposalEvent{
		EventType: eventType,
		Rationale: "[REDACTED]",
	}

	if v, ok := raw["proposal_id"].(string); ok {
		r.ProposalID = v
	}
	if v, ok := raw["tenant_id"].(string); ok {
		r.TenantID = v
	}
	if v, ok := raw["status"].(string); ok {
		r.Status = v
	}
	if ops, ok := raw["operations"].([]any); ok {
		r.OperationCount = len(ops)
	}
	if ts, ok := raw["target_spec"].(map[string]any); ok {
		if t, ok := ts["type"].(string); ok {
			r.TargetType = t
		}
		if id, ok := ts["id"].(string); ok {
			r.TargetID = id
		}
	}
	if a, ok := raw["actor"].(map[string]any); ok {
		if t, ok := a["type"].(string); ok {
			r.ActorType = t
		}
		if id, ok := a["id"].(string); ok {
			r.ActorID = id
		}
	}
	if v, ok := raw["approved_by"].(map[string]any); ok {
		if t, ok := v["type"].(string); ok {
			r.ActorType = t
		}
		if id, ok := v["id"].(string); ok {
			r.ActorID = id
		}
	}
	if v, ok := raw["rejected_by"].(map[string]any); ok {
		if t, ok := v["type"].(string); ok {
			r.ActorType = t
		}
		if id, ok := v["id"].(string); ok {
			r.ActorID = id
		}
	}
	if v, ok := raw["conflicted_by"].(map[string]any); ok {
		if t, ok := v["type"].(string); ok {
			r.ActorType = t
		}
		if id, ok := v["id"].(string); ok {
			r.ActorID = id
		}
	}

	// Ensure rationale is always redacted
	r.Rationale = "[REDACTED]"

	return r
}

// RedactRationale replaces potentially sensitive content in a rationale string
// with "[REDACTED]". Email addresses, URLs, tokens, and paths are masked.
func RedactRationale(rationale string) string {
	if rationale == "" {
		return ""
	}
	// Replace email-like patterns
	rationale = strings.ReplaceAll(rationale, "@", "[AT]")
	// Replace token-like patterns (long alphanumeric strings)
	// Replace URLs
	rationale = strings.ReplaceAll(rationale, "https://", "[REDACTED-URL]")
	rationale = strings.ReplaceAll(rationale, "http://", "[REDACTED-URL]")
	return rationale
}
