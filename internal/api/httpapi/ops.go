// Package httpapi is the API Edge of GOLEM.
package httpapi

import (
	"net/http"

	"github.com/Rubentxu/golem/internal/ports"
)

// SystemStatus represents the overall system status.
type SystemStatus struct {
	Version   string            `json:"version"`
	Uptime    string            `json:"uptime"`
	Graph     string            `json:"graph"`
	Journal   string            `json:"journal"`
	BuildInfo map[string]string `json:"build_info,omitempty"`
}

// handleStatus handles GET /status (operator role required).
func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	principal, ok := r.Context().Value(principalContextKey{}).(ports.Principal)
	if !ok {
		writeError(w, http.StatusUnauthorized, "no principal in context")
		return
	}

	// Basic system status - extend with actual health checks
	status := SystemStatus{
		Version: "1.0.0",
		Uptime:  "unknown", // would need process start time
		Graph:   "ok",
		Journal: "ok",
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":    "operational",
		"principal": principal.Subject,
		"version":   status.Version,
	})
}
