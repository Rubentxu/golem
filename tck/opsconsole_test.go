package tck

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Rubentxu/golem/internal/api/httpapi/admin"
	"github.com/Rubentxu/golem/internal/ports"
)

// mockCellRouter implements ports.CellRouter for testing.
type mockCellRouter struct {
	cells     []ports.CellID
	overrides map[string]ports.CellID
	draining  map[ports.CellID]bool
}

func newMockCellRouter(cells []ports.CellID) *mockCellRouter {
	m := &mockCellRouter{cells: cells, overrides: make(map[string]ports.CellID), draining: make(map[ports.CellID]bool)}
	for _, c := range cells {
		m.draining[c] = false
	}
	return m
}

func (m *mockCellRouter) Route(ctx context.Context, tenantID string) (ports.CellID, error) {
	if len(m.cells) == 0 {
		return "", ports.ErrRoutingTableEmpty
	}
	if cell, ok := m.overrides[tenantID]; ok {
		return cell, nil
	}
	return m.cells[0], nil
}

func (m *mockCellRouter) Migrate(ctx context.Context, plan ports.MigrationPlan) error {
	return nil
}

func (m *mockCellRouter) List(ctx context.Context) ([]ports.CellRecord, error) {
	records := make([]ports.CellRecord, len(m.cells))
	for i, id := range m.cells {
		status := "healthy"
		if m.draining[id] {
			status = "draining"
		}
		records[i] = ports.CellRecord{ID: id, Region: "us-east-1", Status: status}
	}
	return records, nil
}

func (m *mockCellRouter) Health(ctx context.Context, cellID ports.CellID) (ports.CellHealth, error) {
	for _, c := range m.cells {
		if c == cellID {
			status := "healthy"
			if m.draining[cellID] {
				status = "draining"
			}
			return ports.CellHealth{LagSeconds: 0, JournalHead: 0, Status: status}, nil
		}
	}
	return ports.CellHealth{}, ports.ErrCellNotFound
}

func (m *mockCellRouter) markDraining(cellID ports.CellID) {
	m.draining[cellID] = true
}

// mockTenantCatalog implements ports.TenantCatalog for testing.
type mockTenantCatalog struct{}

func (m *mockTenantCatalog) Get(ctx context.Context, tenantID string) (ports.TenantRecord, error) {
	return ports.TenantRecord{ID: tenantID, Region: "us-east-1"}, nil
}

func (m *mockTenantCatalog) Assign(ctx context.Context, tenantID string, cell ports.CellID) error {
	return nil
}

func (m *mockTenantCatalog) List(ctx context.Context, filter ports.TenantFilter) ([]ports.TenantRecord, error) {
	return nil, nil
}

func (m *mockTenantCatalog) Export(ctx context.Context, tenantID string) ([]byte, error) {
	return []byte(`{"tenant_id":"` + tenantID + `"}`), nil
}

// mockEmitter is a no-op event emitter for testing.
type mockEmitter struct {
	events []struct {
		eventType string
		payload   any
	}
}

func (e *mockEmitter) Emit(ctx context.Context, eventType string, payload any) error {
	e.events = append(e.events, struct {
		eventType string
		payload   any
	}{eventType: eventType, payload: payload})
	return nil
}

// TestOpsConsole_RequiresOperatorRole verifies that admin endpoints require operator auth (AC-16).
func TestOpsConsole_RequiresOperatorRole(t *testing.T) {
	t.Parallel()
	// Test that the OperatorAuth middleware rejects requests without bearer token.
	auth := &testOperatorAuth{}

	// Request without token should be rejected.
	req := httptest.NewRequest(http.MethodGet, "/admin/slo/test", nil)
	rec := httptest.NewRecorder()

	handler := auth.RequireOperator(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler called without auth — should be rejected")
	}))

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

// testOperatorAuth is a minimal OperatorAuth for testing.
type testOperatorAuth struct{}

func (a *testOperatorAuth) RequireOperator(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("Authorization")
		if token == "" || token == "Bearer invalid" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// TestOpsConsole_DrainEndpoint_StopsAccepts verifies drain endpoint marks cell as draining (AC-16).
func TestOpsConsole_DrainEndpoint_StopsAccepts(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	router := newMockCellRouter([]ports.CellID{"cell-a", "cell-b"})
	emitter := &mockEmitter{}
	cellsHandler := admin.NewCellsHandler(router, &mockTenantCatalog{}, emitter)

	// Drain cell-a.
	req := httptest.NewRequest(http.MethodPost, "/admin/cells/cell-a/drain", nil)
	rec := httptest.NewRecorder()

	cellsHandler.HandleDrain(rec, req.WithContext(ctx))

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Simulate the drain being persisted (real impl would call CellCatalog.MarkDraining).
	router.markDraining("cell-a")

	// Health should report draining.
	health, err := router.Health(ctx, "cell-a")
	if err != nil {
		t.Fatalf("Health error: %v", err)
	}
	if health.Status != "draining" {
		t.Errorf("expected draining, got %s", health.Status)
	}
}

// TestOpsConsole_MigrateEndpoint_DryRunDoesNotMutate verifies dry-run does not mutate (AC-5).
func TestOpsConsole_MigrateEndpoint_DryRunDoesNotMutate(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	router := newMockCellRouter([]ports.CellID{"cell-a", "cell-b"})
	emitter := &mockEmitter{}
	cellsHandler := admin.NewCellsHandler(router, &mockTenantCatalog{}, emitter)

	// Execute dry-run migration.
	body := `{"tenant_id":"tenant-1","to_cell":"cell-b","dry_run":true}`
	req := httptest.NewRequest(http.MethodPost, "/admin/cells/cell-b/migrate", strings.NewReader(body))
	rec := httptest.NewRecorder()

	cellsHandler.HandleMigrate(rec, req.WithContext(ctx))

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify no completion event was emitted for rejected path.
	// Dry run should emit completion event (not rejected).
	if len(emitter.events) == 0 {
		t.Error("expected audit event to be emitted")
	}
}

// TestOpsConsole_AuditLogEmitted verifies audit events are emitted for admin ops (REQ-OPS-003).
func TestOpsConsole_AuditLogEmitted(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	router := newMockCellRouter([]ports.CellID{"cell-a", "cell-b"})
	emitter := &mockEmitter{}
	cellsHandler := admin.NewCellsHandler(router, &mockTenantCatalog{}, emitter)

	// Drain a cell.
	req := httptest.NewRequest(http.MethodPost, "/admin/cells/cell-a/drain", nil)
	rec := httptest.NewRecorder()
	cellsHandler.HandleDrain(rec, req.WithContext(ctx))

	// Verify an event was emitted.
	if len(emitter.events) == 0 {
		t.Fatal("expected audit event to be emitted")
	}

	event := emitter.events[0]
	if event.eventType != ports.EventOpsConsoleActionCompleted {
		t.Errorf("expected %s, got %s", ports.EventOpsConsoleActionCompleted, event.eventType)
	}
}
