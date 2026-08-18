# Quota Enforcement Runbook

## Overview

This runbook covers the procedure for configuring and managing per-tenant quota enforcement.

## Quota Modes

| Mode | Description |
|------|-------------|
| `soft` | Emit warning but allow operation |
| `throttle` | Delay operation with Retry-After header |
| `hard` | Immediately deny operation |

## Configuration

Set quota limits per tenant:

```go
store := memstore.NewQuotaStore()
store.SetQuota("tenant-123", "events", 10000)
store.SetQuota("tenant-123", "storage", 1_000_000_000)
```

## Enforcement

```go
enforcer := quota.NewEnforcer(store, ports.QuotaModeHard)

decision, err := enforcer.Consume(ctx, "tenant-123", "events", 1)
if decision.Outcome == "denied" {
    // Handle quota exceeded
}
```

## Monitoring

Check quota usage via the Limits API:

```go
limits, err := enforcer.Limits(ctx, "tenant-123")
```

## References

- ADR-076: Per-Tenant Quotas
- REQ-QUOTA-001 through REQ-QUOTA-003
