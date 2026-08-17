# GOLEM — Go Open Lifecycle & Engineering Manager

> **Graph-native, event-driven, auditable software engineering control plane.**

**README language:** English | [Español](README.es.md)

[![Go Reference](https://pkg.go.dev/badge/github.com/Rubentxu/golem.svg)](https://pkg.go.dev/github.com/Rubentxu/golem)
[![Go Report Card](https://goreportcard.com/badge/github.com/Rubentxu/golem)](https://goreportcard.com/report/github.com/Rubentxu/golem)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

GOLEM is a SaaS platform for managing the software engineering lifecycle end
to end: intent, requirements, planning, code, review, builds, tests,
artifacts, dependencies, SBOM, provenance, vulnerabilities, releases,
deployments, evidence, approvals, architecture and operations.

It is **not** a literal Tuleap clone and **not** another issue tracker. GOLEM
changes the architectural substrate:

- **graph-native** — entities and relations form an Engineering Graph;
- **event-sourced** — a Graph Journal preserves the causal history;
- **reactive** — Behaviors react to events and graph patterns;
- **supply-chain-native** — artifacts, SBOM, attestations, signatures, VEX
  and provenance are first-class;
- **agent-ready** — agents operate through Graph Lenses and Change Proposals;
- **provider-independent** — every external dependency sits behind a port
  with a conformance TCK;
- **SaaS-first** — cells, tenant isolation, quotas and observability.

```text
The Graph is the Model.      The Journal is the History.
The Behavior is the Reaction. The Evidence is the Proof.
The Port is the Boundary.
```

## Status

**Phase 0 — bootstrap** (M0 of the roadmap). This repository currently
contains the module skeleton, the hexagonal ports and the architecture
fitness tests. The full specification lives in
[`golem-documentation/`](golem-documentation/11_RESOURCES/START_HERE.md) —
start there.

## Non-negotiable principles

- Domain code never imports vendor SDKs (enforced by `internal/archtest`).
- The event broker is transport, never the source of truth.
- Derived stores (graph projection, search, analytics) are rebuildable from
  the Journal.
- Every critical port owns a conformance TCK.
- Tenant context is mandatory end to end.
- Supply-chain security (SBOM, provenance, signatures) is core, not an
  add-on.

## Getting started

### Prerequisites

- Go 1.26+
- [just](https://github.com/casey/just) — command runner for local tasks

### Build & test

```sh
git clone https://github.com/Rubentxu/golem.git
cd golem
just check   # fmt-check + vet + test — the local gate is the source of truth
just build   # build cmd/*
```

### Run

```sh
just build
./golem-api          # API edge skeleton, serves GET /healthz on :8080
```

## Project layout

```text
cmd/{golem-api,golem-worker,golemctl}   composition root (binaries)
internal/domain                         kernel domain model
internal/application                    commands/queries (CQRS)
internal/ports                          hexagonal port contracts
internal/archtest                       architecture fitness functions
adapters/                               provider implementations behind ports
tck/                                    conformance kits per port
api/                                    API contracts (source: golem-documentation/06_API_SPECS)
schemas/                                event schemas (source: golem-documentation/06_API_SPECS)
docs/adr/                               ADRs for code-level decisions
golem-documentation/                    full product & architecture specification
```

Avoid global `common`, `utils` and `models` packages.

## Development workflow

- **Local gate:** `just check` must pass before any merge. GitHub Actions
  is reserved for the release gate (tag-driven).
- **Architectural changes:** ADR → fitness test → docs → code
  (see [`golem-documentation/10_GOVERNANCE/CONTRIBUTING.md`](golem-documentation/10_GOVERNANCE/CONTRIBUTING.md)).
- **Commits:** Conventional Commits.
- **Adding a provider:** implement the port in `adapters/`, map vendor types
  to canonical ones at the boundary, and pass the port's TCK.

## Roadmap

| Milestone | Focus | Exit criterion |
|---|---|---|
| M0 | Discovery & architecture fitness | CI blocks vendor imports; benchmarks planned |
| M1 | Kernel (Journal, projection, replay) | replay rebuilds graph; reproducible digest |
| M2 | Work / Requirements / Planning MVP | usable flow; Requirement→Work trace |
| M3 | Test / SCM / CI / Artifact lineage | Requirement→…→Artifact→Test trace |
| M4 | Supply-chain security | evidence-based production gate |
| M5 | Extensibility & provider independence | second implementation for critical ports |
| M6 | Reactive graph & scenarios | reproducible what-if |
| M7 | Agentic layer | proposal-only agent writes |
| M8 | SaaS scale & GA | cells, DR, SLOs, security review |

Full roadmap: [`golem-documentation/08_DELIVERY/ROADMAP.md`](golem-documentation/08_DELIVERY/ROADMAP.md).

## Documentation

- [Start here](golem-documentation/11_RESOURCES/START_HERE.md)
- [Vision](golem-documentation/00_OVERVIEW/VISION.md)
- [Architecture](golem-documentation/02_ARCHITECTURE/ARCHITECTURE.md)
- [ADR catalog (ADR-001..052)](golem-documentation/07_ADR/ADR-CATALOG.md)
- [API specs](golem-documentation/06_API_SPECS/API_GUIDELINES.md)

## Contributing

Contributions are welcome. Read
[`golem-documentation/10_GOVERNANCE/CONTRIBUTING.md`](golem-documentation/10_GOVERNANCE/CONTRIBUTING.md)
first — domain changes require context-owner review, adapter changes
require port-owner review, and every architectural change starts with an
ADR.

## License

Released under the [MIT License](LICENSE).
