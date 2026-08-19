package httpapi

import (
	"net/http"

	"github.com/Rubentxu/golem/internal/api/httpapi/admin"
	"github.com/Rubentxu/golem/internal/ports"
)

// AdminMount provides operator-only admin endpoints.
type AdminMount struct {
	adminMux *admin.AdminMux
	obs      ports.Observability
}

// NewAdminMount creates an AdminMount wrapping the provided AdminMux.
func NewAdminMount(adminMux *admin.AdminMux, obs ports.Observability) *AdminMount {
	return &AdminMount{adminMux: adminMux, obs: obs}
}

func (m *AdminMount) Pattern() string { return "/admin" }

func (m *AdminMount) Mount(mux *http.ServeMux, deps MountDeps) error {
	if m.adminMux == nil {
		return nil
	}
	mux.Handle("/admin/", m.adminMux.Handler())
	return nil
}

var _ HTTPMount = (*AdminMount)(nil)
