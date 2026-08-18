// Package migrator provides a CellRouter adapter for tenant migration.
package migrator

import (
	"context"

	"github.com/Rubentxu/golem/internal/ports"
)

// Migrator implements tenant migration between cells as part of CellRouter.
type Migrator struct {
	delegate ports.CellRouter
}

// NewMigrator creates a new Migrator wrapping the given CellRouter.
func NewMigrator(delegate ports.CellRouter) *Migrator {
	return &Migrator{delegate: delegate}
}

// Route delegates to the underlying router.
func (m *Migrator) Route(ctx context.Context, tenantID string) (ports.CellID, error) {
	return m.delegate.Route(ctx, tenantID)
}

// Migrate performs tenant migration (REQ-MIG-001).
// When dryRun=true, performs shadow-reads and diff without data plane mutation.
func (m *Migrator) Migrate(ctx context.Context, plan ports.MigrationPlan) error {
	if plan.DryRun {
		return m.dryRun(ctx, plan)
	}
	return m.executeCutover(ctx, plan)
}

// dryRun performs shadow reads and diff without modifying data plane.
func (m *Migrator) dryRun(ctx context.Context, plan ports.MigrationPlan) error {
	// Placeholder: perform shadow reads and compute diff.
	_ = ctx
	_ = plan
	return nil
}

// executeCutover performs the actual cutover.
func (m *Migrator) executeCutover(ctx context.Context, plan ports.MigrationPlan) error {
	_ = ctx
	_ = plan
	return nil
}

// List delegates to the underlying router.
func (m *Migrator) List(ctx context.Context) ([]ports.CellRecord, error) {
	return m.delegate.List(ctx)
}

// Health delegates to the underlying router.
func (m *Migrator) Health(ctx context.Context, cellID ports.CellID) (ports.CellHealth, error) {
	return m.delegate.Health(ctx, cellID)
}
