package ports

import (
	"context"
	"errors"
)

// Tool port errors (ADR-062).
var (
	// ErrUnknownPermission is returned when a permission is not in the closed catalog.
	ErrUnknownPermission = errors.New("ports: unknown tool permission")
	// ErrToolInvokeFailed is returned when a tool invocation fails.
	ErrToolInvokeFailed = errors.New("ports: tool invocation failed")
)

// ToolSpec describes a tool's interface (ADR-062).
type ToolSpec struct {
	Name        string          `json:"name"`
	Permissions []Permission    `json:"permissions"` // subset of closed Permission catalog
	Description string          `json:"description,omitempty"`
	InputSchema ToolInputSchema `json:"input_schema"`
}

// ToolInputSchema defines the input structure for a tool.
type ToolInputSchema struct {
	Type        string `json:"type"` // "object"
	Description string `json:"description,omitempty"`
}

// ToolInput is the input passed to a tool invocation.
type ToolInput struct {
	TenantID string         `json:"tenant_id"`
	ToolName string         `json:"tool_name"`
	Params   map[string]any `json:"params,omitempty"`
}

// ToolOutput is the output from a tool invocation.
type ToolOutput struct {
	TenantID string `json:"tenant_id"`
	ToolName string `json:"tool_name"`
	Result   any    `json:"result,omitempty"`
	Error    string `json:"error,omitempty"`
	Success  bool   `json:"success"`
}

// Tool is the port for tool/function calling (ADR-062).
// It wraps an invocation with permission checking and journal.
type Tool interface {
	// Invoke executes the tool with the given input.
	// It returns ToolOutput or an error if invocation fails.
	Invoke(ctx context.Context, input ToolInput) (ToolOutput, error)
	// Spec returns the tool specification.
	Spec() ToolSpec
}
