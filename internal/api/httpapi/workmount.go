// Package httpapi is the API Edge of GOLEM.
package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/Rubentxu/golem/internal/application/command"
	appwork "github.com/Rubentxu/golem/internal/application/work"
	"github.com/Rubentxu/golem/internal/ports"
	domainwork "github.com/Rubentxu/golem/internal/work"
)

// WorkMount implements MultiMount for the work context.
// It serves 8 routes across 2 URL prefixes:
//   - /api/v1/work-items  (primary pattern)
//   - /api/v1/work-types  (additional pattern)
type WorkMount struct{}

// Pattern returns the primary URL prefix.
func (m *WorkMount) Pattern() string { return "/api/v1/work-items" }

// Mount registers all 8 work routes.
func (m *WorkMount) Mount(mux *http.ServeMux, deps MountDeps) error {
	// POST /api/v1/work-items
	if _, err := deps.RegisterRoute(mux, "POST", "", "/api/v1/work-items", m.handleCreateWorkItem(deps)); err != nil {
		return fmt.Errorf("register POST /api/v1/work-items: %w", err)
	}
	// GET /api/v1/work-items/{id}
	if _, err := deps.RegisterRoute(mux, "GET", "/{id}", "/api/v1/work-items", m.handleGetWorkItem(deps)); err != nil {
		return fmt.Errorf("register GET /api/v1/work-items/{id}: %w", err)
	}
	// PATCH /api/v1/work-items/{id}
	if _, err := deps.RegisterRoute(mux, "PATCH", "/{id}", "/api/v1/work-items", m.handleUpdateWorkItem(deps)); err != nil {
		return fmt.Errorf("register PATCH /api/v1/work-items/{id}: %w", err)
	}
	// POST /api/v1/work-items/{id}/links
	if _, err := deps.RegisterRoute(mux, "POST", "/{id}/links", "/api/v1/work-items", m.handleLinkWorkItem(deps)); err != nil {
		return fmt.Errorf("register POST /api/v1/work-items/{id}/links: %w", err)
	}
	// POST /api/v1/work-items/{id}/comments
	if _, err := deps.RegisterRoute(mux, "POST", "/{id}/comments", "/api/v1/work-items", m.handleAddComment(deps)); err != nil {
		return fmt.Errorf("register POST /api/v1/work-items/{id}/comments: %w", err)
	}
	// GET /api/v1/work-items/{id}/events
	if _, err := deps.RegisterRoute(mux, "GET", "/{id}/events", "/api/v1/work-items", m.handleItemEvents(deps)); err != nil {
		return fmt.Errorf("register GET /api/v1/work-items/{id}/events: %w", err)
	}

	// Work-types routes (additional pattern).
	// POST /api/v1/work-types
	if _, err := deps.RegisterRoute(mux, "POST", "", "/api/v1/work-types", m.handleRegisterWorkType(deps)); err != nil {
		return fmt.Errorf("register POST /api/v1/work-types: %w", err)
	}
	// GET /api/v1/work-types/{name}
	if _, err := deps.RegisterRoute(mux, "GET", "/{name}", "/api/v1/work-types", m.handleGetWorkType(deps)); err != nil {
		return fmt.Errorf("register GET /api/v1/work-types/{name}: %w", err)
	}

	return nil
}

// AdditionalPatterns returns the secondary URL prefix for work-types.
func (m *WorkMount) AdditionalPatterns() []string { return []string{"/api/v1/work-types"} }

// ---- HTTP handlers (stateless, deps-injected) ----

