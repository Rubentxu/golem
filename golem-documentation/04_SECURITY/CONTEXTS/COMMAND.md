# STRIDE/Data-Flow: Command Context

**Bounded Context**: Command (internal/application/command)
**Wave**: W8.11
**Author**: GOLEM architecture team
**Date**: 2026-08-18
**ADR Reference**: ADR-005, ADR-045

---

## Context Description

The Command context handles all write operations in GOLEM. Commands are deterministic, idempotent requests that mutate domain state via the Journal. Commands are submitted via HTTP edge or internal application layers and travel through the command bus.

## Data Flow

```
[Client]
    │ HTTP POST /commands
    ▼
[HTTP Edge] ──authN──► [Command Handler]
    │                       │
    │                       ▼
    │                [Command Bus]
    │                       │
    │                       ▼
    │                [Command Validator]
    │                       │
    │                       ▼
    │                [Domain Logic]
    │                       │
    │                       ▼
    └───────────► [Journal Append]
```

## STRIDE Analysis

### S — Spoofing
| Threat | Likelihood | Impact | Mitigation |
|--------|------------|--------|------------|
| Client impersonates another tenant | Medium | High | OIDC JWT required; tenant_id extracted from token, not request |
| Command replay as different user | Low | High | Idempotency keys + Journal sequence numbers |

### T — Tampering
| Threat | Likelihood | Impact | Mitigation |
|--------|------------|--------|------------|
| Command payload modified in transit | Low | High | TLS 1.3 at edge; mTLS between internal services |
| Tampering with command after receipt | Low | High | Journal append is immutable; checksums on envelope |

### R — Repudiation
| Threat | Likelihood | Impact | Mitigation |
|--------|------------|--------|------------|
| User denies sending command | Low | High | Audit log + Journal immutable record with principal subject |
| Operator modifies command history | Low | Critical | Journal is append-only; RBAC on admin endpoints |

### I — Information Disclosure
| Threat | Likelihood | Impact | Mitigation |
|--------|------------|--------|------------|
| Command payload leaks tenant secrets | Medium | High | Secrets redacted before journal entry (ADR-066) |
| Verbose error messages reveal internals | Low | Medium | RFC 7807Problem details without stack traces |

### D — Denial of Service
| Threat | Likelihood | Impact | Mitigation |
|--------|------------|--------|------------|
| Command flood exhausts Journal write bandwidth | Medium | High | Per-tenant rate limits via Quota adapter |
| Malformed commands cause handler crash | Low | Medium | Input validation + panics recovered |

### E — Elevation of Privilege
| Threat | Likelihood | Impact | Mitigation |
|--------|------------|--------|------------|
| Command bypasses authorization | Low | Critical | RBAC enforced at HTTP edge; domain validates |
| Privilege escalation via command injection | Very Low | Critical | Commands are typed; no dynamic evaluation |

## Mitigations Summary

| Control | Implementation |
|---------|----------------|
| AuthN | OIDC Bearer token required on all command endpoints |
| AuthZ | RBAC: operator role required for cross-tenant commands |
| Audit | ops.console.action.{completed,rejected} emitted for admin commands |
| Input validation | Schema validation on command payloads |
| Idempotency | Idempotency keys prevent duplicate execution |
| Secrets redaction | Redactor removes secrets before Journal entry |
