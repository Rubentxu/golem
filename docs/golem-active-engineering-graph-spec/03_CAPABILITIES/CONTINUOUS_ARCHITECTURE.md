# Continuous Architecture and Architecture Drift

> **Status:** Proposed reference specification  
> **Project:** GOLEM — Go Open Lifecycle & Engineering Manager  
> **Baseline reviewed:** `main` around commit `113c868…` (2026-08-19)  
> **Goal:** evolve GOLEM into an **Active Engineering Graph** / auditable engineering digital twin.  

## Concept

Treat architecture as live graph semantics, not only diagrams.

## Declared architecture

Sources:
- C4 models;
- architecture-as-code;
- ADRs;
- repository metadata;
- explicit graph assertions.

## Observed architecture

Sources:
- OpenTelemetry traces;
- service mesh;
- Kubernetes;
- cloud inventory;
- runtime configuration;
- code analysis.

## Drift

Compare:

```text
Declared dependency graph
          VS
Observed dependency graph
```

Pattern examples:
- observed edge not declared;
- declared dependency never observed for N days;
- forbidden layer relation;
- service crosses domain boundary unexpectedly;
- runtime resource has no catalog owner.

## ADR graph

Model:
- Decision
- Alternative
- Constraint
- Hypothesis
- TradeOff
- Evidence
- Outcome.

Behavior:
`Constraint CHANGED` + ADR depends on constraint → `ADR_STALE` proposal.

## UX

Architecture Lens supports C4-like semantic zoom:
Domain → System → Container/Service → Component.
