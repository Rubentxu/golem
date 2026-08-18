# STRIDE/Data-Flow: Ingestion Context

**Bounded Context**: Ingestion (internal/application/ingest)
**Wave**: W8.17
**Author**: GOLEM architecture team
**Date**: 2026-08-18
**ADR Reference**: ADR-005, ADR-045

---

## Context Description

The Ingestion context receives external events and transforms them into typed Journal events. Ingestion provides the bridge between external systems and GOLEM's internal event model. Supports idempotent, ordered ingestion with validation.

## Data Flow

```
[External System] ──webhook/HTTP──► [Ingestion Endpoint]
    │                                     │
    │                                     ▼
    │                              [Validator]
    │                                     │
    │                                     ▼
    │                              [Transformer]
    │                                     │
    │                                     ▼
    └────────────────────────► [Journal Append]
```

## STRIDE Analysis

### S — Spoofing
| Threat | Likelihood | Impact | Mitigation |
|--------|------------|--------|------------|
| External system impersonation | Medium | High | Webhook signatures (HMAC); TLS |
| Ingestion endpoint accessed by rogue client | Medium | High | API key + IP allowlist |

### T — Tampering
| Threat | Likelihood | Impact | Mitigation |
|--------|------------|--------|------------|
| Event payload modified in transit | Low | High | TLS 1.3; HMAC webhook signatures |
| Malformed events inject bad state | Low | High | Schema validation; type checking |

### R — Repudiation
| Threat | Likelihood | Impact | Mitigation |
|--------|------------|--------|------------|
| External system denies sending event | Low | Medium | Webhook signature provides proof of origin |
| GOLEM claims event not received | Low | Medium | Receipt acknowledgment; Journal sequence |

### I — Information Disclosure
| Threat | Likelihood | Impact | Mitigation |
|--------|------------|--------|------------|
| External events contain sensitive data | Medium | High | Redactor in ingestion pipeline |
| Verbose validation errors leak schema | Low | Medium | Error responses sanitized |

### D — Denial of Service
| Threat | Likelihood | Impact | Mitigation |
|--------|------------|--------|------------|
| Event flood from compromised integration | High | High | Per-source rate limits; queue backpressure |
| Large payloads exhaust ingestion handler | Medium | Medium | Payload size limits; streaming validation |

### E — Elevation of Privilege
| Threat | Likelihood | Impact | Mitigation |
|--------|------------|--------|------------|
| Ingestion bypasses tenant context | Very Low | Critical | Tenant_id extracted from API key, not event |
| External event elevates privileges | Very Low | Critical | Privilege levels not settable via ingestion |

## Mitigations Summary

| Control | Implementation |
|---------|----------------|
| Webhook auth | HMAC-SHA256 signature verification |
| TLS | All ingestion endpoints require TLS 1.3 |
| Rate limits | Per-source rate limiting via quota adapter |
| Payload limits | Max payload size enforced |
| Secrets redaction | Redactor in transformation pipeline |
| Input validation | Schema validation on all incoming events |
