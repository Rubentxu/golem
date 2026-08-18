// Package admin provides operator-only HTTP endpoints for cell and tenant
// management (ADR-081).
package admin

import (
	"encoding/json"
	"net/http"
)

// Problem is the RFC 7807-style error body.
type Problem struct {
	Type          string `json:"type"`
	Title         string `json:"title"`
	Status        int    `json:"status"`
	Code          string `json:"code"`
	Detail        string `json:"detail,omitempty"`
	CorrelationID string `json:"correlation_id"`
}

func writeProblem(w http.ResponseWriter, status int, code, detail, corr string) {
	title := http.StatusText(status)
	if title == "" {
		title = "Error"
		status = http.StatusInternalServerError
	}
	writeJSON(w, status, Problem{
		Type: "about:blank", Title: title, Status: status,
		Code: code, Detail: detail, CorrelationID: corr,
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		_ = err
	}
}
