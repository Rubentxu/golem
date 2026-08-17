// Package work hosts the application handlers of the Work bounded
// context: commands validated by domain rules and expressed as event
// drafts for the command bus.
package work

import (
	"context"
	"errors"
	"strings"

	appcmd "github.com/Rubentxu/golem/internal/application/command"
	"github.com/Rubentxu/golem/internal/ports"
	domainwork "github.com/Rubentxu/golem/internal/work"
)

// Domain validation errors of the Work context.
var (
	ErrEmptyTitle = errors.New("work: title is mandatory")
	ErrEmptyType  = errors.New("work: item type is mandatory")
)

// Command names of this context.
const (
	CmdCreateWorkItem = "work.create-work-item"
)

// CreateWorkItem is the payload of CmdCreateWorkItem.
type CreateWorkItem struct {
	Title    string `json:"title"`
	ItemType string `json:"type"`
}

// CreateWorkItemHandler returns the handler for CmdCreateWorkItem. The
// item ID is generated server-side from the injected IDGenerator; retries
// of the same command_id return the stored receipt without re-running
// this handler, so the ID stays stable.
func CreateWorkItemHandler(gen ports.IDGenerator) appcmd.Handler {
	return func(_ context.Context, cmd appcmd.Command) ([]appcmd.EventDraft, error) {
		p, ok := cmd.Payload.(CreateWorkItem)
		if !ok {
			return nil, errors.New("work: payload must be work.CreateWorkItem")
		}
		title := strings.TrimSpace(p.Title)
		typ := strings.TrimSpace(p.ItemType)
		if title == "" {
			return nil, ErrEmptyTitle
		}
		if typ == "" {
			return nil, ErrEmptyType
		}

		itemID := gen.NewID()
		return []appcmd.EventDraft{{
			EventType:     domainwork.EventItemCreated,
			StreamID:      "workitem:" + itemID,
			SchemaVersion: 1,
			Payload: domainwork.ItemCreated{
				ItemID:   itemID,
				Title:    title,
				ItemType: typ,
				Status:   "open",
			},
		}}, nil
	}
}
