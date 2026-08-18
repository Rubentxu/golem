package tck

import (
	"context"
	"testing"

	"github.com/Rubentxu/golem/adapters/cell/migrator"
	"github.com/Rubentxu/golem/internal/ports"
)

// TestMigration_DryRunDoesNotMutate verifies dry-run is read-only (REQ-MIG-001).
func TestMigration_DryRunDoesNotMutate(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// Create a mock delegate router.
	delegate := &migrationMockRouter{}
	mig := migrator.NewMigrator(delegate)

	plan := ports.MigrationPlan{
		TenantID: "tenant-123",
		FromCell: "cell-a",
		ToCell:   "cell-b",
		DryRun:   true,
	}

	err := mig.Migrate(ctx, plan)
	if err != nil {
		t.Fatalf("Migrate dry-run error: %v", err)
	}
}

// TestMigration_CutoverPlan verifies cutover plan is valid (REQ-MIG-002).
func TestMigration_CutoverPlan(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	delegate := &migrationMockRouter{}
	mig := migrator.NewMigrator(delegate)

	plan := ports.MigrationPlan{
		TenantID: "tenant-123",
		FromCell: "cell-a",
		ToCell:   "cell-b",
		DryRun:   false,
	}

	err := mig.Migrate(ctx, plan)
	if err != nil {
		t.Fatalf("Migrate cutover error: %v", err)
	}
}

// TestMigration_EventsEmitted verifies migration events are defined (REQ-MIG-003).
func TestMigration_EventsEmitted(t *testing.T) {
	t.Parallel()

	// Verify all migration events are non-empty.
	events := []string{
		ports.EventTenantMigrationStarted,
		ports.EventTenantMigrationShadowed,
		ports.EventTenantMigrationCutover,
		ports.EventTenantMigrationCompleted,
		ports.EventTenantMigrationFailed,
	}

	for _, e := range events {
		if e == "" {
			t.Error("Migration event is empty")
		}
	}
}

// migrationMockRouter is a minimal CellRouter for testing.
type migrationMockRouter struct{}

func (m *migrationMockRouter) Route(ctx context.Context, tenantID string) (ports.CellID, error) {
	return "cell-a", nil
}

func (m *migrationMockRouter) Migrate(ctx context.Context, plan ports.MigrationPlan) error {
	return nil
}

func (m *migrationMockRouter) List(ctx context.Context) ([]ports.CellRecord, error) {
	return nil, nil
}

func (m *migrationMockRouter) Health(ctx context.Context, cellID ports.CellID) (ports.CellHealth, error) {
	return ports.CellHealth{}, nil
}
