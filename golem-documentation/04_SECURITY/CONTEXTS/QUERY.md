# STRIDE/Data-Flow: Query Context

**Bounded Context**: Query (internal/application/projection)
**Wave**: W8.12
**Author**: GOLEM architecture team
**Date**: 2026-08-18
**ADR Reference**: ADR-005, ADR-045

---

## Context Description

The Query context handles all read operations in GOLEM. Queries are side-effect-free operations that read from projections (materialized views of the Journal). Queries are submitted via HTTP edge and answered by the projection layer.

## Data Flow

```
[Client]
    │ HTTP GET /queries
    ▼
[HTTP Edge] ──authN──► [Query Handler]
    │                       │
    │                       ▼
    │                [Projection Store]
    │                       │
    │                       ▼
    └───────────► [JSON Response]
```

## STRIDE Analysis

### S — Spoofing
| Threat | Likelihood | Impact | Mitigation |
|--------|------------|--------|------------|
| Client queries another tenant's data | Medium | High | Tenant isolation enforced by projection filter |
| Query as suspended tenant | Low | Medium | Token validation includes tenant status check |

### T — Tampering
| Threat | Likelihood | Impact | Mitigation |
|--------|------------|--------|------------|
| Query parameter injection | Low | Medium | Parameterized queries; no raw SQL |
| Response manipulation at proxy | Very Low | Medium | TLS end-to-end |

### R — Repudiation
| Threat | Likelihood | Impact | Mitigation |
|--------|------------|--------|------------|
| User denies reading data | Low | Low | Projection reads are not journaled (by design) |
| Operator claims data was different | Low | Medium | Projections are deterministic from Journal |

### I — Information Disclosure
| Threat | Likelihood | Impact | Mitigation |
|--------|------------|--------|------------|
| Cross-tenant data leakage via timing | Low | High | Tenant partitions isolated; no shared state |
| Verbose query errors reveal schema | Low | Medium | Errors sanitized at HTTP edge |

### D — Denial of Service
| Threat | Likelihood | Impact | Mitigation |
|--------|------------|--------|------------|
| Expensive query exhausts memory | Medium | Medium | Query timeouts; result size limits |
| Large result sets cause OOM | Medium | Medium | Pagination enforced |

### E — Elevation of Privilege
| Threat | Likelihood | Impact | Mitigation |
|--------|------------|--------|------------|
| Query bypasses row-level security | Low | Critical | Projection layer enforces tenant filter |
| Index exploitation for data exfiltration | Very Low | High | No raw SQL; ORM layer only |

## Mitigations Summary

| Control | Implementation |
|---------|----------------|
| Tenant isolation | All projections filtered by tenant_id from token |
| Query limits | Timeout + max result set size |
| Input validation | Query parameters validated against schema |
| TLS | All HTTP traffic encrypted |
