// Package search projects journal events into the search index
// (ADR-015: derived, rebuildable). It follows the same stateless
// pattern as the graph projector: events map to upsert-merge documents,
// so replaying the journal into a fresh index reproduces identical
// search results.
package search

import (
	"encoding/json"
	"fmt"

	"github.com/Rubentxu/golem/internal/planning"
	"github.com/Rubentxu/golem/internal/ports"
	"github.com/Rubentxu/golem/internal/projects"
	"github.com/Rubentxu/golem/internal/requirements"
	"github.com/Rubentxu/golem/internal/work"
)

// Projector maps journal events to search documents.
type Projector struct{}

// Project interprets one event; empty result means "nothing to index".
func (Projector) Project(env ports.RawEvent) ([]ports.SearchDoc, error) {
	switch env.EventType {
	case work.EventItemCreated:
		var p work.ItemCreated
		if err := json.Unmarshal(env.Payload, &p); err != nil {
			return nil, fmt.Errorf("search %s: %w", env.EventType, err)
		}
		if p.ItemID == "" {
			return nil, fmt.Errorf("search %s: empty item_id", env.EventType)
		}
		return []ports.SearchDoc{{
			ID:     p.ItemID,
			Tenant: ports.TenantID(env.TenantID),
			Kind:   "WorkItem",
			Title:  p.Title,
			Text:   p.Title + " " + p.ItemType + " " + p.Status,
		}}, nil

	case work.EventItemUpdated:
		var p work.ItemUpdated
		if err := json.Unmarshal(env.Payload, &p); err != nil {
			return nil, fmt.Errorf("search %s: %w", env.EventType, err)
		}
		if p.ItemID == "" {
			return nil, fmt.Errorf("search %s: empty item_id", env.EventType)
		}
		// Merge semantics of SearchIndex handle partial updates; Text
		// is rebuilt only when the status changed (title merge is
		// handled by the Title field).
		doc := ports.SearchDoc{ID: p.ItemID, Tenant: ports.TenantID(env.TenantID)}
		if p.Title != nil {
			doc.Title = *p.Title
		}
		if p.Status != nil {
			doc.Text = " " + *p.Status // status is appended to searchable text
		}
		return []ports.SearchDoc{doc}, nil

	case requirements.EventRequirementCreated:
		var p requirements.RequirementCreated
		if err := json.Unmarshal(env.Payload, &p); err != nil {
			return nil, fmt.Errorf("search %s: %w", env.EventType, err)
		}
		if p.RequirementID == "" {
			return nil, fmt.Errorf("search %s: empty requirement_id", env.EventType)
		}
		return []ports.SearchDoc{{
			ID:     p.RequirementID,
			Tenant: ports.TenantID(env.TenantID),
			Kind:   "Requirement",
			Title:  p.Title,
			Text:   p.Title + " " + p.Statement + " " + p.Status,
		}}, nil
	case work.EventCommentAdded:
		var p work.CommentAdded
		if err := json.Unmarshal(env.Payload, &p); err != nil {
			return nil, fmt.Errorf("search %s: %w", env.EventType, err)
		}
		if p.CommentID == "" {
			return nil, fmt.Errorf("search %s: empty comment_id", env.EventType)
		}
		return []ports.SearchDoc{{
			ID:     p.CommentID,
			Tenant: ports.TenantID(env.TenantID),
			Kind:   "Comment",
			Title:  "Comment on " + p.ItemID,
			Text:   p.Body,
		}}, nil

	case projects.EventProjectCreated:
		var p projects.ProjectCreated
		if err := json.Unmarshal(env.Payload, &p); err != nil {
			return nil, fmt.Errorf("search %s: %w", env.EventType, err)
		}
		if p.ProjectID == "" {
			return nil, fmt.Errorf("search %s: empty project_id", env.EventType)
		}
		return []ports.SearchDoc{{
			ID:     p.ProjectID,
			Tenant: ports.TenantID(env.TenantID),
			Kind:   "Project",
			Title:  p.Name,
			Text:   p.Name + " " + p.Description,
		}}, nil

	case planning.EventIterationCreated:
		var p planning.IterationCreated
		if err := json.Unmarshal(env.Payload, &p); err != nil {
			return nil, fmt.Errorf("search %s: %w", env.EventType, err)
		}
		if p.IterationID == "" {
			return nil, fmt.Errorf("search %s: empty iteration_id", env.EventType)
		}
		return []ports.SearchDoc{{
			ID:     p.IterationID,
			Tenant: ports.TenantID(env.TenantID),
			Kind:   "Iteration",
			Title:  p.Name,
			Text:   p.Name + " iteration sprint",
		}}, nil
	}
	return nil, nil
}
