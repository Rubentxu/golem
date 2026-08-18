package migrator

import (
	"context"
	"testing"

	"github.com/Rubentxu/golem/internal/ports"
)

// TestMigrator_DryRun_NoMutation verifies dry-run does not mutate data plane.
func TestMigrator_DryRun_NoMutation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// Create a mock delegate router.
	delegate := &mockRouter{}
	migrator := NewMigrator(delegate)

	plan := ports.MigrationPlan{
		TenantID: "tenant-123",
		FromCell: "cell-a",
		ToCell:   "cell-b",
		DryRun:   true,
	}

	err := migrator.Migrate(ctx, plan)
	if err != nil {
		t.Fatalf("Migrate dry-run error: %v", err)
	}

	// Verify dry-run did not modify delegate.
	if delegate.listCalled || delegate.healthCalled {
		t.Error("dry-run should not call delegate List or Health")
	}
}

// mockRouter is a minimal CellRouter for testing.
type mockRouter struct {
	listCalled   bool
	healthCalled bool
}

func (m *mockRouter) Route(ctx context.Context, tenantID string) (ports.CellID, error) {
	return "cell-a", nil
}

func (m *mockRouter) Migrate(ctx context.Context, plan ports.MigrationPlan) error {
	return nil
}

func (m *mockRouter) List(ctx context.Context) ([]ports.CellRecord, error) {
	m.listCalled = true
	return nil, nil
}

func (m *mockRouter) Health(ctx context.Context, cellID ports.CellID) (ports.CellHealth, error) {
	m.healthCalled = true
	return ports.CellHealth{}, nil
}

// Compile-time interface check.
var _ ports.CellRouter = (*Migrator)(nil)
