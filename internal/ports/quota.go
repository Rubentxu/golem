package ports

import "context"

// QuotaMode defines how quota enforcement behaves (REQ-QUOTA-002).
type QuotaMode string

const (
	// QuotaModeSoft emits a warning but allows the operation.
	QuotaModeSoft QuotaMode = "soft"
	// QuotaModeThrottle delays the operation with Retry-After.
	QuotaModeThrottle QuotaMode = "throttle"
	// QuotaModeHard denies the operation immediately.
	QuotaModeHard QuotaMode = "hard"
)

// QuotaDecision is the outcome of quota checking (REQ-QUOTA-002..003).
type QuotaDecision struct {
	Outcome      string    `json:"outcome"` // "allowed", "denied", "throttled"
	Mode         QuotaMode `json:"mode"`
	RetryAfterMs int64     `json:"retry_after_ms,omitempty"`
}

// QuotaEnforcer checks and consumes quota before command execution (REQ-QUOTA-001..005).
//
// Consume deducts from the quota for a tenant and returns a decision.
// The mode (soft/throttle/hard) determines behavior when quota is exceeded.
type QuotaEnforcer interface {
	// Consume checks and consumes quota for a tenant.
	// Returns QuotaDecision with outcome and optional retry information.
	Consume(ctx context.Context, tenantID string, capability string, units int64) (QuotaDecision, error)
	// Refund returns consumed units to the tenant quota (e.g., on rollback).
	Refund(ctx context.Context, tenantID string, capability string, units int64) error
	// Limits returns the current quota limits for a tenant.
	Limits(ctx context.Context, tenantID string) (map[string]int64, error)
}
