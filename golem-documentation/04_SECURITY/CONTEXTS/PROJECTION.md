# STRIDE/Data-Flow: Projection Context

**Bounded Context**: Projection (internal/application/projection)
**Wave**: W8.14
**Author**: GOLEM architecture team
**Date**: 2026-08-18
**ADR Reference**: ADR-005, ADR-045

---

## Context Description

The Projection context builds and maintains materialized views from the Journal. Projections are eventually consistent read models optimized for query patterns. They are rebuilt deterministically from the Journal on first load or after recovery.

## Data Flow

```
[Journal] ──subscribe──► [Projection Worker]
    │                           │
    │                           ▼
    │                    [Projection Store]
    │                           │
    │                           ▼
[Query Handler] ◄─────── [Read Model]
```

## STRIDE Analysis

### S — Spoofing
| Threat | Likelihood | Impact | Mitigation |
|--------|------------|--------|------------|
| Projection worker impersonation | Very Low | High | Internal service authentication; mTLS |
| Forged projection updates | Very Low | High | Updates come only from Journal subscription |

### T — Tampering
| Threat | Likelihood | Impact | Mitigation |
|--------|------------|--------|------------|
| Projection store tampered | Low | Medium | Storage encryption at rest; integrity checks |
| Stale data served as fresh | Low | Medium | Version vectors track Journal position |

### R — Repudiation
| Threat | Likelihood | Impact | Mitigation |
|--------|------------|--------|------------|
| Wrong data served to user | Low | Medium | Projections are deterministic; can be rebuilt |
| Operator claims projection was correct | Low | Low | Journal is source of truth; projection is derived |

### I — Information Disclosure
| Threat | Likelihood | Impact | Mitigation |
|--------|------------|--------|------------|
| Projection contains stale sensitive data | Low | Medium | Secrets never stored in projections |
| Cross-tenant data in projection | Very Low | Critical | Tenant filter on all projection reads |

### D — Denial of Service
| Threat | Likelihood | Impact | Mitigation |
|--------|------------|--------|------------|
| Projection rebuild causes resource spike | Low | Medium | Background rebuild with priority scheduling |
| Projection store unavailable | Medium | High | Read replicas; eventual consistency acceptable |

### E — Elevation of Privilege
| Threat | Likelihood | Impact | Mitigation |
|--------|------------|--------|------------|
| Projection bypasses access control | Very Low | Critical | Projections are read-only; filtered by tenant |
| Malicious projection filters data | Very Low | Critical | Projections are code-signed; TCB controlled |

## Mitigations Summary

| Control | Implementation |
|---------|----------------|
| Tenant isolation | All projection reads filtered by tenant_id |
| Determinism | Projections rebuilt from Journal on demand |
| Secrets | Secrets never enter projection layer |
| mTLS | Internal service communication authenticated |
