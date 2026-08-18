# STRIDE/Data-Flow: Provider Context

**Bounded Context**: Provider (internal/domain/provider, adapters/supplychain/)
**Wave**: W8.19
**Author**: GOLEM architecture team
**Date**: 2026-08-18
**ADR Reference**: ADR-005, ADR-045

---

## Context Description

The Provider context manages the registry and lifecycle of capability providers. Providers offer tools, skills, or resources that agents can invoke. Provider metadata includes SBOM, provenance attestations, and signature verification.

## Data Flow

```
[Provider Registry] ──register──► [Provider Catalog]
    │                                  │
    │                                  ▼
    │                           [Signature Verifier]
    │                                  │
    │                                  ▼
    │                           [SBOM Validator]
    │                                  │
    │                                  ▼
    └──────────────────► [Provider Activated]
```

## STRIDE Analysis

### S — Spoofing
| Threat | Likelihood | Impact | Mitigation |
|--------|------------|--------|------------|
| Malicious provider impersonates trusted provider | Medium | Critical | Provider signatures verified; PKI |
| Fake provider metadata | Medium | High | SBOM + provenance attestation required |

### T — Tampering
| Threat | Likelihood | Impact | Mitigation |
|--------|------------|--------|------------|
| Provider binary tampered after registration | Low | Critical | Binary signatures verified on every load |
| SBOM modified | Low | High | SBOM hash verified against registration |

### R — Repudiation
| Threat | Likelihood | Impact | Mitigation |
|--------|------------|--------|------------|
| Provider operator denies registration | Low | Medium | Registration recorded with identity |
| GOLEM claims provider was not registered | Low | Low | Registration immutable in Journal |

### I — Information Disclosure
| Threat | Likelihood | Impact | Mitigation |
|--------|------------|--------|------------|
| Provider exposes internal URLs in metadata | Low | Medium | Metadata validated before storage |
| SBOM reveals internal architecture | Low | Low | SBOM is external-facing artifact |

### D — Denial of Service
| Threat | Likelihood | Impact | Mitigation |
|--------|------------|--------|------------|
| Provider registration flood | Medium | Medium | Registration rate limits |
| Revocation check DoS | Low | Low | Revocation list cached; short TTL |

### E — Elevation of Privilege
| Threat | Likelihood | Impact | Mitigation |
|--------|------------|--------|------------|
| Provider gains elevated access | Very Low | Critical | Providers run in isolated execution context |
| Malicious provider escapes sandbox | Very Low | Critical | WASM isolation; network sandboxing |

## Mitigations Summary

| Control | Implementation |
|---------|----------------|
| Provider signatures | Ed25519/ECDSA signature verification |
| Provenance attestation | SBOM + provenance chain verified |
| Isolation | WASM sandbox for provider execution |
| Network sandbox | Providers cannot access internal network |
| Registration audit | All registrations journaled |
