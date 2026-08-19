# GOLEM — Active Engineering Graph Specification Pack

> **Status:** Proposed reference specification  
> **Project:** GOLEM — Go Open Lifecycle & Engineering Manager  
> **Baseline reviewed:** `main` around commit `113c868…` (2026-08-19)  
> **Goal:** evolve GOLEM into an **Active Engineering Graph** / auditable engineering digital twin.  


This pack translates the architectural/product review of GOLEM into an implementation-oriented set of Markdown specifications, ADRs, spikes and delivery plans.

## North Star

GOLEM should not become a clone of Backstage or Azure DevOps. It should become:

> **An Active Engineering Graph: an auditable digital twin of the software lifecycle that observes engineering systems, explains relationships, reacts to graph changes, simulates consequences and lets humans and agents propose safe changes.**

## Core equation

```text
Graph Journal
  + Temporal Engineering Graph
  + GraphDelta
  + Declarative Patterns
  + Behaviors
  + Scenarios
  + Change Proposals
  + Moldable UI
  = Active Engineering Graph
```

## Recommended reading order

1. `00_EXECUTIVE/VISION.md`
2. `01_PRODUCT/PRD_ACTIVE_ENGINEERING_GRAPH.md`
3. `02_ARCHITECTURE/TARGET_ARCHITECTURE.md`
4. `02_ARCHITECTURE/ACTIVE_GRAPH_RUNTIME.md`
5. `02_ARCHITECTURE/GRAPH_DELTA_PATTERN_ENGINE.md`
6. `02_ARCHITECTURE/EXTENSION_PLATFORM.md`
7. `01_PRODUCT/UX_INFORMATION_ARCHITECTURE.md`
8. `04_ADRS/`
9. `05_DELIVERY/ROADMAP.md`
10. `05_DELIVERY/IMPLEMENTATION_PLAN.md`

## Package map

| Folder | Purpose |
|---|---|
| `00_EXECUTIVE` | Product thesis, principles and operating model |
| `01_PRODUCT` | PRD, capability model, personas and UX |
| `02_ARCHITECTURE` | Target architecture and platform subsystems |
| `03_CAPABILITIES` | Concrete product capability families |
| `04_ADRS` | Proposed architecture decisions |
| `05_DELIVERY` | Roadmap, milestones, backlog, testing, migration |
| `06_SPIKES` | Technical investigations needed before locking choices |
| `07_REFERENCE` | Glossary, traceability, checklists |

## Important implementation stance

The plan deliberately preserves the current **modular-monolith + ports/adapters + journal + graph projection** strengths. Service extraction is evidence-driven, not a default. The main architectural transition is from a central event-driven kernel toward **vertical hexagons plus a graph-reactive platform**.

## Immediate correctness fixes before feature expansion

1. Projection checkpoints must advance across successfully consumed no-op/unknown events.
2. Supply-chain projection must apply **all** chunks for large SBOM events.
3. Command idempotency receipts must become atomic with journal append, or be enforced by the same transactional invariant boundary.
4. Architecture fitness tests should validate layer and bounded-context dependency direction, not only vendor isolation.

