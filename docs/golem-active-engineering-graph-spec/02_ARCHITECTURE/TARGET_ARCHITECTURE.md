# Target Architecture

> **Status:** Proposed reference specification  
> **Project:** GOLEM — Go Open Lifecycle & Engineering Manager  
> **Baseline reviewed:** `main` around commit `113c868…` (2026-08-19)  
> **Goal:** evolve GOLEM into an **Active Engineering Graph** / auditable engineering digital twin.  

## Architectural style

- modular monolith by default;
- vertical hexagons by bounded context;
- append-only authoritative journal;
- graph/search/analytics projections;
- graph-reactive runtime;
- cell-based SaaS scale.

## Logical architecture

```mermaid
flowchart TB
  EDGE[HTTP / CLI / UI / Agent Edge]
  APP[Context Application Ports]
  DOM[Bounded Context Domains]
  J[Graph Journal]
  PROJ[Projection Registry]
  G[Engineering Graph]
  GD[GraphDelta Stream]
  PAT[Pattern Engine]
  BEH[Behavior Runtime]
  SCN[Scenario Engine]
  PROP[Change Proposals]
  POL[Policy/Gate Engine]
  PACK[Capability Pack Runtime]
  SEARCH[Search Projection]
  ANA[Analytics Projection]

  EDGE --> APP --> DOM --> J
  J --> PROJ --> G
  PROJ --> GD
  GD --> PAT --> BEH
  G --> PAT
  G --> EDGE
  G --> SCN --> PROP --> POL --> APP
  J --> SEARCH
  J --> ANA
  PACK --> PROJ
  PACK --> PAT
  PACK --> EDGE
```

## Kernel responsibilities

Kernel remains intentionally small:

- event envelope;
- journal transaction boundary;
- tenant/actor/correlation model;
- command contract;
- projection registration;
- graph mutation contract;
- extension lifecycle;
- budgets/permissions.

Business domains do not belong in kernel switches.

## Evolution boundary

Separate processes are justified when:
- independent scaling is measured;
- failure isolation is needed;
- workload has distinct resource profile;
- ownership boundary is stable;
- latency/SLO requires it.

Do not preemptively split contexts into services.