func (m *WorkMount) handleCreateWorkItem(deps MountDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		corr := m.correlationOf(r)
		tenant, actor, ok := m.tenantActor(r)
		if !ok {
			m.problem(w, http.StatusBadRequest, CodeMissingTenant, "X-Golem-Tenant header is mandatory", corr)
			return
		}
		idemKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
		if len(idemKey) < 8 {
			m.problem(w, http.StatusBadRequest, CodeInvalidArgument, "Idempotency-Key header is required (min 8 chars)", corr)
			return
		}
		var body struct {
			Title    string         `json:"title"`
			Type     string         `json:"type"`
			TypeName string         `json:"type_name,omitempty"`
			Fields   map[string]any `json:"fields,omitempty"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body); err != nil {
			m.problem(w, http.StatusBadRequest, CodeInvalidArgument, "invalid JSON body: "+err.Error(), corr)
			return
		}
		receipt, err := deps.Bus.Submit(r.Context(), command.Command{
			Name:          appwork.CmdCreateWorkItem,
			TenantID:      tenant,
			Actor:         actor,
			CommandID:     idemKey,
			CorrelationID: corr,
			Payload:       appwork.CreateWorkItem{Title: body.Title, ItemType: body.Type, TypeName: body.TypeName, Fields: body.Fields},
		})
		if err != nil {
			m.writeCommandError(w, err, corr)
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
}

func (m *WorkMount) handleGetWorkItem(deps MountDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		corr := m.correlationOf(r)
		tenant, _, ok := m.tenantActor(r)
		if !ok {
			m.problem(w, http.StatusBadRequest, CodeMissingTenant, "X-Golem-Tenant header is mandatory", corr)
			return
		}
		id := r.PathValue("id")

		node, err := deps.GraphNodeFetcher.GetNode(r.Context(), tenant, id)
		if err != nil {
			if errors.Is(err, ports.ErrNodeNotFound) {
				m.problem(w, http.StatusNotFound, CodeNotFound, "entity not found", corr)
				return
			}
			m.problem(w, http.StatusInternalServerError, CodeInternal, "graph query failed", corr)
			return
		}
		// Get stream version for ETag.
		evs, _ := deps.JournalStreamReader.ReadStream(r.Context(), tenant, "workitem:"+id, 0)
		version := uint64(len(evs))
		w.Header().Set("ETag", `"`+strconv.FormatUint(version, 10)+`"`)
		writeJSON(w, http.StatusOK, entityDTO{
			ID: node.ID, Kind: node.Kind, Revision: uint64(node.Revision),
			Version: version, Attributes: node.Attributes,
		})
	}
}

func (m *WorkMount) handleUpdateWorkItem(deps MountDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		corr := m.correlationOf(r)
		tenant, actor, ok := m.tenantActor(r)
		if !ok {
			m.problem(w, http.StatusBadRequest, CodeMissingTenant, "X-Golem-Tenant header is mandatory", corr)
			return
		}
		id := r.PathValue("id")
		idemKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
		if len(idemKey) < 8 {
			m.problem(w, http.StatusBadRequest, CodeInvalidArgument, "Idempotency-Key header is required (min 8 chars)", corr)
			return
		}
		var body struct {
			Title   *string `json:"title,omitempty"`
			Status  *string `json:"status,omitempty"`
			Version *uint64 `json:"expected_version,omitempty"` // If-Match
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body); err != nil {
			m.problem(w, http.StatusBadRequest, CodeInvalidArgument, "invalid JSON body: "+err.Error(), corr)
			return
		}
		cmdPayload := appwork.UpdateWorkItem{ItemID: id, Title: body.Title, Status: body.Status}
		if body.Version != nil {
			cmdPayload.ExpectedVersion = body.Version
		}
		receipt, err := deps.Bus.Submit(r.Context(), command.Command{
			Name:          appwork.CmdUpdateWorkItem,
			TenantID:      tenant,
			Actor:         actor,
			CommandID:     idemKey,
			CorrelationID: corr,
			Payload:       cmdPayload,
		})
		if err != nil {
			m.writeCommandError(w, err, corr)
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
}

func (m *WorkMount) handleLinkWorkItem(deps MountDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		corr := m.correlationOf(r)
		tenant, actor, ok := m.tenantActor(r)
		if !ok {
			m.problem(w, http.StatusBadRequest, CodeMissingTenant, "X-Golem-Tenant header is mandatory", corr)
			return
		}
		id := r.PathValue("id")
		idemKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
		if len(idemKey) < 8 {
			m.problem(w, http.StatusBadRequest, CodeInvalidArgument, "Idempotency-Key header is required (min 8 chars)", corr)
			return
		}
		var body struct {
			ToID     string `json:"to_id"`
			Relation string `json:"relation"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body); err != nil {
			m.problem(w, http.StatusBadRequest, CodeInvalidArgument, "invalid JSON body: "+err.Error(), corr)
			return
		}
		receipt, err := deps.Bus.Submit(r.Context(), command.Command{
			Name:          appwork.CmdLinkWorkItems,
			TenantID:      tenant,
			Actor:         actor,
			CommandID:     idemKey,
			CorrelationID: corr,
			Payload:       appwork.LinkWorkItems{FromID: id, ToID: body.ToID, Relation: body.Relation},
		})
		if err != nil {
			m.writeCommandError(w, err, corr)
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
}

func (m *WorkMount) handleAddComment(deps MountDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		corr := m.correlationOf(r)
		tenant, actor, ok := m.tenantActor(r)
		if !ok {
			m.problem(w, http.StatusBadRequest, CodeMissingTenant, "X-Golem-Tenant header is mandatory", corr)
			return
		}
		id := r.PathValue("id")
		idemKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
		if len(idemKey) < 8 {
			m.problem(w, http.StatusBadRequest, CodeInvalidArgument, "Idempotency-Key header is required (min 8 chars)", corr)
			return
		}
		var body struct {
			Body string `json:"body"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body); err != nil {
			m.problem(w, http.StatusBadRequest, CodeInvalidArgument, "invalid JSON body: "+err.Error(), corr)
			return
		}
		receipt, err := deps.Bus.Submit(r.Context(), command.Command{
			Name:          appwork.CmdAddComment,
			TenantID:      tenant,
			Actor:         actor,
			CommandID:     idemKey,
			CorrelationID: corr,
			Payload:       appwork.AddComment{ItemID: id, Body: body.Body},
		})
		if err != nil {
			m.writeCommandError(w, err, corr)
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
}

func (m *WorkMount) handleItemEvents(deps MountDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		corr := m.correlationOf(r)
		tenant, _, ok := m.tenantActor(r)
		if !ok {
			m.problem(w, http.StatusBadRequest, CodeMissingTenant, "X-Golem-Tenant header is mandatory", corr)
			return
		}
		id := r.PathValue("id")

		evs, err := deps.JournalStreamReader.ReadStream(r.Context(), tenant, "workitem:"+id, 0)
		if err != nil {
			m.problem(w, http.StatusInternalServerError, CodeInternal, "journal read failed", corr)
			return
		}
		if len(evs) == 0 {
			m.problem(w, http.StatusNotFound, CodeNotFound, "entity not found", corr)
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
}

func (m *WorkMount) handleRegisterWorkType(deps MountDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		corr := m.correlationOf(r)
		tenant, actor, ok := m.tenantActor(r)
		if !ok {
			m.problem(w, http.StatusBadRequest, CodeMissingTenant, "X-Golem-Tenant header is mandatory", corr)
			return
		}
		idemKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
		if len(idemKey) < 8 {
			m.problem(w, http.StatusBadRequest, CodeInvalidArgument, "Idempotency-Key header is required (min 8 chars)", corr)
			return
		}
		var body struct {
			Name        string                    `json:"name"`
			Initial     string                    `json:"initial"`
			States      []string                  `json:"states"`
			Transitions []domainwork.Transition  `json:"transitions"`
			Fields      []domainwork.FieldDef    `json:"fields"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body); err != nil {
			m.problem(w, http.StatusBadRequest, CodeInvalidArgument, "invalid JSON body: "+err.Error(), corr)
			return
		}
		receipt, err := deps.Bus.Submit(r.Context(), command.Command{
			Name:          appwork.CmdRegisterWorkType,
			TenantID:      tenant,
			Actor:         actor,
			CommandID:     idemKey,
			CorrelationID: corr,
			Payload:       appwork.RegisterWorkType(body),
		})
		if err != nil {
			m.writeCommandError(w, err, corr)
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
}

func (m *WorkMount) handleGetWorkType(deps MountDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		corr := m.correlationOf(r)
		tenant, _, ok := m.tenantActor(r)
		if !ok {
			m.problem(w, http.StatusBadRequest, CodeMissingTenant, "X-Golem-Tenant header is mandatory", corr)
			return
		}
		name := r.PathValue("name")

		node, err := deps.GraphNodeFetcher.GetNode(r.Context(), tenant, name)
		if err != nil {
			if errors.Is(err, ports.ErrNodeNotFound) {
				m.problem(w, http.StatusNotFound, CodeNotFound, "work type not found", corr)
				return
			}
			m.problem(w, http.StatusInternalServerError, CodeInternal, "graph query failed", corr)
			return
		}
		if node.Kind != "WorkType" {
			m.problem(w, http.StatusNotFound, CodeNotFound, "work type not found", corr)
			return
		}
		writeJSON(w, http.StatusOK, node.Attributes)
	}
}

// ---- helpers ----

func (m *WorkMount) tenantActor(r *http.Request) (ports.TenantID, ports.Actor, bool) {
	tenant := strings.TrimSpace(r.Header.Get("X-Golem-Tenant"))
	if tenant == "" {
		return "", ports.Actor{}, false
	}
	actorType := strings.TrimSpace(r.Header.Get("X-Golem-Actor-Type"))
	actorID := strings.TrimSpace(r.Header.Get("X-Golem-Actor-Id"))
	if actorType == "" || actorID == "" {
		actorType, actorID = "user", "anonymous"
	}
	return ports.TenantID(tenant), ports.Actor{Type: actorType, ID: actorID}, true
}

func (m *WorkMount) correlationOf(r *http.Request) string {
	if c, ok := ports.CorrelationFrom(r.Context()); ok {
		return c.CorrelationID
	}
	return strings.TrimSpace(r.Header.Get("X-Correlation-Id"))
}

func (m *WorkMount) problem(w http.ResponseWriter, status int, code, detail, corr string) {
	title := http.StatusText(status)
	if title == "" {
		title, status = "Error", http.StatusInternalServerError
	}
	writeJSON(w, status, Problem{
		Type: "about:blank", Title: title, Status: status,
		Code: code, Detail: detail, CorrelationID: corr,
	})
}

func (m *WorkMount) writeCommandError(w http.ResponseWriter, err error, corr string) {
	switch {
	case errors.Is(err, appwork.ErrEmptyTitle), errors.Is(err, appwork.ErrEmptyType),
		errors.Is(err, appwork.ErrNothingToUpdate), errors.Is(err, appwork.ErrInvalidRelation),
		errors.Is(err, appwork.ErrInvalidTypeDef), errors.Is(err, appwork.ErrUnknownTypeName),
		errors.Is(err, appwork.ErrFieldValidation), errors.Is(err, appwork.ErrInvalidTransition):
		m.problem(w, http.StatusUnprocessableEntity, CodeDomainRejection, err.Error(), corr)
	case errors.Is(err, appwork.ErrItemNotFound):
		m.problem(w, http.StatusNotFound, CodeNotFound, err.Error(), corr)
	default:
		m.problem(w, http.StatusInternalServerError, CodeInternal, "command failed", corr)
	}
}
