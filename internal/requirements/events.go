// Package requirements defines the Requirements bounded context:
// requirement statements and their baselines (BOUNDED_CONTEXTS). The
// Requirement→Work traceability of M2 is carried by canonical IMPLEMENTS
// edges created through the work link command.
package requirements

// RequirementCreated is the payload of requirements.requirement.created.v1.
type RequirementCreated struct {
	RequirementID string `json:"requirement_id"`
	Title         string `json:"title"`
	Statement     string `json:"statement"`
	Status        string `json:"status"`
}

// Event type names of this context.
const (
	EventRequirementCreated = "requirements.requirement.created.v1"
)
