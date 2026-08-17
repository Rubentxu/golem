// Package requirements hosts the application handlers of the
// Requirements bounded context.
package requirements

import (
	"context"
	"errors"
	"strings"

	appcmd "github.com/Rubentxu/golem/internal/application/command"
	"github.com/Rubentxu/golem/internal/ports"
	domainreq "github.com/Rubentxu/golem/internal/requirements"
)

// Domain validation errors of the Requirements context.
var (
	ErrEmptyTitle = errors.New("requirements: title is mandatory")
)

// Command names of this context.
const (
	CmdCreateRequirement = "requirements.create-requirement"
)

// CreateRequirement is the payload of CmdCreateRequirement.
type CreateRequirement struct {
	Title     string `json:"title"`
	Statement string `json:"statement"`
}

// CreateRequirementHandler returns the handler for CmdCreateRequirement.
func CreateRequirementHandler(gen ports.IDGenerator) appcmd.Handler {
	return func(_ context.Context, cmd appcmd.Command) ([]appcmd.EventDraft, error) {
		p, ok := cmd.Payload.(CreateRequirement)
		if !ok {
			return nil, errors.New("requirements: payload must be requirements.CreateRequirement")
		}
		title := strings.TrimSpace(p.Title)
		if title == "" {
			return nil, ErrEmptyTitle
		}

		id := gen.NewID()
		return []appcmd.EventDraft{{
			EventType:     domainreq.EventRequirementCreated,
			StreamID:      "requirement:" + id,
			SchemaVersion: 1,
			Payload: domainreq.RequirementCreated{
				RequirementID: id,
				Title:         title,
				Statement:     strings.TrimSpace(p.Statement),
				Status:        "draft",
			},
		}}, nil
	}
}
