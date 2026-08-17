package httpapi

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"

	"github.com/Rubentxu/golem/internal/application/ingest"
	apprelease "github.com/Rubentxu/golem/internal/application/release"
	"github.com/Rubentxu/golem/internal/application/supplychain"
	"github.com/Rubentxu/golem/internal/ports"
)

// IngestService is the provider event-sink dependency.
type IngestService interface {
	Ingest(ctx context.Context, tenant ports.TenantID, provider string, payload []byte) (*ingest.Report, error)
}

// ---- POST /api/v1/ingest/{provider} ----
//
// Webhook entry point: raw provider payload, command ids derived from
// the provider identity (redelivery-safe). No Idempotency-Key header —
// external idempotency lives in the derived command ids.

func (s *Server) handleIngest(w http.ResponseWriter, r *http.Request) {
	corr := s.correlationOf(r)
	tenant, _, ok := tenantActor(r)
	if !ok {
		s.problem(w, http.StatusBadRequest, CodeMissingTenant, "X-Golem-Tenant header is mandatory", corr)
		return
	}
	if s.ingest == nil {
		s.problem(w, http.StatusNotImplemented, CodeInternal, "ingest is not configured in this deployment", corr)
		return
	}

	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 4<<20))
	if err != nil {
		s.problem(w, http.StatusBadRequest, CodeInvalidArgument, "cannot read body: "+err.Error(), corr)
		return
	}

	rep, err := s.ingest.Ingest(r.Context(), tenant, r.PathValue("provider"), body)
	if err != nil {
		s.problem(w, http.StatusBadRequest, CodeInvalidArgument, err.Error(), corr)
		return
	}

	status := http.StatusAccepted
	if rep.Accepted == 0 && len(rep.Errors) > 0 {
		status = http.StatusUnprocessableEntity
	}
	writeJSON(w, status, map[string]any{
		"accepted":       rep.Accepted,
		"duplicates":     rep.Duplicates,
		"command_ids":    rep.CommandIDs,
		"errors":         errorStrings(rep.Errors),
		"correlation_id": corr,
	})
}

func errorStrings(errs []error) []string {
	out := make([]string, 0, len(errs))
	for _, e := range errs {
		out = append(out, e.Error())
	}
	return out
}

// ---- POST /api/v1/releases and /api/v1/releases/{id}/gate ----

func (s *Server) handleCreateRelease(w http.ResponseWriter, r *http.Request) {
	var body apprelease.CreateCandidate
	if err := decodeBody(w, r, &body); err != nil {
		corr := s.correlationOf(r)
		s.problem(w, http.StatusBadRequest, CodeInvalidArgument, "invalid JSON body: "+err.Error(), corr)
		return
	}
	s.submitCommand(w, r, apprelease.CmdCreateCandidate, body)
}

func (s *Server) handleEvaluateGate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	s.submitCommand(w, r, apprelease.CmdEvaluateGate, apprelease.EvaluateGate{ReleaseID: id})
}

// ---- GET /api/v1/releases/{id} ----

func (s *Server) handleGetRelease(w http.ResponseWriter, r *http.Request) {
	corr := s.correlationOf(r)
	tenant, _, ok := tenantActor(r)
	if !ok {
		s.problem(w, http.StatusBadRequest, CodeMissingTenant, "X-Golem-Tenant header is mandatory", corr)
		return
	}
	n, err := s.graph.GetNode(r.Context(), tenant, r.PathValue("id"))
	if err != nil {
		if errors.Is(err, ports.ErrNodeNotFound) {
			s.problem(w, http.StatusNotFound, CodeNotFound, "release not found", corr)
			return
		}
		s.problem(w, http.StatusInternalServerError, CodeInternal, "graph query failed", corr)
		return
	}
	if n.Kind != "Release" {
		s.problem(w, http.StatusNotFound, CodeNotFound, "release not found", corr)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id": n.ID, "kind": n.Kind, "attributes": n.Attributes,
	})
}

// ---- GET /api/v1/components/{purl}/blast-radius ----

func (s *Server) handleBlastRadius(w http.ResponseWriter, r *http.Request) {
	corr := s.correlationOf(r)
	tenant, _, ok := tenantActor(r)
	if !ok {
		s.problem(w, http.StatusBadRequest, CodeMissingTenant, "X-Golem-Tenant header is mandatory", corr)
		return
	}
	purl := r.PathValue("purl")
	if purl == "" {
		s.problem(w, http.StatusBadRequest, CodeInvalidArgument, "purl path parameter is mandatory", corr)
		return
	}
	// URL decode the purl (http.Route decode is available in Go 1.22+).
	decoded, err := url.PathUnescape(purl)
	if err != nil {
		s.problem(w, http.StatusBadRequest, CodeInvalidArgument, "invalid purl encoding", corr)
		return
	}

	result, err := supplychain.BlastRadius(r.Context(), s.graph, tenant, decoded)
	if err != nil {
		if errors.Is(err, supplychain.ErrInvalidPurlForBlast) {
			s.problem(w, http.StatusUnprocessableEntity, CodeDomainRejection, "unknown component: "+url.QueryEscape(decoded), corr)
			return
		}
		s.problem(w, http.StatusInternalServerError, CodeInternal, "blast radius query failed", corr)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
