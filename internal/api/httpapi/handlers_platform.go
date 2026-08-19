package httpapi

import (
	"net/http"
)

// PlatformMount provides platform-level health, readiness, status and metrics endpoints.
type PlatformMount struct {
	metricsHandler http.Handler
}

// NewPlatformMount creates a PlatformMount with optional metrics handler.
func NewPlatformMount(metricsHandler http.Handler) *PlatformMount {
	return &PlatformMount{metricsHandler: metricsHandler}
}

func (m *PlatformMount) Pattern() string { return "" }

func (m *PlatformMount) Mount(mux *http.ServeMux, deps MountDeps) error {
	if _, err := deps.RegisterRoute(mux, "GET", "", "/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}); err != nil {
		return err
	}

	if _, err := deps.RegisterRoute(mux, "GET", "", "/readyz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ready"}`))
	}); err != nil {
		return err
	}

	if _, err := deps.RegisterRoute(mux, "GET", "", "/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"operational"}`))
	}); err != nil {
		return err
	}

	if m.metricsHandler != nil {
		mux.Handle("/metrics", m.metricsHandler)
	}

	return nil
}

var _ HTTPMount = (*PlatformMount)(nil)
