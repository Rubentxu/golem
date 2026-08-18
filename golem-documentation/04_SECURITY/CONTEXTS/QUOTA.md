# STRIDE/Data-Flow: Quota Context

**Bounded Context**: Quota (internal/domain/quota, adapters/quota/)
**Wave**: W8.22
**Author**: GOLEM architecture team
**Date**: 2026-08-18
**ADR Reference**: ADR-045

---

## Context Description

The Quota context enforces resource consumption limits per tenant. Quotas track usage against limits for commands, storage, events, and API calls. When a quota is exceeded, operations are rejected with RFC 7807 error.

## Data Flow

```
[Command/Query] ──check──► [Quota Enforcer]
    │                           │
    │                           ▼
    │                    [Quota Adapter: memstore]
    │                           │
    │         ┌─────────────────┼─────────────────┐
    │         ▼                 ▼                 ▼
    │    [Within Limit]   [Near Limit]      [Exceeded]
    │         │                 │                 │
    │         └─────────────────┴─────────────────┘
    │                           │
    └──────────────────► [Allow / Reject]
```

## STRIDE Analysis

### S — Spoofing
| Threat | Likelihood | Impact | Mitigation |
|--------|------------|--------|------------|
| Quota bypass via fake tenant ID | Low | High | Tenant ID from authenticated token |
| Quota consumption attributed to wrong tenant | Very Low | Medium | Per-tenant quota accounting |

### T — Tampering
| Threat | Likelihood | Impact | Mitigation |
|--------|------------|--------|------------|
| Quota counter manipulated | Low | High | Quota events journaled; counters derived from Journal |
| Quota limit changed without authorization | Low | High | RBAC on quota limit changes |

### R — Repudiation
| Threat | Likelihood | Impact | Mitigation |
|--------|------------|--------|------------|
| Tenant denies exceeding quota | Low | Medium | Quota usage logged with timestamps |
| Operator claims quota was not enforced | Very Low | Medium | Quota rejection events journaled |

### I — Information Disclosure
| Threat | Likelihood | Impact | Mitigation |
|--------|------------|--------|------------|
| Quota limits leaked to competitors | Low | Medium | Quota queries require operator role |
| Usage patterns reveal business activity | Low | Low | Per-tenant aggregation only |

### D — Denial of Service
| Threat | Likelihood | Impact | Mitigation |
|--------|------------|--------|------------|
| Quota exhaustion causes legitimate tenant outage | Medium | High | Gradual rejection; warning events before hard limit |
| Quota check becomes bottleneck | Low | Medium | Quota adapter optimized; caching |

### E — Elevation of Privilege
| Threat | Likelihood | Impact | Mitigation |
|--------|------------|--------|------------|
| Quota bypass via internal API | Very Low | Critical | Internal APIs also subject to quota checks |
| Tenant exceeds quota via race condition | Low | Medium | Atomic quota operations |

## Mitigations Summary

| Control | Implementation |
|---------|----------------|
| AuthN required | All quota checks require valid token |
| Tenant isolation | Quota counters per-tenant |
| Audit | Quota exhaustion events emitted |
| Gradual rejection | Warning before hard limit |
| Atomic ops | Quota updates are atomic |
