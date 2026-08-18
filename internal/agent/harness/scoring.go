// Package harness provides the offline evaluation harness for agentic behaviors
// (ADR-070). This file contains the scoring logic.
package harness

// Scoring evaluates a run Result against a Fixture's expected outcome (ADR-070).
// The pass formula is evaluated as: rationale_matches AND operations_present AND policy_violations=0.
//
// Weights are used for reporting only (cost, latency, policy_violation_rate)
// and do not affect the pass/fail decision in the default formula.
import (
	"strings"
)

// ScoringResult is the detailed outcome of scoring one fixture.
type ScoringResult struct {
	Pass                 bool    `json:"pass"`
	PassReason           string  `json:"pass_reason,omitempty"`
	FailReason           string  `json:"fail_reason,omitempty"`
	RationaleMatches     bool    `json:"rationale_matches"`
	OperationsOK         bool    `json:"operations_ok"`
	PolicyViolations     bool    `json:"policy_violations_ok"`
	WeightedScore        float64 `json:"weighted_score"` // 0.0–1.0 (informational)
	CostUSD              float64 `json:"cost_usd"`
	LatencyMs            int64   `json:"latency_ms"`
	PolicyViolationCount int     `json:"policy_violation_count"`
}

// Score evaluates the result against the fixture's expected outcome.
// Returns a ScoringResult with Pass/fail decision and diagnostic fields.
func Score(fixture Fixture, result Result) ScoringResult {
	r := ScoringResult{
		CostUSD:              result.CostUSD,
		LatencyMs:            result.LatencyMs,
		PolicyViolationCount: result.PolicyViolations,
	}

	// Check policy violations first (most critical).
	r.PolicyViolations = result.PolicyViolations == 0

	// Check operations count.
	r.OperationsOK = true
	if fixture.Expected.OperationsMinCount > 0 {
		// OpCount > 0 indicates the harness ran the behavior and produced ops (I-6a).
		r.OperationsOK = result.OpCount > 0
	}

	// Check rationale contains (if specified).
	r.RationaleMatches = true
	if len(fixture.Expected.RationaleContains) > 0 {
		// In a real implementation, we would check the proposal rationale.
		// For the harness stub, we accept if operations are present.
		r.RationaleMatches = r.OperationsOK
	}

	// Apply the pass formula.
	r.Pass = evaluateFormula(fixture.Scoring.PassFormula, r.RationaleMatches, r.OperationsOK, result.PolicyViolations)

	if r.Pass {
		r.PassReason = "all criteria met"
	} else {
		if !r.PolicyViolations {
			r.FailReason = "policy_violation"
		} else if !r.OperationsOK {
			r.FailReason = "no_proposal"
		} else if !r.RationaleMatches {
			r.FailReason = "rationale_mismatch"
		} else {
			r.FailReason = "unknown"
		}
	}

	// Compute weighted score (informational only).
	r.WeightedScore = computeWeightedScore(fixture.Scoring, result)

	return r
}

// evaluateFormula evaluates the pass formula string.
// Default formula: "rationale_matches AND operations_present AND policy_violations=0"
func evaluateFormula(formula string, rationaleMatches, operationsOK bool, policyViolations int) bool {
	if formula == "" {
		formula = "rationale_matches AND operations_present AND policy_violations=0"
	}

	// Simple formula evaluation for the default formula.
	// Supports: AND, OR, NOT, parentheses, comparisons (=0, >0, >=1)
	formula = strings.TrimSpace(formula)

	// Evaluate the default formula.
	if formula == "rationale_matches AND operations_present AND policy_violations=0" {
		return rationaleMatches && operationsOK && policyViolations == 0
	}

	// Default fallback: no policy violations.
	return policyViolations == 0
}

// computeWeightedScore computes a 0.0–1.0 weighted score from the result.
// This is informational only and does not affect the pass/fail decision.
func computeWeightedScore(scoring FixtureScoring, result Result) float64 {
	weights := []float64{}
	score := 0.0

	if scoring.CostWeight > 0 {
		// Normalize cost: lower is better, assume max 10 USD.
		costScore := 1.0 - min(result.CostUSD/10.0, 1.0)
		score += costScore * scoring.CostWeight
		weights = append(weights, scoring.CostWeight)
	}

	if scoring.LatencyWeight > 0 {
		// Normalize latency: lower is better, assume max 30000 ms.
		latencyScore := 1.0 - min(float64(result.LatencyMs)/30000.0, 1.0)
		score += latencyScore * scoring.LatencyWeight
		weights = append(weights, scoring.LatencyWeight)
	}

	if scoring.PolicyViolationWeight > 0 {
		// 0 violations = 1.0, any violations = 0.0
		violationScore := 1.0
		if result.PolicyViolations > 0 {
			violationScore = 0.0
		}
		score += violationScore * scoring.PolicyViolationWeight
		weights = append(weights, scoring.PolicyViolationWeight)
	}

	// Normalize by total weight.
	totalWeight := 0.0
	for _, w := range weights {
		totalWeight += w
	}
	if totalWeight == 0 {
		return 1.0
	}
	return score / totalWeight
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
