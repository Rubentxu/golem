# SPIKE — Graph Database Selection

> **Status:** Proposed reference specification  
> **Project:** GOLEM — Go Open Lifecycle & Engineering Manager  
> **Baseline reviewed:** `main` around commit `113c868…` (2026-08-19)  
> **Goal:** evolve GOLEM into an **Active Engineering Graph** / auditable engineering digital twin.  

## Question

Which graph engine best fits GOLEM's SaaS workload while preserving adapter portability?

## Candidates

Evaluate a short list that includes:
- a strong distributed graph candidate;
- a fast single-node/embedded candidate;
- a mature property-graph candidate;
- current reference/in-memory adapter.

## Workloads

1. tenant-scoped point lookup;
2. 1-3 hop directional neighborhood;
3. high-degree component blast radius;
4. Path to Production;
5. vulnerability propagation;
6. temporal/as-of query;
7. graph diff support;
8. bulk rebuild ingestion;
9. concurrent tenants;
10. native pattern execution if available.

## Measure

- P50/P95/P99;
- throughput;
- memory;
- storage;
- rebuild speed;
- operational complexity;
- backup/restore;
- multi-tenancy options;
- Go client maturity;
- license;
- horizontal scaling;
- change-stream support.

## Exit

Produce weighted decision matrix and ADR recommendation. Do not select on synthetic BFS alone.
