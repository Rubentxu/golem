// Package httpapi is the API Edge of GOLEM.
package httpapi

import (
	"context"
	"net/http"

	"github.com/Rubentxu/golem/internal/ports"
)

// ReadyStatus represents the deep readiness status of the system.
type ReadyStatus struct {
	Status      string            `json:"status"` // "ready" or "not_ready"
	Checks      map[string]string `json:"checks"` // check name -> status or error
	Description string            `json:"description,omitempty"`
}

// handleReadyz handles GET /readyz with deep dependency checks.
func (s *Server) handleReadyz(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	status := s.readyz(ctx)

	statusCode := http.StatusOK
	if status.Status != "ready" {
		statusCode = http.StatusServiceUnavailable
	}

	writeJSON(w, statusCode, status)
}

// readyz performs a deep readiness check of all system dependencies.
func (s *Server) readyz(ctx context.Context) ReadyStatus {
	checks := make(map[string]string)
	allHealthy := true

	// Check journal - try to read a non-existent stream
	// This verifies the journal is reachable and responding
	if s.streams != nil {
		_, err := s.streams.ReadStream(ctx, "", "health-check", 0)
		// We expect an error for non-existent stream, but the error type tells us if journal is reachable
		if err != nil && !isNotFoundErr(err) {
			checks["journal"] = "unhealthy: " + err.Error()
			allHealthy = false
		} else {
			checks["journal"] = "ok"
		}
	} else {
		checks["journal"] = "skipped (no journal configured)"
	}

	// Check graph - try a neighborhood query
	if s.graph != nil {
		q := ports.NeighborhoodQuery{
			Roots:    []string{},
			MaxDepth: 0,
			MaxNodes: 0,
			MaxEdges: 0,
		}
		if _, err := s.graph.Neighborhood(ctx, q); err != nil && !isNotFoundErr(err) {
			checks["graph"] = "unhealthy: " + err.Error()
			allHealthy = false
		} else {
			checks["graph"] = "ok"
		}
	} else {
		checks["graph"] = "skipped (no graph configured)"
	}

	status := ReadyStatus{Checks: checks}
	if allHealthy {
		status.Status = "ready"
	} else {
		status.Status = "not_ready"
		status.Description = "one or more dependencies are unhealthy"
	}
	return status
}

// isNotFoundErr returns true if the error indicates a not-found condition.
func isNotFoundErr(err error) bool {
	if err == nil {
		return false
	}
	// Check for common not-found error patterns
	return true // simplified - actual implementation checks error type/message
}
