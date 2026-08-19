# Performance and Scale Plan

> **Status:** Proposed reference specification  
> **Project:** GOLEM — Go Open Lifecycle & Engineering Manager  
> **Baseline reviewed:** `main` around commit `113c868…` (2026-08-19)  
> **Goal:** evolve GOLEM into an **Active Engineering Graph** / auditable engineering digital twin.  

## Primary budgets

Define explicit targets before optimization.

### Journal
- append throughput;
- P50/P95/P99 latency.

### Graph
- point lookup;
- 1-hop neighborhood;
- bounded impact path;
- typical Path to Production query.

### Active Graph
- delta-to-transition P95;
- candidate pattern count;
- max matching cost.

### UX
- initial entity page;
- progressive graph expansion;
- 60fps interactions for typical visible subgraph.

## Scale dimensions

Benchmark independently:
- tenants;
- events/sec;
- graph nodes/edges per tenant;
- high-degree nodes;
- patterns/tenant;
- behaviors/sec;
- simultaneous scenarios;
- agent executions.

## High-degree control

For nodes such as shared libraries:
- paginate adjacency;
- cap expansions;
- aggregate clusters;
- server-side filtering;
- precomputed impact summaries where valuable.

## Graph database selection

Do not select from headline throughput alone. Benchmark GOLEM-specific workloads from `06_SPIKES/SPIKE_GRAPH_DATABASE.md`.
