# STRIDE/Data-Flow: Journal Context

**Bounded Context**: Journal (internal/domain/journal, adapters/journal/)
**Wave**: W8.13
**Author**: GOLEM architecture team
**Date**: 2026-08-18
**ADR Reference**: ADR-005, ADR-031, ADR-045

---

## Context Description

The Journal is the immutable event store and source of truth for GOLEM. All state changes are captured as typed events in the Journal. The Journal provides append-only semantics with sequence numbers, causal ordering, and tenant isolation.

## Data Flow

```
[Command Handler]
    │
    ▼ Journal Append
[Journal Adapter] ──► [Storage: bbolt / Postgres-RDS]
    │
    ▼
[Event Envelope] ──► [Sequenced, Checksummed, Tenant-scoped]
```

## STRIDE Analysis

### S — Spoofing
| Threat | Likelihood | Impact | Mitigation |
|--------|------------|--------|------------|
| Fake event injection | Very Low | Critical | Append requires valid command envelope with principal |
| Tenant forges events for another tenant | Very Low | Critical | Tenant ID mandatory in envelope; storage partition |

### T — Tampering
| Threat | Likelihood | Impact | Mitigation |
|--------|------------|--------|------------|
| Event tampering after append | Very Low | Critical | Append-only; checksum per event; immutable storage |
| Replay attack (old events replayed) | Low | Medium | Sequence numbers detect gaps; idempotency keys |

### R — Repudiation
| Threat | Likelihood | Impact | Mitigation |
|--------|------------|--------|------------|
| User denies action recorded in Journal | Very Low | Critical | Immutable audit trail; principal in envelope |
| Operator modifies event to cover tracks | Very Low | Critical | Journal append is append-only; no UPDATE/DELETE API |

### I — Information Disclosure
| Threat | Likelihood | Impact | Mitigation |
|--------|------------|--------|------------|
| Event content leaks sensitive data | Medium | High | Redactor removes secrets before append |
| Cross-tenant event visibility | Very Low | Critical | Tenant partition enforced at storage layer |

### D — Denial of Service
| Threat | Likelihood | Impact | Mitigation |
|--------|------------|--------|------------|
| Journal disk space exhaustion | Medium | High | Per-tenant quotas; retention policies |
| Append flood causes write starvation | Medium | High | Per-tenant write throttling |

### E — Elevation of Privilege
| Threat | Likelihood | Impact | Mitigation |
|--------|------------|--------|------------|
| Event with elevated privileges forged | Very Low | Critical | All events require valid AuthN; RBAC on event types |
| Privilege escalation via event content | Very Low | Critical | Event schemas are typed; no dynamic evaluation |

## Mitigations Summary

| Control | Implementation |
|---------|----------------|
| Immutability | Append-only API; no update/delete |
| Checksums | SHA-256 on each envelope |
| Tenant isolation | Tenant ID mandatory; storage partition |
| Secrets redaction | Redactor removes secrets before journal entry |
| Quotas | Per-tenant write limits |
