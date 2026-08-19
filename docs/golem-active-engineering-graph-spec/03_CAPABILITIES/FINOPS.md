# FinOps as a Graph Lens

> **Status:** Proposed reference specification  
> **Project:** GOLEM — Go Open Lifecycle & Engineering Manager  
> **Baseline reviewed:** `main` around commit `113c868…` (2026-08-19)  
> **Goal:** evolve GOLEM into an **Active Engineering Graph** / auditable engineering digital twin.  

## Scope

GOLEM should enable cost reasoning without becoming a billing platform.

## Entities

- CloudResource
- Service
- Environment
- Team
- CostCenter
- CostObservation
- Deployment
- Release

## Relations

```text
Team OWNS Service
Service RUNS_ON CloudResource
CloudResource INCURS CostObservation
Deployment TARGETS Environment
```

## Questions

- What is the cost footprint of this service?
- Which resources are unowned?
- What did this release change in infrastructure cost?
- Which high-cost resources serve no production path?
- How does cost map to teams/domains?

## Architecture

Cost data is an analytics/time-series projection linked to graph identities; do not store massive billing time series as ordinary graph properties.
