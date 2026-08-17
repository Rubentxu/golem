package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/Rubentxu/golem/internal/application/command"
	appplanning "github.com/Rubentxu/golem/internal/application/planning"
	appprojects "github.com/Rubentxu/golem/internal/application/projects"
	appwork "github.com/Rubentxu/golem/internal/application/work"
	"github.com/Rubentxu/golem/internal/ports"
)

// submitCommand is the shared write path: headers, decode, submit,
// receipt mapping.
func (s *Server) submitCommand(w http.ResponseWriter, r *http.Request, name string, payload any) {
	corr := s.correlationOf(r)
	tenant, actor, ok := tenantActor(r)
	if !ok {
		s.problem(w, http.StatusBadRequest, CodeMissingTenant, "X-Golem-Tenant header is mandatory", corr)
		return
	}
	idemKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if len(idemKey) < 8 {
		s.problem(w, http.StatusBadRequest, CodeInvalidArgument, "Idempotency-Key header is required (min 8 chars)", corr)
		return
	}

	receipt, err := s.commands.Submit(r.Context(), command.Command{
		Name: name, TenantID: tenant, Actor: actor,
		CommandID: idemKey, CorrelationID: corr, Payload: payload,
	})
	if err != nil {
		s.writeCommandError(w, err, corr)
		return
	}
	status := http.StatusAccepted
	if receipt.Duplicate {
		status = http.StatusOK
	}
	writeJSON(w, status, Receipt{
		CommandID: receipt.CommandID, EventIDs: receipt.EventIDs,
		Position: uint64(receipt.Position), Duplicate: receipt.Duplicate,
	})
}

// ---- POST /api/v1/projects ----

func (s *Server) handleCreateProject(w http.ResponseWriter, r *http.Request) {
	var body appprojects.CreateProject
	if err := decodeBody(w, r, &body); err != nil {
		corr := s.correlationOf(r)
		s.problem(w, http.StatusBadRequest, CodeInvalidArgument, "invalid JSON body: "+err.Error(), corr)
		return
	}
	s.submitCommand(w, r, appprojects.CmdCreateProject, body)
}

// ---- POST /api/v1/planning/iterations and /milestones ----

func (s *Server) handleCreateIteration(w http.ResponseWriter, r *http.Request) {
	var body appplanning.CreateIteration
	if err := decodeBody(w, r, &body); err != nil {
		corr := s.correlationOf(r)
		s.problem(w, http.StatusBadRequest, CodeInvalidArgument, "invalid JSON body: "+err.Error(), corr)
		return
	}
	s.submitCommand(w, r, appplanning.CmdCreateIteration, body)
}

func (s *Server) handleCreateMilestone(w http.ResponseWriter, r *http.Request) {
	var body appplanning.CreateMilestone
	if err := decodeBody(w, r, &body); err != nil {
		corr := s.correlationOf(r)
		s.problem(w, http.StatusBadRequest, CodeInvalidArgument, "invalid JSON body: "+err.Error(), corr)
		return
	}
	s.submitCommand(w, r, appplanning.CmdCreateMilestone, body)
}

// ---- POST /api/v1/work-items/{id}/comments ----

func (s *Server) handleAddComment(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var body struct {
		Body string `json:"body"`
	}
	if err := decodeBody(w, r, &body); err != nil {
		corr := s.correlationOf(r)
		s.problem(w, http.StatusBadRequest, CodeInvalidArgument, "invalid JSON body: "+err.Error(), corr)
		return
	}
	s.submitCommand(w, r, appwork.CmdAddComment, appwork.AddComment{ItemID: id, Body: body.Body})
}

// ---- GET /api/v1/work-items/{id}/events (item history from the journal) ----

