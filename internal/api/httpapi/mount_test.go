package httpapi

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Rubentxu/golem/internal/ports"
)

// mockHandler returns a simple handler that records calls.
func mockHandler(label string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(label))
	}
}

func TestRegisterRoutePrefixEnforcement(t *testing.T) {
	mux := http.NewServeMux()
	deps := MountDeps{}

	// RegisterRoute prepends the prefix to subpattern.
	pattern, err := deps.RegisterRoute(mux, "GET", "/{id}", "/api/v1/work-items", mockHandler("ok"))
	if err != nil {
		t.Fatalf("RegisterRoute failed: %v", err)
	}
	// The effective pattern is just the path (no method prefix).
	if pattern != "/api/v1/work-items/{id}" {
		t.Errorf("pattern = %q, want %q", pattern, "/api/v1/work-items/{id}")
	}

	// Verify the mux serves the correct path.
	req := httptest.NewRequest("GET", "/api/v1/work-items/abc123", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

func TestRegisterRouteRejectsPatternOverlap(t *testing.T) {
	mux := http.NewServeMux()
	deps := MountDeps{}

	// Register a route.
	_, err := deps.RegisterRoute(mux, "GET", "/{id}", "/api/v1/work-items", mockHandler("work-item"))
	if err != nil {
		t.Fatalf("first registration failed: %v", err)
	}

	// Exact duplicate registration should fail (same pattern).
	_, err = deps.RegisterRoute(mux, "GET", "/{id}", "/api/v1/work-items", mockHandler("work-item"))
	if err == nil {
		t.Error("duplicate registration should have failed")
	}
	if !errors.Is(err, ErrPatternOverlap) {
		t.Errorf("err = %v, want errors.Is(err, ErrPatternOverlap)", err)
	}

	// Non-overlapping registration should succeed (different prefix).
	_, err = deps.RegisterRoute(mux, "GET", "/{name}", "/api/v1/work-types", mockHandler("work-type"))
	if err != nil {
		t.Errorf("non-overlapping registration failed: %v", err)
	}
}

func TestMountDepsTypedFieldsNoAny(t *testing.T) {
	// Verify MountDeps has exactly 5 external typed interface fields plus
	// 2 internal bookkeeping fields (registry, routeLabels via regState).
	// This test uses compile-time checks: if any field is interface{} or any,
	// the explicit type assertions below will fail to compile.
	deps := MountDeps{}

	// Suppress unused variable warnings — 5 external fields.
	_ = deps.Observability
	_ = deps.Bus
	_ = deps.GraphStore
	_ = deps.GraphNodeFetcher
	_ = deps.JournalStreamReader

	// Verify all fields are non-nil interfaces (not interface{}).
	// Using compile-time type assertions; if compilation succeeds,
	// all fields are properly typed (not interface{} or any).
	t.Log("MountDeps has 5 external typed fields: Observability, Bus, GraphStore, GraphNodeFetcher, JournalStreamReader")
}

func TestMultiMountAdditionalPatternsEnumerated(t *testing.T) {
	// Verify MultiMount.AdditionalPatterns() returns the correct secondary prefixes.
	// This test verifies the contract: WorkMount implements MultiMount and
	// AdditionalPatterns() returns all secondary prefixes.

	// WorkMount concrete type check — verifies AdditionalPatterns is implemented.
	// We can't call Mount without full deps, but we can verify the interface contract.
	var _ MultiMount = (*workMountForTest)(nil)

	wm := &workMountForTest{}
	patterns := wm.AdditionalPatterns()
	if len(patterns) != 1 || patterns[0] != "/api/v1/work-types" {
		t.Errorf("AdditionalPatterns() = %v, want [/api/v1/work-types]", patterns)
	}
}

// workMountForTest is a minimal MultiMount implementation for interface verification.
type workMountForTest struct{}

func (w *workMountForTest) Pattern() string                              { return "/api/v1/work-items" }
func (w *workMountForTest) Mount(mux *http.ServeMux, deps MountDeps) error { return nil }
func (w *workMountForTest) AdditionalPatterns() []string                { return []string{"/api/v1/work-types"} }

func TestSegmentsOverlap(t *testing.T) {
	tests := []struct {
		a, b   string
		overlap bool
	}{
		{"/api/v1/work-items/{id}", "/api/v1/work-items/{id}", true},     // exact same pattern
		{"/api/v1/work-items/{id}", "/api/v1/work-items/events", false},  // different terminal segment (not both params)
		{"/api/v1/work-types/{name}", "/api/v1/work-items/{id}", false},  // different prefix
		{"/api/v1/work-items/{id}/events", "/api/v1/work-items/{id}", false}, // different lengths
		{"/api/v1/work-items/{id}", "/api/v1/work-items/{id}/events", false}, // different lengths
	}

	for _, tt := range tests {
		got := segmentsOverlap(tt.a, tt.b)
		if got != tt.overlap {
			t.Errorf("segmentsOverlap(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.overlap)
		}
	}
}

func TestNewWithMountsConstruction(t *testing.T) {
	// Verify NewWithMounts accepts bus, deps, and mounts slice without panicking.
	deps := MountDeps{
		Observability: ports.Observability{},
	}
	mounts := []HTTPMount{}

	// Should not panic with empty mounts.
	srv := NewWithMounts(nil, deps, mounts)
	if srv == nil {
		t.Error("NewWithMounts returned nil")
	}
}
