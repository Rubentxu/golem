# STRIDE/Data-Flow: Cell Context

**Bounded Context**: Cell (internal/domain/cell, adapters/cell/)
**Wave**: W8.20
**Author**: GOLEM architecture team
**Date**: 2026-08-18
**ADR Reference**: ADR-074, ADR-045

---

## Context Description

The Cell context manages data partitioning and routing. Cells are isolated partitions that host tenant data. CellRouter provides consistent hashing for tenant-to-cell assignment with configurable override. Cell lifecycle includes promotion, drainage, and demotion.

## Data Flow

```
[Tenant Request] ──Route──► [CellRouter]
    │                              │
    │                              ▼
    │                       [Jump Hash Ring]
    │                              │
    │                              ▼
    │              ┌───────────────┴───────────────┐
    │              ▼                               ▼
    │         [Cell A]                       [Cell B]
    │              │                               │
    │              └───────────────┬───────────────┘
    │                              │
    └──────────────────► [Cell-Partitioned Journal]
```

## STRIDE Analysis

### S — Spoofing
| Threat | Likelihood | Impact | Mitigation |
|--------|------------|--------|------------|
| Request routed to wrong cell | Low | High | Consistent hash; cell assignment deterministic |
| Cell impersonation in routing | Very Low | Critical | Cell identity verified via TLS |

### T — Tampering
| Threat | Likelihood | Impact | Mitigation |
|--------|------------|--------|------------|
| Cell routing table tampered | Very Low | Critical | Routing table versioned; immutability via Journal |
| Cross-cell data tampering | Very Low | Critical | Cell isolation enforced at storage layer |

### R — Repudiation
| Threat | Likelihood | Impact | Mitigation |
|--------|------------|--------|------------|
| Cell claims request never received | Low | Medium | Cell-level audit log |
| Operator modifies routing without record | Very Low | Critical | All routing changes journaled |

### I — Information Disclosure
| Threat | Likelihood | Impact | Mitigation |
|--------|------------|--------|------------|
| Cross-cell data leakage via routing | Very Low | Critical | Cell isolation; no shared state |
| Routing table leaks cell topology | Low | Medium | Routing table not exposed externally |

### D — Denial of Service
| Threat | Likelihood | Impact | Mitigation |
|--------|------------|--------|------------|
| Cell failure causes tenant outage | Medium | High | Multi-cell active-active; failover |
| Routing algorithm DoS | Low | Medium | Consistent hash is O(log n); no heavy computation |
| Drain attack (deliberate cell removal) | Low | High | Drain requires operator role; audit trail |

### E — Elevation of Privilege
| Threat | Likelihood | Impact | Mitigation |
|--------|------------|--------|------------|
| Cell migration bypasses authorization | Low | Critical | Cell migration requires operator role |
| Tenant data accessed from wrong cell | Very Low | Critical | Cell partition enforced |

## Mitigations Summary

| Control | Implementation |
|---------|----------------|
| Cell isolation | Storage partition per cell; no cross-cell access |
| Routing integrity | Jump hash with override table; versioned |
| Operator auth | RBAC enforced on cell operations |
| Audit | cell.promoted, cell.demoted, cell.routing.conflict events |
| Failover | Active-passive per cell; automatic promotion |
