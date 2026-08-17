// Package planning defines the Planning bounded context: backlog,
// iterations (sprints), milestones and boards (BOUNDED_CONTEXTS).
package planning

import "time"

// IterationCreated is the payload of planning.iteration.created.v1.
type IterationCreated struct {
	IterationID string    `json:"iteration_id"`
	Name        string    `json:"name"`
	Start       time.Time `json:"start"`
	End         time.Time `json:"end"`
}

// MilestoneCreated is the payload of planning.milestone.created.v1.
type MilestoneCreated struct {
	MilestoneID string    `json:"milestone_id"`
	Name        string    `json:"name"`
	TargetDate  time.Time `json:"target_date"`
}

// Event type names of this context.
const (
	EventIterationCreated = "planning.iteration.created.v1"
	EventMilestoneCreated = "planning.milestone.created.v1"
)
