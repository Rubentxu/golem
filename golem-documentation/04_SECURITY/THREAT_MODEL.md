# Threat Model

| Amenaza | Mitigación |
|---|---|
| Cross-tenant access | tenant context, partition, authorization, fuzz/TCK |
| Event tampering | immutable Journal, checksums/signatures where required |
| Forged artifact origin | provenance + signatures |
| Malicious pack | signature, WASM/remote isolation |
| Compromised provider | narrow permissions + evidence verification |
| Agent prompt injection | Lens minimization, tool policy, proposal-only |
| Replay/double execution | idempotency/inbox |
| Broker loss | broker != source of truth |
| Graph query DoS | budgets, deadlines, quotas |
| Dependency compromise | SBOM, scan, pinned builds, provenance |
| Insider change | approvals, audit, separation of duties |
| Secret leakage | schema/redaction checks |

Cada bounded context completa STRIDE/data-flow review antes de GA.
