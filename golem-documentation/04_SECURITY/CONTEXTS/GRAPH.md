# STRIDE/Data-Flow: Graph Context

**Bounded Context**: Graph (internal/domain/graph, adapters/graph/)
**Wave**: W8.16
**Author**: GOLEM architecture team
**Date**: 2026-08-18
**ADR Reference**: ADR-005, ADR-045

---

## Context Description

The Graph context provides a queryable graph of entities and relationships. It ingests events from the Journal and exposes graph traversal queries. Used for entity correlation, lineage, and relationship queries.

## Data Flow

```
[Journal] ──ingest──► [Graph Builder]
    │                       │
    │                       ▼
    │                [Graph Store: memstore/adapters]
    │                       │
    │                       ▼
[Graph Query API] ◄─── [Traversal Engine]
```

## STRIDE Analysis

### S — Spoofing
| Threat | Likelihood | Impact | Mitigation |
|--------|------------|--------|------------|
| Graph queries as another tenant | Medium | High | Tenant isolation on all graph queries |
| Fake node injection via event | Very Low | High | Events validated before graph ingestion |

### T — Tampering
| Threat | Likelihood | Impact | Mitigation |
|--------|------------|--------|------------|
| Graph node/edge tampered | Low | Medium | Graph is derived from Journal; rebuildable |
| Malicious query modifies graph | Very Low | High | Graph queries are read-only |

### R — Repudiation
| Threat | Likelihood | Impact | Mitigation |
|--------|------------|--------|------------|
| User denies relationship existence | Low | Low | Graph is deterministic from Journal |
| Operator claims graph was different | Low | Medium | Graph rebuild from Journal always possible |

### I — Information Disclosure
| Threat | Likelihood | Impact | Mitigation |
|--------|------------|--------|------------|
| Graph traversal reveals hidden relationships | Medium | High | Tenant filter on all queries |
| Sensitive entity properties in graph | Medium | High | Property-level access control |
| Graph structure leaks organizational info | Low | Medium | Query depth limits; result size limits |

### D — Denial of Service
| Threat | Likelihood | Impact | Mitigation |
|--------|------------|--------|------------|
| Deep graph traversal causes timeout | Medium | Medium | Query depth limits; deadline enforcement |
| Graph size explosion from event flood | Medium | High | Graph build rate limits |

### E — Elevation of Privilege
| Threat | Likelihood | Impact | Mitigation |
|--------|------------|--------|------------|
| Graph query bypasses authorization | Low | Critical | Graph queries filtered by tenant_id |
| Exploiting graph for lateral movement | Very Low | High | No direct entity access; only traversal |

## Mitigations Summary

| Control | Implementation |
|---------|----------------|
| Tenant isolation | All graph queries filtered by tenant |
| Query limits | Depth limits, result size limits, deadlines |
| Property access | Schema-enforced property visibility |
| Rebuild capability | Graph always rebuildable from Journal |
