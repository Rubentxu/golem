# STRIDE/Data-Flow: TenantCatalog Context

**Bounded Context**: TenantCatalog (internal/domain/tenant/catalog.go)
**Wave**: W8.21
**Author**: GOLEM architecture team
**Date**: 2026-08-18
**ADR Reference**: ADR-045

---

## Context Description

The TenantCatalog context manages tenant metadata, relationships, and configurations. It provides tenant resolution, cell assignment, and tenant lifecycle management. The catalog is the control plane for multi-tenancy.

## Data Flow

```
[Admin API] ──manage──► [TenantCatalog]
    │                         │
    │                         ▼
    │                  [Catalog Store]
    │                         │
    │                         ▼
[CellRouter] ◄──query── [Tenant Resolution]
    │
    └──────────────► [Tenant Metadata]
```

## STRIDE Analysis

### S — Spoofing
| Threat | Likelihood | Impact | Mitigation |
|--------|------------|--------|------------|
| Malicious tenant registration | Medium | High | Admin approval required; identity verification |
| Tenant impersonation in API | Medium | High | OIDC token required; tenant_id from token |

### T — Tampering
| Threat | Likelihood | Impact | Mitigation |
|--------|------------|--------|------------|
| Tenant metadata tampered | Low | High | Catalog writes journaled; immutable |
| Cell assignment modified | Low | Medium | Assignment changes tracked in Journal |

### R — Repudiation
| Threat | Likelihood | Impact | Mitigation |
|--------|------------|--------|------------|
| Tenant denies configuration change | Low | Medium | All changes logged with principal |
| Admin denies disabling tenant | Low | High | Admin actions journaled with operator identity |

### I — Information Disclosure
| Threat | Likelihood | Impact | Mitigation |
|--------|------------|--------|------------|
| Tenant list exposed to unauthorized | Medium | High | Catalog queries require operator role |
| Cross-tenant metadata leaked | Very Low | Critical | Tenant isolation enforced |

### D — Denial of Service
| Threat | Likelihood | Impact | Mitigation |
|--------|------------|--------|------------|
| Tenant registration flood | Medium | Medium | Registration rate limits |
| Catalog query exhaustion | Low | Medium | Query caching; read replicas |

### E — Elevation of Privilege
| Threat | Likelihood | Impact | Mitigation |
|--------|------------|--------|------------|
| Tenant escalates to admin | Very Low | Critical | RBAC enforced; operator role required |
| Cross-tenant cell assignment | Very Low | Critical | Cross-tenant operations journaled; RBAC |

## Mitigations Summary

| Control | Implementation |
|---------|----------------|
| Admin auth | RBAC: operator role for all catalog mutations |
| Tenant isolation | Catalog queries scoped to authorized tenant |
| Audit | All catalog changes journaled |
| Rate limits | Per-tenant registration throttling |
| Read replicas | Catalog read scaling |
