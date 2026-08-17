// Package work hosts the application handlers of the Work bounded
// context: commands validated by domain rules and expressed as event
// drafts for the command bus.
package work

import (
	"context"
	"errors"
	"fmt"
	"strings"

	appcmd "github.com/Rubentxu/golem/internal/application/command"
	"github.com/Rubentxu/golem/internal/ports"
	domainwork "github.com/Rubentxu/golem/internal/work"
)

// Domain validation errors of the Work context.
var (
	ErrEmptyTitle      = errors.New("work: title is mandatory")
	ErrEmptyType       = errors.New("work: item type is mandatory")
	ErrItemNotFound    = errors.New("work: work item not found")
	ErrInvalidRelation = errors.New("work: invalid relation")
	ErrNothingToUpdate = errors.New("work: nothing to update")
)

// Command names of this context.
const (
	CmdCreateWorkItem = "work.create-work-item"
	CmdUpdateWorkItem = "work.update-work-item"
	CmdLinkWorkItems  = "work.link-work-items"
)

// CanonicalRelation reports whether rel belongs to the GOLEM ontology
// (GRAPH_MODEL relation set).
func CanonicalRelation(rel string) bool {
	switch strings.ToUpper(strings.TrimSpace(rel)) {
	case "IMPLEMENTS", "VERIFIES", "DEPENDS_ON", "CONTAINS", "BUILT_BY",
		"PRODUCED", "DERIVED_FROM", "HAS_SBOM", "ATTESTED_BY", "SIGNED_BY",
		"AFFECTED_BY", "MITIGATED_BY", "RELEASED_AS", "DEPLOYED_TO",
		"OWNED_BY", "APPROVED_BY", "CAUSED_BY", "EVIDENCED_BY":
		return true
	}
	return false
}

// CreateWorkItem is the payload of CmdCreateWorkItem.
type CreateWorkItem struct {
	Title    string `json:"title"`
	ItemType string `json:"type"`
}

// UpdateWorkItem is the payload of CmdUpdateWorkItem. Nil fields are
// unchanged; at least one must be set. ExpectedVersion (optional,
// HTTP If-Match) pins the journal stream version — a concurrent change
// fails with ports.ErrVersionConflict (ADR-021); nil means the handler
// derives the current version (last-write-wins).
type UpdateWorkItem struct {
	ItemID          string  `json:"item_id"`
	Title           *string `json:"title,omitempty"`
	Status          *string `json:"status,omitempty"`
	ExpectedVersion *uint64 `json:"expected_version,omitempty"`
}

// LinkWorkItems is the payload of CmdLinkWorkItems.
type LinkWorkItems struct {
	FromID   string `json:"from_id"`
	ToID     string `json:"to_id"`
	Relation string `json:"relation"`
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

// UpdateWorkItemHandler returns the handler for CmdUpdateWorkItem. The
// item must exist (its journal stream is non-empty) and, when expected is
// provided, the stream must still be at that version — enforced
// atomically by the journal's conditional append (ADR-021).
func UpdateWorkItemHandler(journal ports.JournalStore) appcmd.Handler {
	return func(ctx context.Context, cmd appcmd.Command) ([]appcmd.EventDraft, error) {
		p, ok := cmd.Payload.(UpdateWorkItem)
		if !ok {
			return nil, errors.New("work: payload must be work.UpdateWorkItem")
		}
		if strings.TrimSpace(p.ItemID) == "" {
			return nil, ErrItemNotFound
		}
		if p.Title == nil && p.Status == nil {
			return nil, ErrNothingToUpdate
		}
		if p.Title != nil && strings.TrimSpace(*p.Title) == "" {
			return nil, ErrEmptyTitle
		}
		if p.Status != nil && strings.TrimSpace(*p.Status) == "" {
			return nil, errors.New("work: status is mandatory when provided")
		}

		stream := "workitem:" + p.ItemID
		evs, err := journal.ReadStream(ctx, cmd.TenantID, stream, 0)
		if err != nil {
			return nil, err
		}
		if len(evs) == 0 {
			return nil, ErrItemNotFound
		}
		version := uint64(len(evs))
		if p.ExpectedVersion != nil {
			version = *p.ExpectedVersion
		}

		return []appcmd.EventDraft{{
			EventType:             domainwork.EventItemUpdated,
			StreamID:              stream,
			SchemaVersion:         1,
			Payload:               domainwork.ItemUpdated{ItemID: p.ItemID, Title: p.Title, Status: p.Status},
			ExpectedStreamVersion: &version,
		}}, nil
	}
}

// LinkWorkItemsHandler returns the handler for CmdLinkWorkItems. Both
// endpoints must exist as graph nodes of the tenant; the relation must
// belong to the canonical ontology. Existence is checked against the
// graph projection (authoritative for cross-context nodes such as
// Requirements).
func LinkWorkItemsHandler(graph ports.GraphStore) appcmd.Handler {
	exists := func(ctx context.Context, tenant ports.TenantID, id string) (bool, error) {
		sub, err := graph.Neighborhood(ctx, ports.NeighborhoodQuery{
			TenantID: tenant, Roots: []string{id}, MaxDepth: 1, MaxNodes: 1, MaxEdges: 1,
		})
		if err != nil {
			return false, err
		}
		return len(sub.Nodes) > 0, nil
	}
	return func(ctx context.Context, cmd appcmd.Command) ([]appcmd.EventDraft, error) {
		p, ok := cmd.Payload.(LinkWorkItems)
		if !ok {
			return nil, errors.New("work: payload must be work.LinkWorkItems")
		}
		rel := strings.ToUpper(strings.TrimSpace(p.Relation))
		if !CanonicalRelation(rel) {
			return nil, fmt.Errorf("%w: %s", ErrInvalidRelation, p.Relation)
		}
		if strings.TrimSpace(p.FromID) == "" || strings.TrimSpace(p.ToID) == "" || p.FromID == p.ToID {
			return nil, ErrInvalidRelation
		}
		fromOK, err := exists(ctx, cmd.TenantID, p.FromID)
		if err != nil {
			return nil, err
		}
		toOK, err := exists(ctx, cmd.TenantID, p.ToID)
		if err != nil {
			return nil, err
		}
		if !fromOK || !toOK {
			return nil, ErrItemNotFound
		}

		return []appcmd.EventDraft{{
			EventType:     domainwork.EventItemLinked,
			StreamID:      "workitem:" + p.FromID,
			SchemaVersion: 1,
			Payload:       domainwork.ItemLinked{FromID: p.FromID, ToID: p.ToID, Relation: rel},
		}}, nil
	}
}