type eventDTO struct {
	EventID       string          `json:"event_id"`
	EventType     string          `json:"event_type"`
	SchemaVersion int             `json:"schema_version"`
	OccurredAt    time.Time       `json:"occurred_at"`
	Actor         ports.Actor     `json:"actor"`
	CorrelationID string          `json:"correlation_id,omitempty"`
	CommandID     string          `json:"command_id,omitempty"`
	Payload       json.RawMessage `json:"payload"`
}

func (s *Server) handleItemEvents(w http.ResponseWriter, r *http.Request) {
	corr := s.correlationOf(r)
	tenant, _, ok := tenantActor(r)
	if !ok {
		s.problem(w, http.StatusBadRequest, CodeMissingTenant, "X-Golem-Tenant header is mandatory", corr)
		return
	}
	id := r.PathValue("id")

	evs, err := s.streams.ReadStream(r.Context(), tenant, "workitem:"+id, 0)
	if err != nil {
		s.problem(w, http.StatusInternalServerError, CodeInternal, "journal read failed", corr)
		return
	}
	if len(evs) == 0 {
		s.problem(w, http.StatusNotFound, CodeNotFound, "entity not found", corr)
		return
	}
	out := make([]eventDTO, 0, len(evs))
	for _, e := range evs {
		out = append(out, eventDTO{
			EventID: e.EventID, EventType: e.EventType, SchemaVersion: e.SchemaVersion,
			OccurredAt: e.OccurredAt, Actor: e.Actor,
			CorrelationID: e.CorrelationID, CommandID: e.CommandID,
			Payload: e.Payload,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"events": out, "count": len(out)})
}

// ---- GET /api/v1/planning/iterations/{id}/board ----
//
// The board is a derived view (no Board entity in M2): the CONTAINS
// edges of the iteration plus each item's current status — read from
// the projection, tenant-scoped and bounded.

type boardColumn struct {
	Status string      `json:"status"`
	Items  []boardItem `json:"items"`
}

type boardItem struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Type  string `json:"type"`
}

func (s *Server) handleIterationBoard(w http.ResponseWriter, r *http.Request) {
	corr := s.correlationOf(r)
	tenant, _, ok := tenantActor(r)
	if !ok {
		s.problem(w, http.StatusBadRequest, CodeMissingTenant, "X-Golem-Tenant header is mandatory", corr)
		return
	}
	id := r.PathValue("id")

	if _, err := s.graph.GetNode(r.Context(), tenant, id); err != nil {
		s.problem(w, http.StatusNotFound, CodeNotFound, "iteration not found", corr)
		return
	}

	sub, err := s.graph.Neighborhood(r.Context(), ports.NeighborhoodQuery{
		TenantID: tenant, Roots: []string{id}, MaxDepth: 1, MaxNodes: 500, MaxEdges: 500,
	})
	if err != nil {
		s.problem(w, http.StatusInternalServerError, CodeInternal, "graph query failed", corr)
		return
	}

	// Board columns from CONTAINS edges toward WorkItems.
	columns := map[string]*boardColumn{}
	order := []string{}
	for _, e := range sub.Edges {
		if e.Type != "CONTAINS" {
			continue
		}
		member := e.TargetID
		if member == id {
			member = e.SourceID
		}
		n, err := s.graph.GetNode(r.Context(), tenant, member)
		if err != nil || n.Kind != "WorkItem" {
			continue
		}
		status, _ := n.Attributes["status"].(string)
		if status == "" {
			status = "unknown"
		}
		col, seen := columns[status]
		if !seen {
			col = &boardColumn{Status: status}
			columns[status] = col
			order = append(order, status)
		}
		title, _ := n.Attributes["title"].(string)
		typ, _ := n.Attributes["type"].(string)
		col.Items = append(col.Items, boardItem{ID: n.ID, Title: title, Type: typ})
	}

	out := make([]boardColumn, 0, len(order))
	for _, st := range order {
		out = append(out, *columns[st])
	}
	writeJSON(w, http.StatusOK, map[string]any{"iteration_id": id, "columns": out})
}
