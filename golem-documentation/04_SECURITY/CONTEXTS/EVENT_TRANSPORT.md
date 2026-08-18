# STRIDE/Data-Flow: Event Transport Context

**Bounded Context**: Event Transport (internal/ports/transport, adapters/transport/)
**Wave**: W8.15
**Author**: GOLEM architecture team
**Date**: 2026-08-18
**ADR Reference**: ADR-005, ADR-045

---

## Context Description

The Event Transport context moves events between bounded contexts and external systems. It provides pub/sub semantics over NATS JetStream. Events are encoded as Envelopes with typed Payloads and travel asynchronously.

## Data Flow

```
[Journal] ──publish──► [NATS JetStream] ──subscribe──► [Subscriber Bounded Context]
    │                                                                       │
    └────────────────────────────────────────────────────────────────────────┘
                              (same event, different consumers)
```

## STRIDE Analysis

### S — Spoofing
| Threat | Likelihood | Impact | Mitigation |
|--------|------------|--------|------------|
| Fake event published to stream | Low | High | Publisher authentication via NATS credentials |
| Subscriber impersonation | Low | Medium | NATS subject permissions per service account |

### T — Tampering
| Threat | Likelihood | Impact | Mitigation |
|--------|------------|--------|------------|
| Event tampered in transit | Very Low | High | TLS on NATS connections; checksums in envelope |
| Replay of old events | Low | Medium | Consumer cursors prevent replay beyond offset |

### R — Repudiation
| Threat | Likelihood | Impact | Mitigation |
|--------|------------|--------|------------|
| Publisher denies sending | Low | Medium | NATS audit log; Journal is source of truth |
| Subscriber claims not received | Low | Medium | NATS acknowledgements; consumer position tracked |

### I — Information Disclosure
| Threat | Likelihood | Impact | Mitigation |
|--------|------------|--------|------------|
| Event content intercepted | Low | High | TLS on all NATS connections |
| Sensitive headers exposed | Medium | High | Redactor removes secrets before transport |

### D — Denial of Service
| Threat | Likelihood | Impact | Mitigation |
|--------|------------|--------|------------|
| Event flood saturates NATS bandwidth | Medium | High | Per-stream rate limits; backpressure |
| Consumer group overwhelmed | Medium | Medium | Consumer push limits; batch processing |

### E — Elevation of Privilege
| Threat | Likelihood | Impact | Mitigation |
|--------|------------|--------|------------|
| Subscriber consumes unauthorized streams | Low | High | NATS subject ACLs enforced |
| Malicious subscription to internal streams | Low | High | Internal streams not exported externally |

## Mitigations Summary

| Control | Implementation |
|---------|----------------|
| TLS | All NATS connections encrypted |
| Authentication | NATS service accounts with scoped credentials |
| Authorization | Subject ACLs per consumer group |
| Backpressure | Consumer push limits; batch acknowledgements |
| Secrets redaction | Events redacted before transport |
