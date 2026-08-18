# STRIDE/Data-Flow: Metering Context

**Bounded Context**: Metering (internal/domain/metering, adapters/metering/)
**Wave**: W8.23
**Author**: GOLEM architecture team
**Date**: 2026-08-18
**ADR Reference**: ADR-045

---

## Context Description

The Metering context tracks and reports resource usage for billing and compliance. Metering collects usage events, aggregates them, and exposes metering queries. Usage data feeds the billing system and SLA reporting.

## Data Flow

```
[Operations] ──meter──► [Metering Collector]
    │                         │
    │                         ▼
    │                  [Usage Events]
    │                         │
    │                         ▼
    │                  [Rollup Aggregator]
    │                         │
    │                         ▼
[Metering Query API] ◄─── [Aggregated Usage]
```

## STRIDE Analysis

### S — Spoofing
| Threat | Likelihood | Impact | Mitigation |
|--------|------------|--------|------------|
| Fake usage events for free tier | Medium | Medium | Events signed by command handler |
| Metering data attributed to wrong tenant | Very Low | High | Tenant ID from authenticated context |

### T — Tampering
| Threat | Likelihood | Impact | Mitigation |
|--------|------------|--------|------------|
| Usage events modified after collection | Very Low | High | Events immutable once collected |
| Aggregated data manipulated | Very Low | Medium | Aggregation derived deterministically from events |

### R — Repudiation
| Threat | Likelihood | Impact | Mitigation |
|--------|------------|--------|------------|
| Tenant claims usage was different | Low | Medium | Usage events immutable; auditable |
| Billing system disputes meter data | Low | High | Metering query endpoint with Journal backing |

### I — Information Disclosure
| Threat | Likelihood | Impact | Mitigation |
|--------|------------|--------|------------|
| Usage patterns reveal business secrets | Medium | Medium | Only aggregated data exposed; raw events admin-only |
| Cross-tenant usage comparison | Low | Medium | Tenant filter on all metering queries |

### D — Denial of Service
| Threat | Likelihood | Impact | Mitigation |
|--------|------------|--------|------------|
| Usage event flood | Medium | Medium | Per-tenant rate limits |
| Metering query becomes resource intensive | Low | Medium | Query timeouts; aggregation limits |

### E — Elevation of Privilege
| Threat | Likelihood | Impact | Mitigation |
|--------|------------|--------|------------|
| Raw metering events accessed by non-admin | Low | High | Raw events require admin role |
| Metering API exposes internal metrics | Very Low | Medium | Only billable usage exposed externally |

## Mitigations Summary

| Control | Implementation |
|---------|----------------|
| Event immutability | Once collected, metering events cannot be modified |
| Tenant isolation | All queries filtered by tenant |
| Admin-only raw data | Raw events accessible only to operator role |
| Billing accuracy | Aggregation deterministic from events |
| Rate limits | Per-tenant metering rate limits |
