// Package projects defines the Projects bounded context: project
// spaces and their configuration (BOUNDED_CONTEXTS).
package projects

import "github.com/Rubentxu/golem/internal/work"

// ProjectCreated is the payload of projects.project.created.v1.
type ProjectCreated struct {
	ProjectID   string                `json:"project_id"`
	Name        string                `json:"name"`
	Description string                `json:"description"`
	External    work.ExternalIdentity `json:"external,omitempty"`
}

// Event type names of this context.
const (
	EventProjectCreated = "projects.project.created.v1"
)
