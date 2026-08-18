// Package memstore provides a policy adapter with default-deny for agents.
package memstore

import (
	"context"
	"strings"

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
