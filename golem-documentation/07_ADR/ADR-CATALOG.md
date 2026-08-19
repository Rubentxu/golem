# ADR Catalog

| ADR | Estado | Decisión |
|---|---|---|
| [ADR-001](ADR-001.md) | Accepted | Use Go for kernel and backend services |
| [ADR-002](ADR-002.md) | Accepted | Hexagonal architecture is mandatory |
| [ADR-003](ADR-003.md) | Accepted | Bounded contexts and SOLID over shared mega-models |
| [ADR-004](ADR-004.md) | Accepted | Engineering Graph is the canonical domain model |
| [ADR-005](ADR-005.md) | Accepted | Graph Journal is the authoritative causal history |
| [ADR-006](ADR-006.md) | Accepted | Use CQRS-style command/query separation |
| [ADR-007](ADR-007.md) | Accepted | Use cell-based SaaS architecture |
| [ADR-008](ADR-008.md) | Accepted | Tenant context is mandatory end-to-end |
| [ADR-009](ADR-009.md) | Proposed | Multi-region writes are spike-gated |
| [ADR-010](ADR-010.md) | Accepted | Public API is OpenAPI-first |
| [ADR-011](ADR-011.md) | Accepted | Typed internal RPC without domain leakage |
| [ADR-012](ADR-012.md) | Accepted | NATS JetStream is reference event transport, not core dependency |
| [ADR-013](ADR-013.md) | Accepted | Graph database choice requires benchmark gate (resuelto vía ADR-087: Dgraph elegida) |
| [ADR-014](ADR-014.md) | Accepted | Large blobs use S3-compatible ObjectStore |
| [ADR-015](ADR-015.md) | Accepted | Search is a rebuildable derived projection |
| [ADR-016](ADR-016.md) | Accepted | Analytics is a rebuildable derived projection |
| [ADR-017](ADR-017.md) | Accepted | OIDC is the identity boundary |
| [ADR-018](ADR-018.md) | Accepted | Policy engine behind PolicyEvaluator port (supersedes ADR-063 when available) |
| [ADR-019](ADR-019.md) | Accepted | OpenTelemetry is the observability contract |
| [ADR-020](ADR-020.md) | Accepted | Use transactional outbox and idempotent inbox |
| [ADR-021](ADR-021.md) | Accepted | Optimistic concurrency is the default mutation model |
| [ADR-022](ADR-022.md) | Accepted | Artifacts are content-addressed |
| [ADR-023](ADR-023.md) | Superseded by [ADR-053](ADR-053.md) | Support SPDX and CycloneDX SBOM |
| [ADR-024](ADR-024.md) | Superseded by [ADR-054](ADR-054.md) | Use SLSA/in-toto compatible provenance |
| [ADR-025](ADR-025.md) | Superseded by [ADR-054](ADR-054.md) | Sigstore is reference signing adapter |
| [ADR-026](ADR-026.md) | Superseded by [ADR-055](ADR-055.md) | VEX is first-class security data |
| [ADR-027](ADR-027.md) | Accepted | Web UI is an adapter, not the domain |
| [ADR-028](ADR-028.md) | Accepted | Graph visualization is neighborhood-based |
| [ADR-029](ADR-029.md) | Accepted | Third-party extension code uses WASM or remote plugins |
| [ADR-030](ADR-030.md) | Accepted | Graph Journal combines state semantics and causal history |
| [ADR-031](ADR-031.md) | Accepted | Events are immutable first-class entities |
| [ADR-032](ADR-032.md) | Accepted | Events are accepted before external integration delivery |
| [ADR-033](ADR-033.md) | Accepted | Event broker is transport, never source of truth |
| [ADR-034](ADR-034.md) | Accepted | Reactive Behaviors are a kernel primitive |
| [ADR-035](ADR-035.md) | Accepted | Relations may own behavior |
| [ADR-036](ADR-036.md) | Accepted | Graph Pattern Subscriptions are bounded and compiled |
| [ADR-037](ADR-037.md) | Superseded by ADR-059 | Graph Lenses bound execution context |
| [ADR-038](ADR-038.md) | Accepted | Change Proposal is the privileged write primitive |
| [ADR-039](ADR-039.md) | Accepted | Agents default to proposal-only writes |
| [ADR-040](ADR-040.md) | Accepted | Execution Frames bound goals, permissions and budgets (supersedes ADR-064 when available) |
| [ADR-041](ADR-041.md) | Superseded by ADR-060 | Scenario fork/diff/promote is first-class |
| [ADR-042](ADR-042.md) | Accepted | Capability Packs are the domain extension unit |
| [ADR-043](ADR-043.md) | Accepted | Capability Packs are distributed as OCI artifacts |
| [ADR-044](ADR-044.md) | Accepted | Third-party code is isolated from hot kernel |
| [ADR-045](ADR-045.md) | Accepted | Every external dependency sits behind a port |
| [ADR-046](ADR-046.md) | Accepted | Every critical port owns a conformance TCK |
| [ADR-047](ADR-047.md) | Accepted | Vendor data types never cross adapter boundaries |
| [ADR-048](ADR-048.md) | Accepted | Provider capabilities are negotiated explicitly |
| [ADR-049](ADR-049.md) | Accepted | Derived stores must be rebuildable |
| [ADR-050](ADR-050.md) | Accepted | Canonical provider-neutral export is mandatory |
| [ADR-051](ADR-051.md) | Accepted | Provider migration is a supported operational workflow |
| [ADR-052](ADR-052.md) | Accepted | Replaceability Level is an architecture fitness metric |
| [ADR-053](ADR-053.md) | Accepted | SBOM ingestion through the SBOMParser port |
| [ADR-054](ADR-054.md) | Accepted | Provenance and signing behind dedicated ports |
| [ADR-055](ADR-055.md) | Accepted | Vulnerability and VEX as first-class graph data |
| [ADR-056](ADR-056.md) | Accepted | Typed graph traversal with explicit truncation |
| [ADR-057](ADR-057.md) | Accepted | Provider Profiles for adapter composition |
| [ADR-058](ADR-058.md) | Accepted | Capability Packs v1: declarative activation over the journal |
| [ADR-059](ADR-059.md) | Accepted | Behavior Engine v1: deterministic native behaviors |
| [ADR-060](ADR-060.md) | Accepted | Scenarios: fork/diff/promote over overlay deltas |
| [ADR-061](ADR-061.md) | Accepted | LLM Provider Port and Capabilities |
| [ADR-062](ADR-062.md) | Accepted | Tool Port and Capability Catalog v2 |
| [ADR-065](ADR-065.md) | Accepted | Redaction Pipeline for PII Safety |
| [ADR-066](ADR-066.md) | Accepted | LLM Redactor in Journal Pipeline |
| [ADR-067](ADR-067.md) | Accepted | Tracing Correlation Context |
| [ADR-068](ADR-068.md) | Accepted | OTel Span Per Call with Correlation |
| [ADR-070](ADR-070.md) | Accepted | Agent Eval Harness for Behavior Scoring |
| [ADR-071](ADR-071.md) | Accepted | Held-Out Fixtures for Agent Eval |
| [ADR-072](ADR-072.md) | Accepted | Agentic Behavior Kind v1 |
| [ADR-073](ADR-073.md) | Accepted | Metering Hook on Command Bus |
| [ADR-074](ADR-074.md) | Accepted | Multi-cell data partitioning (D1) |
| [ADR-075](ADR-075.md) | Accepted | Tenant migration (D2) |
| [ADR-076](ADR-076.md) | Accepted | Quotas per tenant (D3) |
| [ADR-077](ADR-077.md) | Accepted | Metering pipeline to S3 (D4) |
| [ADR-078](ADR-078.md) | Accepted | Audit export with KMS signature (D5) |
| [ADR-079](ADR-079.md) | Accepted | DR automation with restore drill (D6) |
| [ADR-080](ADR-080.md) | Accepted | SLO framework with error budget (D7) |
| [ADR-081](ADR-081.md) | Accepted | Ops console operator-only (D8) |
| [ADR-082](ADR-082.md) | Accepted | OIDC bearer + JWKS discovery (D9) |
| [ADR-083](ADR-083.md) | Accepted | Profile composition + AWS managed (D10) |
| [ADR-084](ADR-084.md) | Accepted | Eliminar journal postgres adapter (no hay ADR que lo justifique) |
| [ADR-085](ADR-085.md) | Accepted | NATS JetStream required en profiles non-dev; prod corrige transport a natsjs |
| [ADR-086](ADR-086.md) | Accepted | Benchmark gate para ADR-013: quadlets estables para candidates de graph DB |
| [ADR-087](ADR-087.md) | Accepted | Graph database elegida: Dgraph (resultado benchmark gate: W1 3957/s vs HugeGraph 363/s; HugeGraph descartado por 99% error rate en edge kind enforcement; NebulaGraph diferido con triggers de re-evaluación) |
