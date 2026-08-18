# STRIDE/Data-Flow: DR Context

**Bounded Context**: DR / Disaster Recovery (internal/application/migration, adapters/journal/)
**Wave**: W8.24
**Author**: GOLEM architecture team
**Date**: 2026-08-18
**ADR Reference**: ADR-005, ADR-045

---

## Context Description

The DR context handles backup, restore, and migration operations. It provides point-in-time recovery, cross-cell migration, and disaster recovery drills. DR operations are orchestrated by the control plane with tenant safety as the primary concern.

## Data Flow

```
[DR Orchestrator] ──backup──► [Journal Snapshot]
    │                               │
    │                               ▼
    │                        [S3 Backup Store]
    │                               │
    │       ┌───────────────────────┴───────────────────────┐
    │       ▼                                               ▼
    │  [Cross-Cell Migration]                          [Point-in-Time Restore]
    │       │                                               │
    │       └───────────────────────┬───────────────────────┘
    │                               │
    └──────────────────────► [Target Cell Journal]
```

## STRIDE Analysis

### S — Spoofing
| Threat | Likelihood | Impact | Mitigation |
|--------|------------|--------|------------|
| Fake backup restoration | Very Low | Critical | Restore requires operator role; multi-party confirmation |
| DR drill impersonation | Very Low | Medium | DR drills use isolated environment |

### T — Tampering
| Threat | Likelihood | Impact | Mitigation |
|--------|------------|--------|------------|
| Backup tampered | Very Low | Critical | Backups encrypted at rest; integrity hash |
| Restore point manipulated | Very Low | Critical | Restore targets versioned snapshots only |

### R — Repudiation
| Threat | Likelihood | Impact | Mitigation |
|--------|------------|--------|------------|
| Tenant denies data was restored | Low | High | Restore operations logged with timestamps |
| Operator denies performing restore | Very Low | Critical | All DR operations journaled; immutable audit |

### I — Information Disclosure
| Threat | Likelihood | Impact | Mitigation |
|--------|------------|--------|------------|
| Backup exposes tenant data | Medium | Critical | Backups encrypted; key management via KMS |
| DR drill exposes production data | Low | High | DR drills use anonymized data |

### D — Denial of Service
| Threat | Likelihood | Impact | Mitigation |
|--------|------------|--------|------------|
| Backup exhaustion of storage | Medium | High | Backup retention policies; tiered storage |
| Restore operation blocks production | Low | Medium | Restore operations scheduled; throttling |

### E — Elevation of Privilege
| Threat | Likelihood | Impact | Mitigation |
|--------|------------|--------|------------|
| Unauthorized cross-tenant migration | Very Low | Critical | Cross-tenant migration requires super-operator |
| Restore to wrong cell | Very Low | Critical | Restore target explicitly specified; validated |

## Mitigations Summary

| Control | Implementation |
|---------|----------------|
| Encryption | Backups encrypted with KMS-managed keys |
| Access control | Restore requires operator role + multi-party |
| Audit | All DR operations journaled with principal |
| Retention | Backup retention policy enforced |
| DR drills | Regular drills validate recoverability |
