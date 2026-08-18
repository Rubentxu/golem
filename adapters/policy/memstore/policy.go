// Package memstore provides a policy adapter with default-deny for agents.
package memstore

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/Rubentxu/golem/internal/ports"
)

// PolicyStore implements ports.PolicyEvaluator with default-deny for agents.
// It provides a simple in-memory policy evaluation with glob pattern support.
type PolicyStore struct {
	// rules maps (actorType, targetPattern) to decision
	rules map[string]map[string]ports.DecisionOutcome
}

// New creates a new PolicyStore with default-deny for agents.
func New() *PolicyStore {
	return &PolicyStore{
		rules: make(map[string]map[string]ports.DecisionOutcome),
	}
}

// Evaluate implements ports.PolicyEvaluator.
// Default: deny all actions by actors with Type="agent".
// Supports glob patterns in rules (e.g., "graph:node:*" matches "graph:node:test").
func (p *PolicyStore) Evaluate(ctx context.Context, action ports.Action) (ports.Decision, error) {
	// Check explicit rules first
	if targetRules, ok := p.rules[action.Actor.Type]; ok {
		// Try exact match first
		if outcome, ok := targetRules[action.Target]; ok {
			return ports.Decision{Outcome: outcome}, nil
		}
		// Try glob pattern match
		for pattern, outcome := range targetRules {
			if matchGlob(action.Target, pattern) {
				return ports.Decision{Outcome: outcome}, nil
			}
		}
	}

	// Default: deny for agents
	if action.Actor.Type == "agent" {
		return ports.Decision{Outcome: ports.DecisionOutcomeDeny}, nil
	}

	// For non-agents, default allow reads
	if action.Type == "read" {
		return ports.Decision{Outcome: ports.DecisionOutcomeAllow}, nil
	}

	// Other actions by non-agents require approval
	return ports.Decision{Outcome: ports.DecisionOutcomeApprovalRequired}, nil
}

// AddRule adds an explicit allow rule for an actor type and target.
// Supports glob patterns (e.g., "graph:node:*" matches any "graph:node:X").
func (p *PolicyStore) AddRule(actorType, target string, outcome ports.DecisionOutcome) {
	if p.rules[actorType] == nil {
		p.rules[actorType] = make(map[string]ports.DecisionOutcome)
	}
	p.rules[actorType][target] = outcome
}

// matchGlob reports whether s matches the glob pattern.
// Supports "*" which matches any sequence of characters.
func matchGlob(s, pattern string) bool {
	if pattern == "*" {
		return true
	}
	// Simple glob: split on '*' and check prefixes/suffixes
	parts := strings.Split(pattern, "*")
	if len(parts) == 1 {
		return s == pattern
	}
	// Check prefix
	if !strings.HasPrefix(s, parts[0]) {
		return false
	}
	// Check suffix
	if !strings.HasSuffix(s, parts[len(parts)-1]) {
		return false
	}
	return true
}

// Ensure PolicyStore implements PolicyEvaluator
var _ ports.PolicyEvaluator = (*PolicyStore)(nil)

// QuotaStore implements ports.QuotaEnforcer with in-memory tracking.
type QuotaStore struct {
	// quotas maps tenantID -> capability -> quota limit
	quotas map[string]map[string]int64
	// usage maps tenantID -> capability -> current usage
	usage map[string]map[string]*usageCounter
	// windows maps tenantID -> capability -> window start time
	windows map[string]map[string]time.Time
	mu      sync.Mutex
}

type usageCounter struct {
	count       int64
	windowStart time.Time
}

// NewQuotaStore creates a new QuotaStore.
func NewQuotaStore() *QuotaStore {
	return &QuotaStore{
		quotas:  make(map[string]map[string]int64),
		usage:   make(map[string]map[string]*usageCounter),
		windows: make(map[string]map[string]time.Time),
	}
}

// Consume checks and consumes quota for a tenant.
func (q *QuotaStore) Consume(ctx context.Context, tenantID, capability string, units int64) (ports.QuotaDecision, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	limit := q.getLimit(tenantID, capability)
	now := time.Now()

	// Reset window if expired (1 hour default).
	windowDuration := time.Hour
	if ws, ok := q.windows[tenantID][capability]; ok {
		if now.Sub(ws) > windowDuration {
			q.resetWindow(tenantID, capability, now)
		}
	} else {
		if q.windows[tenantID] == nil {
			q.windows[tenantID] = make(map[string]time.Time)
		}
		q.windows[tenantID][capability] = now
	}

	current := q.getUsage(tenantID, capability)
	if current+units > limit {
		return ports.QuotaDecision{
			Outcome: "denied",
			Mode:    ports.QuotaModeHard,
		}, nil
	}

	// Ensure usage entry exists.
	if q.usage[tenantID] == nil {
		q.usage[tenantID] = make(map[string]*usageCounter)
	}
	if q.usage[tenantID][capability] == nil {
		q.usage[tenantID][capability] = &usageCounter{windowStart: time.Now()}
	}

	// Consume.
	q.usage[tenantID][capability].count += units
	return ports.QuotaDecision{
		Outcome: "allowed",
		Mode:    ports.QuotaModeHard,
	}, nil
}

// Refund returns consumed units to the tenant quota.
func (q *QuotaStore) Refund(ctx context.Context, tenantID, capability string, units int64) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if uc, ok := q.usage[tenantID][capability]; ok {
		uc.count -= units
		if uc.count < 0 {
			uc.count = 0
		}
	}
	return nil
}

// Limits returns the current quota limits for a tenant.
func (q *QuotaStore) Limits(ctx context.Context, tenantID string) (map[string]int64, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	limits := make(map[string]int64)
	if caps, ok := q.quotas[tenantID]; ok {
		for cap, limit := range caps {
			limits[cap] = limit
		}
	}
	return limits, nil
}

// SetQuota sets the quota limit for a tenant capability.
func (q *QuotaStore) SetQuota(tenantID, capability string, limit int64) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.quotas[tenantID] == nil {
		q.quotas[tenantID] = make(map[string]int64)
	}
	q.quotas[tenantID][capability] = limit
}

func (q *QuotaStore) getLimit(tenantID, capability string) int64 {
	if caps, ok := q.quotas[tenantID]; ok {
		if limit, ok := caps[capability]; ok {
			return limit
		}
	}
	return 1000 // default limit
}

func (q *QuotaStore) getUsage(tenantID, capability string) int64 {
	if usage, ok := q.usage[tenantID]; ok {
		if uc, ok := usage[capability]; ok {
			return uc.count
		}
	}
	return 0
}

func (q *QuotaStore) resetWindow(tenantID, capability string, now time.Time) {
	if q.usage[tenantID] == nil {
		q.usage[tenantID] = make(map[string]*usageCounter)
	}
	q.usage[tenantID][capability] = &usageCounter{
		count:       0,
		windowStart: now,
	}
	if q.windows[tenantID] == nil {
		q.windows[tenantID] = make(map[string]time.Time)
	}
	q.windows[tenantID][capability] = now
}

// Ensure QuotaStore implements QuotaEnforcer
var _ ports.QuotaEnforcer = (*QuotaStore)(nil)
