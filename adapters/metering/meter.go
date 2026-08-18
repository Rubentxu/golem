// Package metering provides a usage metering adapter.
package metering

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"time"

	"github.com/Rubentxu/golem/internal/ports"
)

// Meter implements ports.UsageMeter with in-memory storage.
type Meter struct {
	events    []ports.MeteringEvent
	uptime    map[string]float64
	errBudget map[string]float64
	mu        sync.RWMutex
}

// NewMeter creates a new UsageMeter.
func NewMeter() *Meter {
	return &Meter{
		events:    make([]ports.MeteringEvent, 0, 10000),
		uptime:    make(map[string]float64),
		errBudget: make(map[string]float64),
	}
}

// Record implements ports.UsageMeter.
func (m *Meter) Record(ctx context.Context, event ports.MeteringEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.events = append(m.events, event)

	// Update uptime gauge (simplified: count success events).
	if event.CostUSD >= 0 {
		m.uptime[event.Capability] = 0.99 // placeholder
	}

	return nil
}

// Rollup implements ports.UsageMeter.
func (m *Meter) Rollup(ctx context.Context, hour time.Time) ([]ports.MeteringRollup, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	rollups := make([]ports.MeteringRollup, 0)
	byTenantCap := make(map[string]map[string]int64)
	byTenantCost := make(map[string]map[string]float64)

	// Aggregate events by tenant and capability.
	for _, e := range m.events {
		if e.Timestamp.Truncate(time.Hour) != hour.Truncate(time.Hour) {
			continue
		}
		if byTenantCap[e.TenantID] == nil {
			byTenantCap[e.TenantID] = make(map[string]int64)
			byTenantCost[e.TenantID] = make(map[string]float64)
		}
		byTenantCap[e.TenantID][e.Capability] += e.Units
		byTenantCost[e.TenantID][e.Capability] += e.CostUSD
	}

	// Build rollups with digest.
	for tenantID, caps := range byTenantCap {
		for cap, units := range caps {
			cost := byTenantCost[tenantID][cap]
			content := tenantID + cap + string(rune(int(units))) + string(rune(int(cost*100)))
			digest := sha256.Sum256([]byte(content))

			rollups = append(rollups, ports.MeteringRollup{
				TenantID:     tenantID,
				Hour:         hour.Truncate(time.Hour),
				Capability:   cap,
				TotalUnits:   units,
				TotalCostUSD: cost,
				Digest:       hex.EncodeToString(digest[:]),
			})
		}
	}

	return rollups, nil
}

// UptimeGauge implements ports.UsageMeter.
func (m *Meter) UptimeGauge(ctx context.Context, capability string) (float64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if uptime, ok := m.uptime[capability]; ok {
		return uptime, nil
	}
	return 0.0, nil
}

// ErrorBudgetGauge implements ports.UsageMeter.
func (m *Meter) ErrorBudgetGauge(ctx context.Context, capability string) (float64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if eb, ok := m.errBudget[capability]; ok {
		return eb, nil
	}
	return 0.0, nil
}

// Ensure Meter implements UsageMeter
var _ ports.UsageMeter = (*Meter)(nil)
