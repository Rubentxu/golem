package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/Rubentxu/golem/internal/ports"
)

// PackRegistry is the port the packs endpoints need. Implemented by
// adapters/registry/filesystem (M5.1) and future OCI adapters (M8).
type PackRegistry = ports.PackRegistry

// WithPacks sets the capability-pack registry (enables
// POST /api/v1/packs/activate).
func (s *Server) WithPacks(reg PackRegistry) *Server {
	s.packs = reg
	return s
}

type activatePackBody struct {
	// SourcePath is the pack location relative to the registry root
	// (filesystem adapter: ./packs). Q9.
	SourcePath string `json:"source_path"`
}

type packActivatedReceipt struct {
	Name            string `json:"name"`
	Version         string `json:"version"`
	IntegrityDigest string `json:"integrity_digest"`
	EventID         string `json:"event_id"`
	Position        uint64 `json:"position"`
}

// handleActivatePack activates a capability pack for the caller's tenant.
// The tenant comes from X-Golem-Tenant (same contract as every other
// endpoint); the pack source is explicit in the body.
func (s *Server) handleActivatePack(w http.ResponseWriter, r *http.Request) {
	corr := s.correlationOf(r)

	if s.packs == nil {
		s.problem(w, http.StatusServiceUnavailable, CodeInternal, "pack registry not configured", corr)
		return
	}

	tenant, _, ok := tenantActor(r)
	if !ok {
		s.problem(w, http.StatusBadRequest, CodeMissingTenant, "X-Golem-Tenant header is mandatory", corr)
		return
	}

	var body activatePackBody
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body); err != nil {
		s.problem(w, http.StatusBadRequest, CodeInvalidArgument, "invalid JSON body: "+err.Error(), corr)
		return
	}
	body.SourcePath = strings.TrimSpace(body.SourcePath)
	if body.SourcePath == "" {
		s.problem(w, http.StatusBadRequest, CodeInvalidArgument, "source_path is required", corr)
		return
	}

	pack, err := s.packs.Load(r.Context(), body.SourcePath)
	if err != nil {
		s.writePackError(w, err, corr)
		return
	}

	pos, err := s.packs.Activate(r.Context(), tenant, pack)
	if err != nil {
		s.writePackError(w, err, corr)
		return
	}

	st, err := s.packs.Status(r.Context(), tenant, pack.Name)
	if err != nil || st == nil {
		// Activation succeeded; status is best-effort enrichment.
		st = &ports.PackStatus{Name: pack.Name, Version: pack.Version, IntegrityDigest: pack.IntegrityDigest}
	}

	writeJSON(w, http.StatusAccepted, packActivatedReceipt{
		Name:            pack.Name,
		Version:         pack.Version,
		IntegrityDigest: pack.IntegrityDigest,
		EventID:         st.ActivatedEventID,
		Position:        uint64(pos),
	})
}

// writePackError maps pack sentinel errors to HTTP problem responses.
func (s *Server) writePackError(w http.ResponseWriter, err error, corr string) {
	switch {
	case errors.Is(err, ports.ErrPackNotFound):
		s.problem(w, http.StatusNotFound, CodeNotFound, err.Error(), corr)
	case errors.Is(err, ports.ErrPackAlreadyActivated):
		s.problem(w, http.StatusConflict, CodeRevisionConflict, err.Error(), corr)
	case errors.Is(err, ports.ErrPackManifestInvalid),
		errors.Is(err, ports.ErrPackIntegrityFailed),
		errors.Is(err, ports.ErrPackUnknownPermission),
		errors.Is(err, ports.ErrUnsupportedInM51):
		s.problem(w, http.StatusUnprocessableEntity, CodeDomainRejection, err.Error(), corr)
	default:
		s.problem(w, http.StatusInternalServerError, CodeInternal, err.Error(), corr)
	}
}
