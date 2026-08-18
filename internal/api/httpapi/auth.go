// Package httpapi is the API Edge of GOLEM.
package httpapi

import (
	"context"
	"net/http"
	"strings"

	"github.com/Rubentxu/golem/internal/ports"
)

// OperatorAuth is the RBAC middleware that requires the "operator" role.
type OperatorAuth struct {
	authN ports.AuthN
}

func NewOperatorAuth(authN ports.AuthN) *OperatorAuth {
	return &OperatorAuth{authN: authN}
}

// RequireOperator returns a middleware that enforces operator role.
func (a *OperatorAuth) RequireOperator(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := extractBearerToken(r)
		if token == "" {
			writeError(w, http.StatusUnauthorized, "missing bearer token")
			return
		}

		principal, err := a.authN.VerifyBearer(r.Context(), token)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "invalid token: "+err.Error())
			return
		}

		if !isOperator(principal) {
			writeError(w, http.StatusForbidden, "operator role required")
			return
		}

		// Inject principal into context
		ctx := context.WithValue(r.Context(), principalContextKey{}, principal)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// isOperator returns true if the principal has the operator role.
func isOperator(p ports.Principal) bool {
	// Check groups
	for _, g := range p.Groups {
		if g == "golem.operator" {
			return true
		}
	}
	// Check claims for operator capability
	if caps, ok := p.Claims["capabilities"].([]any); ok {
		for _, c := range caps {
			if c == "golem.operator" {
				return true
			}
		}
	}
	return false
}

// extractBearerToken extracts the bearer token from the Authorization header.
func extractBearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, "Bearer ") {
		return ""
	}
	return strings.TrimPrefix(h, "Bearer ")
}

// writeError writes a JSON error response.
func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write([]byte(`{"error":` + `"` + msg + `"` + `}`))
}

type principalContextKey struct{}
