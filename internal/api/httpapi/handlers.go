package httpapi

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/Rubentxu/golem/internal/application/command"
	appreq "github.com/Rubentxu/golem/internal/application/requirements"
	appwork "github.com/Rubentxu/golem/internal/application/work"
	"github.com/Rubentxu/golem/internal/ports"
)

// ---- GET /api/v1/work-items/{id} and /api/v1/requirements/{id} ----

// entityDTO is the read model of a projected node: the node plus the
// stream version of its journal stream, exposed as revision for
// optimistic concurrency (ETag).
type entityDTO struct {
	ID         string         `json:"id"`
	Kind       string         `json:"kind"`
	Revision   uint64         `json:"revision"`
	Version    uint64         `json:"stream_version"`
	Attributes map[string]any `json:"attributes"`
}

func (s *Server) handleGetWorkItem(w http.ResponseWriter, r *http.Request) {
	s.handleGetEntity(w, r, "workitem")
}

func (s *Server) handleGetRequirement(w http.ResponseWriter, r *http.Request) {
	s.handleGetEntity(w, r, "requirement")
}

func (s *Server) handleGetEntity(w http.ResponseWriter, r *http.Request, streamPrefix string) {
	corr := s.correlationOf(r)
	tenant, _, ok := tenantActor(r)
	if !ok {
		s.problem(w, http.StatusBadRequest, CodeMissingTenant, "X-Golem-Tenant header is mandatory", corr)
		return
	}
	id := r.PathValue("id")

	node, err := s.graph.GetNode(r.Context(), tenant, id)
	if err != nil {
		if errors.Is(err, ports.ErrNodeNotFound) {
			s.problem(w, http.StatusNotFound, CodeNotFound, "entity not found", corr)
			return
		}
		s.problem(w, http.StatusInternalServerError, CodeInternal, "graph query failed", corr)
		return
	}

	version := s.streamVersion(r, tenant, streamPrefix+":"+id)
	w.Header().Set("ETag", `"`+strconv.FormatUint(version, 10)+`"`)
	writeJSON(w, http.StatusOK, entityDTO{
		ID: node.ID, Kind: node.Kind, Revision: uint64(node.Revision),
		Version: version, Attributes: node.Attributes,
	})
}

func (s *Server) streamVersion(r *http.Request, tenant ports.TenantID, stream string) uint64 {
	evs, err := s.streams.ReadStream(r.Context(), tenant, stream, 0)
	if err != nil {
		return 0
	}
	return uint64(len(evs))
}

// ---- PATCH /api/v1/work-items/{id} ----

type updateWorkItemBody struct {
	Title  *string `json:"title"`
	Status *string `json:"status,omitempty"`
}

