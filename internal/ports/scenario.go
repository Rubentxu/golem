package ports

import (
	"context"
	"encoding/json"
	"time"
)

// Scenario is the fork model of SCENARIOS_FORK_DIFF_PROMOTE.md: a base
// journal position plus overlay events (the delta — the full graph is
// never copied), optional behavior overrides and a budget.
type Scenario struct {
	ID           string
	TenantID     TenantID
	BasePosition StreamPosition
	Overlay      []json.RawMessage // overlay events, applied in order
	Overrides    map[string]string // behavior version overrides (id → version)
	Approved     bool
	CreatedAt    time.Time
}

// ScenarioStore persists scenario definitions (overlay deltas). The
// memstore adapter is the v1 implementation; a file-backed adapter arrives
// in M6.1 (the port exists so that change does not ripple).
type ScenarioStore interface {
	Save(ctx context.Context, s *Scenario) error
	Load(ctx context.Context, id string) (*Scenario, error)
	Delete(ctx context.Context, id string) error
}
