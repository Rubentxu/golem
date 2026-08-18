// Package slo provides an SLO tracker adapter.
package slo

import (
	"context"
	"sync"
	"time"

	"github.com/Rubentxu/golem/internal/ports"
)

// Tracker implements ports.SLOTracker with in-memory tracking.
type Tracker struct {
	slos   map[string]ports.SLO
	events []sloEvent
	mu     sync.RWMutex
}

type sloEvent struct {
	sloName string
	value   float64
	at      time.Time
}

// NewTracker creates a new SLOTracker.
func NewTracker() *Tracker {
	return &Tracker{
		slos:   make(map[string]ports.SLO),
		events: make([]sloEvent, 0, 1000),
	}
}

// Record implements ports.SLOTracker.
func (t *Tracker) Record(ctx context.Context, sloName string, value float64) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.events = append(t.events, sloEvent{
		sloName: sloName,
		value:   value,
		at:      time.Now(),
	})
	return nil
}

// Evaluate implements ports.SLOTracker.
func (t *Tracker) Evaluate(ctx context.Context) ([]ports.SLOViolation, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	violations := make([]ports.SLOViolation, 0)

	for name, slo := range t.slos {
		budgetConsumed := t.computeBudgetConsumed(name, slo)
		burnRate := t.computeBurnRate(name, slo)

		if budgetConsumed >= slo.ErrorBudget {
			violations = append(violations, ports.SLOViolation{
				SLOName:        name,
				BudgetConsumed: budgetConsumed,
				BurnRate:       burnRate,
			})
		}
	}

	return violations, nil
}

// RegisterSLO registers an SLO for tracking.
func (t *Tracker) RegisterSLO(slo ports.SLO) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.slos[slo.Name] = slo
}

func (t *Tracker) computeBudgetConsumed(sloName string, slo ports.SLO) float64 {
	// Placeholder: compute error budget consumed.
	// In a real implementation, this would analyze events within the window.
	count := 0
	total := 0
	for _, e := range t.events {
		if e.sloName == sloName {
			total++
			if e.value < slo.Target {
				count++
			}
		}
	}
	if total == 0 {
		return 0
	}
	return float64(count) / float64(total)
}

func (t *Tracker) computeBurnRate(sloName string, slo ports.SLO) float64 {
	// Placeholder: compute burn rate.
	return t.computeBudgetConsumed(sloName, slo)
}

// Ensure Tracker implements SLOTracker
var _ ports.SLOTracker = (*Tracker)(nil)