func (s *Server) handleUpdateWorkItem(w http.ResponseWriter, r *http.Request) {
	corr := s.correlationOf(r)
	tenant, actor, ok := tenantActor(r)
	if !ok {
		s.problem(w, http.StatusBadRequest, CodeMissingTenant, "X-Golem-Tenant header is mandatory", corr)
		return
	}
	id := r.PathValue("id")

	idemKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if len(idemKey) < 8 {
		s.problem(w, http.StatusBadRequest, CodeInvalidArgument, "Idempotency-Key header is required (min 8 chars)", corr)
		return
	}

	var body updateWorkItemBody
	if err := decodeBody(w, r, &body); err != nil {
		s.problem(w, http.StatusBadRequest, CodeInvalidArgument, "invalid JSON body: "+err.Error(), corr)
		return
	}

	cmdPayload := appwork.UpdateWorkItem{ItemID: id, Title: body.Title, Status: body.Status}

	// If-Match carries the expected stream version (optional optimistic
	// concurrency, API_GUIDELINES); absent means last-write-wins.
	if ifMatch := strings.TrimSpace(r.Header.Get("If-Match")); ifMatch != "" {
		v, err := parseETagVersion(ifMatch)
		if err != nil {
			s.problem(w, http.StatusBadRequest, CodeInvalidArgument, "If-Match must be a quoted stream version like \"3\"", corr)
			return
		}
		cmdPayload.ExpectedVersion = &v
	}

	receipt, err := s.commands.Submit(r.Context(), command.Command{
		Name:          appwork.CmdUpdateWorkItem,
		TenantID:      tenant,
		Actor:         actor,
		CommandID:     idemKey,
		CorrelationID: corr,
		Payload:       cmdPayload,
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

func parseETagVersion(ifMatch string) (uint64, error) {
	v := strings.TrimSpace(ifMatch)
	v = strings.TrimPrefix(v, `W/`)
	if len(v) >= 2 && v[0] == '"' && v[len(v)-1] == '"' {
		v = v[1 : len(v)-1]
	}
	return strconv.ParseUint(v, 10, 64)
}

// ---- POST /api/v1/work-items/{id}/links ----

type linkBody struct {
	ToID     string `json:"to_id"`
	Relation string `json:"relation"`
}

func (s *Server) handleLinkWorkItem(w http.ResponseWriter, r *http.Request) {
	corr := s.correlationOf(r)
	tenant, actor, ok := tenantActor(r)
	if !ok {
		s.problem(w, http.StatusBadRequest, CodeMissingTenant, "X-Golem-Tenant header is mandatory", corr)
		return
	}
	id := r.PathValue("id")

	idemKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if len(idemKey) < 8 {
		s.problem(w, http.StatusBadRequest, CodeInvalidArgument, "Idempotency-Key header is required (min 8 chars)", corr)
		return
	}

	var body linkBody
	if err := decodeBody(w, r, &body); err != nil {
		s.problem(w, http.StatusBadRequest, CodeInvalidArgument, "invalid JSON body: "+err.Error(), corr)
		return
	}

	receipt, err := s.commands.Submit(r.Context(), command.Command{
		Name:          appwork.CmdLinkWorkItems,
		TenantID:      tenant,
		Actor:         actor,
		CommandID:     idemKey,
		CorrelationID: corr,
		Payload:       appwork.LinkWorkItems{FromID: id, ToID: body.ToID, Relation: body.Relation},
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

// ---- POST /api/v1/requirements ----

type createRequirementBody struct {
	Title     string `json:"title"`
	Statement string `json:"statement"`
}

func (s *Server) handleCreateRequirement(w http.ResponseWriter, r *http.Request) {
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

	var body createRequirementBody
	if err := decodeBody(w, r, &body); err != nil {
		s.problem(w, http.StatusBadRequest, CodeInvalidArgument, "invalid JSON body: "+err.Error(), corr)
		return
	}

	receipt, err := s.commands.Submit(r.Context(), command.Command{
		Name:          appreq.CmdCreateRequirement,
		TenantID:      tenant,
		Actor:         actor,
		CommandID:     idemKey,
		CorrelationID: corr,
		Payload:       appreq.CreateRequirement{Title: body.Title, Statement: body.Statement},
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

// ---- shared error mapping ----

func (s *Server) writeCommandError(w http.ResponseWriter, err error, corr string) {
	switch {
	case errors.Is(err, appwork.ErrEmptyTitle), errors.Is(err, appwork.ErrEmptyType),
		errors.Is(err, appwork.ErrNothingToUpdate), errors.Is(err, appwork.ErrInvalidRelation),
		errors.Is(err, appreq.ErrEmptyTitle):
		s.problem(w, http.StatusUnprocessableEntity, CodeDomainRejection, err.Error(), corr)
	case errors.Is(err, appwork.ErrItemNotFound):
		s.problem(w, http.StatusNotFound, CodeNotFound, err.Error(), corr)
	case errors.Is(err, ports.ErrVersionConflict):
		s.problem(w, http.StatusConflict, CodeRevisionConflict, "stream moved since read; re-read and retry (If-Match)", corr)
	case errors.Is(err, ports.ErrEmptyTenant), errors.Is(err, ports.ErrEmptyActor):
		s.problem(w, http.StatusBadRequest, CodeInvalidArgument, err.Error(), corr)
	default:
		s.problem(w, http.StatusInternalServerError, CodeInternal, "command failed", corr)
	}
}
