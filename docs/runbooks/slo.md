# SLO Tracking Runbook

## Overview

This runbook covers the procedure for configuring and monitoring Service Level Objectives (SLOs).

## SLO Modes

| Mode | Description |
|------|-------------|
| `disabled` | No tracking |
| `audit` | Track but don't enforce |
| `soft` | Emit warnings |
| `hard` | Block on violation |

## Configuration

Register SLOs for tracking:

```go
tracker := slo.NewTracker()
tracker.RegisterSLO(ports.SLO{
    Name:         "availability",
    Target:       0.999,      // 99.9% uptime
    WindowHours:  168,        // 7 days
    ErrorBudget:  0.1,        // 10% error budget
})
```

## Recording Metrics

```go
// Record successful operation
_ = tracker.Record(ctx, "availability", 1.0)

// Record failed operation
_ = tracker.Record(ctx, "availability", 0.0)
```

## Evaluating SLOs

```go
violations, err := tracker.Evaluate(ctx)
if len(violations) > 0 {
    // Alert: SLO violated
}
```

## Events

SLO violations emit the following events:
- `slo.violation.detected.v1` when error budget is consumed

## References

- ADR-077: SLO Enforcement Modes
- REQ-SLO-001 through REQ-SLO-003
