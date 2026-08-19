// Package work hosts the application handlers of the Work bounded context.
package work

import "context"

// WorkItemReader is the narrow port for reading work items.
type WorkItemReader interface {
	// GetTypeDef returns the WorkType definition for the given type name.
	// Returns ErrWorkTypeNotFound if the type does not exist.
	GetTypeDef(ctx context.Context, tenant, typeName string) (WorkTypeDef, error)
}

// WorkItemWriter is the narrow port for writing work items.
// Commands that append to the own-stream (AddComment, etc.) use this to
// preserve the ADR-101 atomic journal path.
type WorkItemWriter interface {
	// AppendCommand appends a command to the work item's own stream.
	AppendCommand(ctx context.Context, cmd WorkItemCommand) error
}

// WorkItemCommand is a command to append to a work item's own stream.
type WorkItemCommand struct {
	// TenantID is the tenant scope.
	TenantID string
	// ItemID is the target work item.
	ItemID string
	// Name is the command name (e.g. "work.add-comment").
	Name string
	// Payload is the JSON-encoded command payload.
	Payload []byte
}

// WorkTypeDef describes a work type's states, transitions, and custom fields.
type WorkTypeDef struct {
	Name        string
	Initial     string
	States      []string
	Transitions []string
	Fields      []string
}
