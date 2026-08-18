# SLO Framework

**M8 GA — v1.0.0**

This document describes the Service Level Objectives (SLOs) for the GOLEM platform,
the 13 SLIs tracked, error budget policies, and burn rate alerting thresholds.

## Overview

The SLO framework provides quantifiable reliability targets for all platform components.
Each SLO is tracked via the `SLOTracker` port and evaluated every 60 seconds by the
`Evaluator` service.

## SLI Definitions (13 total)

| # | SLI Name | Description | Target | Window | Unit |
|---|----------|-------------|--------|--------|------|
| 1 | `command.latency.p99` | Command API p99 latency | < 250ms | 1h rolling | ms |
| 2 | `command.error_rate` | Command API error rate | < 0.1% | 1h rolling | percent |
| 3 | `system.availability` | System availability | ≥ 99.9% | 30d | percent |
| 4 | `journal.replay_time.p99` | Journal replay p99 time | < 50ms | 1h rolling | ms |
| 5 | `agent.eval_pass_rate` | Agent evaluation pass rate | ≥ 80% | 1h rolling | percent |
| 6 | `oidc.verify_latency.p99` | OIDC token verify p99 latency | < 100ms | 1h rolling | ms |
| 7 | `ops.console.action_latency.p99` | Ops console action p99 latency | < 500ms | 1h rolling | ms |
| 8 | `audit.export.success_rate` | Audit export success rate | ≥ 99% | 1h rolling | percent |
| 9 | `metering.rollup.success_rate` | Metering rollup success rate | ≥ 99.5% | 1h rolling | percent |
| 10 | `quota.check_latency.p99` | Quota check p99 latency | < 100ms | 1h rolling | ms |
| 11 | `cell.migrate.duration.p99` | Cell migration p99 duration | < 500ms | 24h | ms |
| 12 | `snapshot.duration.p99` | DR snapshot p99 duration | < 5min | 24h | ms |
| 13 | `meter.query_latency.p99` | Metering query p99 latency | < 200ms | 1h rolling | ms |

## Error Budget Policy

Each SLO has an error budget derived from its target:

```
error_budget = (1 - target) × window_duration
```

Example: `command.latency.p99` with target 0.999 and 1h window:
- Allowed error rate: 0.1%
- Error budget per window: 0.1% of events

Budget consumption is computed as:

```
budget_consumed = error_rate / error_budget
```

Where `error_rate = bad_events / total_events` (bad = value < target).

## Burn Rate Alerting

Burn rate is computed as:

```
burn_rate = error_rate / allowed_error_rate
```

Alert thresholds (ADR-080 §3):

| Condition | Alert | Severity |
|-----------|-------|----------|
| Burn rate > 2x (1h window) | `slo.budget.burn.v1` event + page on-call | High |
| Burn rate > 2x (6h window) | `slo.budget.burn.v1` event + escalate | High |
| Budget exhausted > 90% | `slo.budget.exhausted.v1` event | Critical |
| Budget exhausted 100% | SLO violated — page critical | Critical |

## Instrumentation

All SLIs are instrumented via the OTel SDK:

- **Counters**: For discrete events (error counts, success counts)
- **Histograms**: For latency distributions (p50/p95/p99 via histogram quantiles)
- **OTLP export**: When `OTEL_EXPORTER_OTLP_ENDPOINT` is set
- **Prometheus**: Via `/metrics` endpoint (prometheus-client-golang)

### Histogram Buckets

Default buckets for latency SLIs:

```go
{0.5, 1, 2, 5, 10, 20, 50, 100, 200, 500, 1000, 2000, 5000, 10000, 30000, 60000}
```

P50/P95/P99 optimized buckets:

```go
{5, 10, 25, 50, 100, 150, 200, 250, 300, 400, 500, 750, 1000}
```

## Dashboard

The SLO dashboard (W6.18) displays:

- Burn rate trends per SLI
- Error budget consumption over time
- Violation history
- Current status (green/yellow/red)

## References

- ADR-080: SLO Framework
- REQ-SLO-001 through REQ-SLO-004
- W6 implementation tasks: W6.1–W6.18
