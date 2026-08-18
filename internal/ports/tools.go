package ports

import (
	"context"
	"encoding/json"
	"errors"
)

// Tool port errors (ADR-062).
var (
	// ErrUnknownPermission is returned when a permission is not in the closed catalog.
	ErrUnknownPermission = errors.New("ports: unknown tool permission")
	// ErrInvalidToolInput is returned when the tool input is malformed.
	ErrInvalidToolInput = errors.New("ports: invalid tool input")
	// ErrToolInvocation is returned when a tool invocation fails.
	ErrToolInvocation = errors.New("ports: tool invocation failed")
)

// ToolSpec describes a tool's interface (ADR-062).
type ToolSpec struct {
	ID          string          `json:"id"`
	Version     string          `json:"version"`
	Permissions []Permission    `json:"permissions"` // subset of closed Permission catalog
	Schema      json.RawMessage `json:"schema"`      // JSON-Schema for Input/Output
}

// ToolInput is the input passed to a tool invocation (ADR-062, C4).
type ToolInput struct {
	TenantID     TenantID        `json:"tenant_id"`
	FrameID      string          `json:"frame_id"`
	Arguments    json.RawMessage `json:"arguments"`
	EvidenceRefs []string        `json:"evidence_refs,omitempty"`
}

// ToolOutput is the output from a tool invocation (ADR-062, C4).
type ToolOutput struct {
	Result          json.RawMessage `json:"result,omitempty"`
	EvidenceRefs    []string        `json:"evidence_refs,omitempty"`
	RedactedSummary string          `json:"redacted_summary,omitempty"`
}

// Tool is the port for tool/function calling (ADR-062).
type Tool interface {
	ID() string
	Version() string
	Schema() json.RawMessage
	Permissions() []Permission
	Invoke(context.Context, ToolInput) (ToolOutput, error)
}
