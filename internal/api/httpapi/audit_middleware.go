// Package httpapi is the API Edge of GOLEM.
package httpapi

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/Rubentxu/golem/internal/ports"
)

// AuditLogger emits ops.console.action events for admin endpoints.
type AuditLogger interface {
	Emit(ctx context.Context, eventType string, payload any) error
}

// auditMiddleware wraps admin handlers to emit audit events (REQ-OPS-003).
func (s *Server) auditMiddleware(h http.Handler, action, targetPattern string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		corr := r.Header.Get("X-Correlation-Id")
		if corr == "" {
			corr = strconv.FormatInt(int64(time.Now().UnixNano()), 10)
		}

		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		h.ServeHTTP(rec, r.WithContext(r.Context()))

		// Emit audit event after handler runs.
		if s.AuditLogger != nil {
			principal, _ := r.Context().Value(principalContextKey{}).(ports.Principal)
			status := "completed"
			if rec.status >= 400 {
				status = "rejected"
			}
			subject := ""
			if principal.Subject != "" {
				subject = principal.Subject
			}
			payload := ports.OpsConsoleActionPayload{
				Action:      action,
				Target:      targetPattern,
				Status:      status,
				Subject:     subject,
				Correlation: corr,
			}
			_ = s.AuditLogger.Emit(r.Context(), ports.EventOpsConsoleActionCompleted, payload)
		}
	})
}

// statusRecorder captures the status code written by a handler.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}
