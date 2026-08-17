// Package work hosts the application handlers of the Work bounded
// context: commands validated by domain rules and expressed as event
// drafts for the command bus.
package work

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	appcmd "github.com/Rubentxu/golem/internal/application/command"
	"github.com/Rubentxu/golem/internal/ports"
	domainwork "github.com/Rubentxu/golem/internal/work"
)

// Domain validation errors of the Work context.
var (
	ErrEmptyTitle        = errors.New("work: title is mandatory")
	ErrEmptyType         = errors.New("work: item type is mandatory")
	ErrItemNotFound      = errors.New("work: work item not found")
	ErrInvalidRelation   = errors.New("work: invalid relation")
	ErrNothingToUpdate   = errors.New("work: nothing to update")
	ErrInvalidTypeDef    = errors.New("work: invalid work type definition")
	ErrUnknownTypeName   = errors.New("work: unknown work type")
	ErrFieldValidation   = errors.New("work: field validation failed")
	ErrInvalidTransition = errors.New("work: invalid status transition")
)

// Command names of this context.
const (
	CmdCreateWorkItem   = "work.create-work-item"
	CmdUpdateWorkItem   = "work.update-work-item"
	CmdLinkWorkItems    = "work.link-work-items"
	CmdRegisterWorkType = "work.register-work-type"
	CmdAddComment       = "work.add-comment"
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

// CreateWorkItem is the payload of CmdCreateWorkItem. ItemID and
// External are importer-only escapes: normal callers leave them empty
// (server-generated id); importers pass a stable id + provider identity
// so re-imports are idempotent (command dedup) and traceable back to the
// source system (GRAPH_MODEL ExternalIdentity).
type CreateWorkItem struct {
	Title    string                      `json:"title"`
	ItemType string                      `json:"type"`
	TypeName string                      `json:"type_name,omitempty"`
	Fields   map[string]any              `json:"fields,omitempty"`
	ItemID   string                      `json:"item_id,omitempty"`  // importer-only
	External domainwork.ExternalIdentity `json:"external,omitempty"` // importer-only
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

// AddComment is the payload of CmdAddComment.
type AddComment struct {
	ItemID string `json:"item_id"`
	Body   string `json:"body"`
}

// AddCommentHandler returns the handler for CmdAddComment. The comment
// is journaled on the item stream (immutable collaboration record); it
// deliberately projects no graph node — history and search carry it.
// Command dedup keeps retries single-comment (same command_id).
func AddCommentHandler(gen ports.IDGenerator, journal ports.JournalStore) appcmd.Handler {
	return func(ctx context.Context, cmd appcmd.Command) ([]appcmd.EventDraft, error) {
		p, ok := cmd.Payload.(AddComment)
		if !ok {
			return nil, errors.New("work: payload must be work.AddComment")
		}
		if strings.TrimSpace(p.ItemID) == "" {
			return nil, ErrItemNotFound
		}
		if strings.TrimSpace(p.Body) == "" {
			return nil, errors.New("work: comment body is mandatory")
		}
		stream := "workitem:" + p.ItemID
		evs, err := journal.ReadStream(ctx, cmd.TenantID, stream, 0)
		if err != nil {
			return nil, err
		}
		if len(evs) == 0 {
			return nil, ErrItemNotFound
		}
		return []appcmd.EventDraft{{
			EventType:     domainwork.EventCommentAdded,
			StreamID:      stream,
			SchemaVersion: 1,
			Payload: domainwork.CommentAdded{
				ItemID: p.ItemID, CommentID: gen.NewID(), Body: strings.TrimSpace(p.Body),
			},
		}}, nil
	}
}

// LinkWorkItems is the payload of CmdLinkWorkItems.
type LinkWorkItems struct {
	FromID   string `json:"from_id"`
	ToID     string `json:"to_id"`
	Relation string `json:"relation"`
}

// RegisterWorkType is the payload of CmdRegisterWorkType.
type RegisterWorkType struct {
	Name        string                  `json:"name"`
	Initial     string                  `json:"initial"`
	States      []string                `json:"states"`
	Transitions []domainwork.Transition `json:"transitions"`
	Fields      []domainwork.FieldDef   `json:"fields"`
}

// RegisterWorkTypeHandler validates and journals a WorkType definition.
// The definition is authoritative once projected; items reference it by
// name at creation.
func RegisterWorkTypeHandler() appcmd.Handler {
	return func(_ context.Context, cmd appcmd.Command) ([]appcmd.EventDraft, error) {
		p, ok := cmd.Payload.(RegisterWorkType)
		if !ok {
			return nil, errors.New("work: payload must be work.RegisterWorkType")
		}
		if err := validateTypeDef(p); err != nil {
			return nil, err
		}
		return []appcmd.EventDraft{{
			EventType:     domainwork.EventTypeRegistered,
			StreamID:      "worktype:" + p.Name,
			SchemaVersion: 1,
			Payload: domainwork.TypeRegistered{
				Name: p.Name, Initial: p.Initial,
				States: p.States, Transitions: p.Transitions, Fields: p.Fields,
			},
		}}, nil
	}
}

func validateTypeDef(p RegisterWorkType) error {
	name := strings.TrimSpace(p.Name)
	if name == "" {
		return fmt.Errorf("%w: name is mandatory", ErrInvalidTypeDef)
	}
	if len(p.States) == 0 {
		return fmt.Errorf("%w: at least one state", ErrInvalidTypeDef)
	}
	seen := map[string]bool{}
	for _, s := range p.States {
		if strings.TrimSpace(s) == "" || seen[s] {
			return fmt.Errorf("%w: states must be non-empty and unique", ErrInvalidTypeDef)
		}
		seen[s] = true
	}
	if !seen[p.Initial] {
		return fmt.Errorf("%w: initial %q is not a state", ErrInvalidTypeDef, p.Initial)
	}
	for _, t := range p.Transitions {
		if !seen[t.From] || !seen[t.To] {
			return fmt.Errorf("%w: transition %s→%s references unknown states", ErrInvalidTypeDef, t.From, t.To)
		}
	}
	reserved := map[string]bool{"title": true, "type": true, "status": true}
	fnames := map[string]bool{}
	for _, f := range p.Fields {
		n := strings.TrimSpace(f.Name)
		if n == "" || reserved[n] || fnames[n] {
			return fmt.Errorf("%w: field names must be unique and not reserved", ErrInvalidTypeDef)
		}
		fnames[n] = true
		switch f.Type {
		case "string", "number", "bool":
		default:
			return fmt.Errorf("%w: field %q type must be string|number|bool", ErrInvalidTypeDef, n)
		}
	}
	return nil
}

// workTypeOf loads a projected WorkType definition by name.
func workTypeOf(ctx context.Context, graph ports.GraphStore, tenant ports.TenantID, name string) (*domainwork.TypeRegistered, error) {
	n, err := graph.GetNode(ctx, tenant, name)
	if err != nil {
		if errors.Is(err, ports.ErrNodeNotFound) {
			return nil, fmt.Errorf("%w: %s", ErrUnknownTypeName, name)
		}
		return nil, err
	}
	if n.Kind != "WorkType" {
		return nil, fmt.Errorf("%w: %s", ErrUnknownTypeName, name)
	}
	return typeFromAttrs(n.Attributes), nil
}

// typeFromAttrs rebuilds a definition from projected node attributes
// (mirror of the projector mapping).
func typeFromAttrs(a map[string]any) *domainwork.TypeRegistered {
	decode := func(v any, out any) {
		if b, err := json.Marshal(v); err == nil {
			_ = json.Unmarshal(b, out)
		}
	}
	def := &domainwork.TypeRegistered{}
	if s, ok := a["name"].(string); ok {
		def.Name = s
	}
	if s, ok := a["initial"].(string); ok {
		def.Initial = s
	}
	decode(a["states"], &def.States)
	decode(a["transitions"], &def.Transitions)
	decode(a["fields"], &def.Fields)
	return def
}

// validateFields checks custom fields against the type schema.
func validateFields(def *domainwork.TypeRegistered, fields map[string]any) error {
	byName := map[string]domainwork.FieldDef{}
	for _, f := range def.Fields {
		byName[f.Name] = f
	}
	for name, def := range byName {
		v, present := fields[name]
		if !present || v == nil {
			if def.Required {
				return fmt.Errorf("%w: %q is required", ErrFieldValidation, name)
			}
			continue
		}
		switch def.Type {
		case "string":
			if _, ok := v.(string); !ok {
				return fmt.Errorf("%w: %q must be a string", ErrFieldValidation, name)
			}
		case "number":
			if _, ok := v.(float64); !ok {
				return fmt.Errorf("%w: %q must be a number", ErrFieldValidation, name)
			}
		case "bool":
			if _, ok := v.(bool); !ok {
				return fmt.Errorf("%w: %q must be a bool", ErrFieldValidation, name)
			}
		}
	}
	for name := range fields {
		if _, known := byName[name]; !known {
			return fmt.Errorf("%w: %q is not defined in the type schema", ErrFieldValidation, name)
		}
	}
	return nil
}

// CreateWorkItemHandler returns the handler for CmdCreateWorkItem. The
// item ID is generated server-side from the injected IDGenerator; retries
// of the same command_id return the stored receipt without re-running
// this handler, so the ID stays stable. When TypeName is set, the type
// definition (graph projection) validates Fields and supplies the
// initial workflow state.
func CreateWorkItemHandler(gen ports.IDGenerator, graph ports.GraphStore) appcmd.Handler {
	return func(ctx context.Context, cmd appcmd.Command) ([]appcmd.EventDraft, error) {
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

		status := "open"
		fields := p.Fields
		if name := strings.TrimSpace(p.TypeName); name != "" {
			def, err := workTypeOf(ctx, graph, cmd.TenantID, name)
			if err != nil {
				return nil, err
			}
			if err := validateFields(def, fields); err != nil {
				return nil, err
			}
			status = def.Initial
		}

		itemID := strings.TrimSpace(p.ItemID)
		if itemID == "" {
			itemID = gen.NewID()
		}
		return []appcmd.EventDraft{{
			EventType:     domainwork.EventItemCreated,
			StreamID:      "workitem:" + itemID,
			SchemaVersion: 1,
			Payload: domainwork.ItemCreated{
				ItemID: itemID, Title: title, ItemType: typ,
				TypeName: strings.TrimSpace(p.TypeName), Status: status, Fields: fields,
				External: p.External,
			},
		}}, nil
	}
}

// UpdateWorkItemHandler returns the handler for CmdUpdateWorkItem. The
// item must exist (its journal stream is non-empty) and, when expected is
// provided, the stream must still be at that version — enforced
// atomically by the journal's conditional append (ADR-021). Status
// changes of typed items must follow the type workflow (the current
// status is folded from the stream; the workflow from the projection).
func UpdateWorkItemHandler(journal ports.JournalStore, graph ports.GraphStore) appcmd.Handler {
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

		if p.Status != nil {
			current, typeName := foldItemState(evs)
			if typeName != "" {
				def, err := workTypeOf(ctx, graph, cmd.TenantID, typeName)
				if err != nil {
					return nil, err
				}
				if err := validateTransition(def, current, *p.Status); err != nil {
					return nil, err
				}
			}
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

// foldItemState rebuilds (currentStatus, typeName) from the item stream:
// created sets both; updates overwrite the status. Domain folding is
// deterministic and cheap (streams are short).
func foldItemState(evs []ports.RawEvent) (status, typeName string) {
	for _, env := range evs {
		switch env.EventType {
		case domainwork.EventItemCreated:
			var p domainwork.ItemCreated
			if json.Unmarshal(env.Payload, &p) == nil {
				status, typeName = p.Status, p.TypeName
			}
		case domainwork.EventItemUpdated:
			var p domainwork.ItemUpdated
			if json.Unmarshal(env.Payload, &p) == nil && p.Status != nil {
				status = *p.Status
			}
		}
	}
	return status, typeName
}

// validateTransition allows same-status no-ops and declared transitions.
func validateTransition(def *domainwork.TypeRegistered, from, to string) error {
	if from == to {
		return nil
	}
	for _, t := range def.Transitions {
		if t.From == from && t.To == to {
			return nil
		}
	}
	return fmt.Errorf("%w: %s→%s in type %q", ErrInvalidTransition, from, to, def.Name)
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
