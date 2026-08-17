// Package planning hosts the application handlers of the Planning
// bounded context.
package planning

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	appcmd "github.com/Rubentxu/golem/internal/application/command"
	domainplanning "github.com/Rubentxu/golem/internal/planning"
	"github.com/Rubentxu/golem/internal/ports"
)

// Domain validation errors of the Planning context.
var (
	ErrEmptyName    = errors.New("planning: name is mandatory")
	ErrInvalidRange = errors.New("planning: end must not precede start")
)

// Command names of this context.
const (
	CmdCreateIteration = "planning.create-iteration"
	CmdCreateMilestone = "planning.create-milestone"
)

// CreateIteration is the payload of CmdCreateIteration.
type CreateIteration struct {
	Name  string    `json:"name"`
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// CreateMilestone is the payload of CmdCreateMilestone.
type CreateMilestone struct {
	Name       string    `json:"name"`
	TargetDate time.Time `json:"target_date"`
}

// CreateIterationHandler returns the handler for CmdCreateIteration.
func CreateIterationHandler(gen ports.IDGenerator) appcmd.Handler {
	return func(_ context.Context, cmd appcmd.Command) ([]appcmd.EventDraft, error) {
		p, ok := cmd.Payload.(CreateIteration)
		if !ok {
			return nil, errors.New("planning: payload must be planning.CreateIteration")
		}
		name := strings.TrimSpace(p.Name)
		if name == "" {
			return nil, ErrEmptyName
		}
		if !p.End.IsZero() && !p.Start.IsZero() && p.End.Before(p.Start) {
			return nil, ErrInvalidRange
		}
		id := gen.NewID()
		return []appcmd.EventDraft{{
			EventType:     domainplanning.EventIterationCreated,
			StreamID:      "iteration:" + id,
			SchemaVersion: 1,
			Payload:       domainplanning.IterationCreated{IterationID: id, Name: name, Start: p.Start, End: p.End},
		}}, nil
	}
}

// CreateMilestoneHandler returns the handler for CmdCreateMilestone.
func CreateMilestoneHandler(gen ports.IDGenerator) appcmd.Handler {
	return func(_ context.Context, cmd appcmd.Command) ([]appcmd.EventDraft, error) {
		p, ok := cmd.Payload.(CreateMilestone)
		if !ok {
			return nil, errors.New("planning: payload must be planning.CreateMilestone")
		}
		name := strings.TrimSpace(p.Name)
		if name == "" {
			return nil, fmt.Errorf("%w: name is mandatory", ErrEmptyName)
		}
		id := gen.NewID()
		return []appcmd.EventDraft{{
			EventType:     domainplanning.EventMilestoneCreated,
			StreamID:      "milestone:" + id,
			SchemaVersion: 1,
			Payload:       domainplanning.MilestoneCreated{MilestoneID: id, Name: name, TargetDate: p.TargetDate},
		}}, nil
	}
}
