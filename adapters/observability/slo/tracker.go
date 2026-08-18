// Package slo provides an SLO tracker adapter.
package slo

import (
	"context"
	"sync"
	"time"

	"github.com/Rubentxu/golem/internal/ports"
)

// BurnRateAlert represents a burn rate alert (>2x threshold).
type BurnRateAlert struct {
	SLOName     string
	BurnRate    float64
	WindowHours int
	BudgetLeft  float64
	ErrorRate   float64
	AllowedRate float64
}

// ExhaustedAlert represents an exhausted budget alert (>90% consumed).
type ExhaustedAlert struct {
	SLOName         string
	BudgetConsumed  float64
	BudgetRemaining float64
	WindowHours     int
}

// Tracker implements ports.SLOTracker with in-memory tracking.
type Tracker struct {
	slos            map[string]ports.SLO
	events          []sloEvent
	mu              sync.RWMutex
	burnRateAlerts  []BurnRateAlert
	exhaustedAlerts []ExhaustedAlert
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

	t.burnRateAlerts = nil
	t.exhaustedAlerts = nil
	violations := make([]ports.SLOViolation, 0)

	for name, slo := range t.slos {
		budgetConsumed := t.computeBudgetConsumed(name, slo)
		burnRate := t.computeBurnRate(name, slo)
		allowedErrorRate := 1.0 - slo.Target
		budgetRemaining := 1.0 - budgetConsumed

		// Check burn rate > 2x threshold (ADR-080 §3)
		if burnRate > 2.0 {
			t.burnRateAlerts = append(t.burnRateAlerts, BurnRateAlert{
				SLOName:     name,
				BurnRate:    burnRate,
				WindowHours: slo.WindowHours,
				BudgetLeft:  budgetRemaining,
				ErrorRate:   burnRate * allowedErrorRate,
				AllowedRate: allowedErrorRate,
			})
		}

		// Check budget exhausted > 90% (ADR-080 §3)
		if budgetConsumed > 0.9 {
			t.exhaustedAlerts = append(t.exhaustedAlerts, ExhaustedAlert{
				SLOName:         name,
				BudgetConsumed:  budgetConsumed,
				BudgetRemaining: budgetRemaining,
				WindowHours:     slo.WindowHours,
			})
		}

		if budgetConsumed >= 1.0 {
			violations = append(violations, ports.SLOViolation{
				SLOName:        name,
				BudgetConsumed: budgetConsumed,
				BurnRate:       burnRate,
			})
		}
	}

	return violations, nil
}

// BurnRateAlerts returns burn rate alerts from the last Evaluate call.
func (t *Tracker) BurnRateAlerts() []BurnRateAlert {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.burnRateAlerts
}

// ExhaustedAlerts returns exhausted budget alerts from the last Evaluate call.
func (t *Tracker) ExhaustedAlerts() []ExhaustedAlert {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.exhaustedAlerts
}

// RegisterSLO registers an SLO for tracking.
func (t *Tracker) RegisterSLO(slo ports.SLO) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.slos[slo.Name] = slo
}

func (t *Tracker) computeBudgetConsumed(sloName string, slo ports.SLO) float64 {
	// Compute error budget consumed within the SLO window.
	// Formula: budget_consumed = error_rate / error_budget
	// error_rate = bad_events / total_events (where bad = value < target)
	// If error_rate equals error_budget, budget_consumed = 1.0 (100%)
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
	if total == 0 || slo.ErrorBudget == 0 {
		return 0
	}
	errorRate := float64(count) / float64(total)
	return errorRate / slo.ErrorBudget
}

func (t *Tracker) computeBurnRate(sloName string, slo ports.SLO) float64 {
	// Burn rate = error_rate / allowed_error_rate
	// allowed_error_rate = 1 - target
	// e.g., target=0.999 → allowed_error_rate=0.001
	// burn_rate = 2.0 means we're burning error budget 2x faster than allowed
	if slo.Target >= 1.0 {
		return 0
	}
	allowedErrorRate := 1.0 - slo.Target
	if allowedErrorRate == 0 {
		return 0
	}

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
	errorRate := float64(count) / float64(total)
	return errorRate / allowedErrorRate
}

// Ensure Tracker implements SLOTracker
var _ ports.SLOTracker = (*Tracker)(nil)
