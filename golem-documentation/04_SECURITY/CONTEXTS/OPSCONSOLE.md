# STRIDE/Data-Flow: OpsConsole Context

**Bounded Context**: OpsConsole (internal/api/httpapi/admin/, internal/application/admin/)
**Wave**: W8.26
**Author**: GOLEM architecture team
**Date**: 2026-08-18
**ADR Reference**: ADR-081, ADR-045

---

## Context Description

The OpsConsole context provides the administrative API for GOLEM operations. It exposes cell management, tenant management, SLO queries, metering, and DR operations. All OpsConsole endpoints require operator role and emit audit events.

## Data Flow

```
[Operator] ──HTTP──► [AuthN Middleware] ──verified──► [Admin Handler]
    │                   │                                  │
    │                   │                                  ▼
    │                   │                           [RBAC Check]
    │                   │                                  │
    │                   │                                  ▼
    │                   │                           [Operation Handler]
    │                   │                                  │
    │                   │                                  ▼
    │                   │                    ┌──────────────┴──────────────┐
    │                   │                    ▼                              ▼
    │                   │              [Domain Ops]                   [Audit Logger]
    │                   │                    │                              │
    │                   └────────────────────┴──────────────────────────────┘
    │                                        │
    └───────────────────────────► [Journal: ops.console.action.*]
```

## STRIDE Analysis

### S — Spoofing
| Threat | Likelihood | Impact | Mitigation |
|--------|------------|--------|------------|
| Rogue operator authentication | Medium | Critical | OIDC with MFA; operator role verification |
| Token theft/replay | Low | High | Short-lived tokens; token binding |

### T — Tampering
| Threat | Likelihood | Impact | Mitigation |
|--------|------------|--------|------------|
| Admin request tampered | Very Low | High | TLS; request signing optional |
| Audit log tampered | Very Low | Critical | Audit log immutable; Journal-backed |

### R — Repudiation
| Threat | Likelihood | Impact | Mitigation |
|--------|------------|--------|------------|
| Operator denies action | Very Low | Critical | All ops require valid operator token; audit logged |
| GOLEM claims operation not performed | Very Low | Critical | Ops journaled with principal, timestamp, correlation |

### I — Information Disclosure
| Threat | Likelihood | Impact | Mitigation |
|--------|------------|--------|------------|
| Admin API reveals tenant data | Low | High | Operator role required; tenant data access logged |
| Audit log exposed to unauthorized | Low | Critical | Audit log access requires operator role |
| Admin endpoint errors leak internals | Low | Medium | Error responses sanitized; RFC 7807 only |

### D — Denial of Service
| Threat | Likelihood | Impact | Mitigation |
|--------|------------|--------|------------|
| Admin endpoint DoS | Medium | High | Operator role required; rate limits per operator |
| Audit log flood | Low | Medium | Audit log rotation; storage quotas |

### E — Elevation of Privilege
| Threat | Likelihood | Impact | Mitigation |
|--------|------------|--------|------------|
| Non-operator accesses admin endpoints | Medium | Critical | RBAC enforced; operator role required |
| Operator escalates to super-operator | Very Low | Critical | Role hierarchy enforced; break-glass requires second operator |
| Cross-tenant operation by operator | Very Low | Critical | Cross-tenant ops require explicit tenant grant |

## Mitigations Summary

| Control | Implementation |
|---------|----------------|
| AuthN | OIDC Bearer token with MFA for operators |
| AuthZ | RBAC: golem.operator role required on all admin endpoints |
| Audit | ops.console.action.{completed,rejected} emitted for all admin operations |
| TLS | All admin endpoints require TLS 1.3 |
| Rate limits | Per-operator rate limiting |
| Error handling | RFC 7807Problem responses; no stack traces |
| Token binding | Short-lived tokens; refresh rotation |
