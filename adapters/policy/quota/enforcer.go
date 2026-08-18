// Package quota provides per-tenant quota enforcement as part of the policy adapters.
package quota

import (
	"context"
	"sync"

	"github.com/Rubentxu/golem/internal/ports"
)

// Enforcer implements per-tenant quota enforcement.
type Enforcer struct {
	delegate ports.QuotaEnforcer
	mode     ports.QuotaMode
	mu       sync.Mutex
}

// NewEnforcer creates a new QuotaEnforcer with the given mode.
func NewEnforcer(delegate ports.QuotaEnforcer, mode ports.QuotaMode) *Enforcer {
	return &Enforcer{
		delegate: delegate,
		mode:     mode,
	}
}

// Consume checks and consumes quota for a tenant.
func (e *Enforcer) Consume(ctx context.Context, tenantID, capability string, units int64) (ports.QuotaDecision, error) {
	if e.delegate == nil {
		// No-op delegate: allow all.
		return ports.QuotaDecision{Outcome: "allowed", Mode: e.mode}, nil
	}

	decision, err := e.delegate.Consume(ctx, tenantID, capability, units)
	if err != nil {
		return decision, err
	}

	// Apply enforcement mode.
	switch e.mode {
	case ports.QuotaModeHard:
		if decision.Outcome == "denied" {
			return ports.QuotaDecision{Outcome: "denied", Mode: e.mode}, nil
		}
	case ports.QuotaModeThrottle:
		if decision.Outcome == "denied" {
			return ports.QuotaDecision{
				Outcome:      "throttled",
				Mode:         e.mode,
				RetryAfterMs: 1000, // default retry-after
			}, nil
		}
	case ports.QuotaModeSoft:
		// Soft mode: always allow but log.
		return ports.QuotaDecision{Outcome: "allowed", Mode: e.mode}, nil
	}

	return decision, nil
}

// Refund returns consumed units to the tenant quota.
func (e *Enforcer) Refund(ctx context.Context, tenantID, capability string, units int64) error {
	if e.delegate == nil {
		return nil
	}
	return e.delegate.Refund(ctx, tenantID, capability, units)
}

// Limits returns the current quota limits for a tenant.
func (e *Enforcer) Limits(ctx context.Context, tenantID string) (map[string]int64, error) {
	if e.delegate == nil {
		return map[string]int64{}, nil
	}
	return e.delegate.Limits(ctx, tenantID)
}
