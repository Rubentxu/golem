# UX — Graph and Moldable Visualization

> **Status:** Proposed reference specification  
> **Project:** GOLEM — Go Open Lifecycle & Engineering Manager  
> **Baseline reviewed:** `main` around commit `113c868…` (2026-08-19)  
> **Goal:** evolve GOLEM into an **Active Engineering Graph** / auditable engineering digital twin.  

## Principle

The graph engine powers the UI; users interact with **Lenses**, not an unbounded hairball.

## First-class Lenses

- Architecture
- Dependencies
- Ownership
- Delivery Lineage
- Supply Chain
- Runtime Topology
- Security Exposure
- Evidence
- Causality
- Cost
- Change Impact

## Semantic zoom

### Level 0
Domain/system clusters.

### Level 1
Services/resources.

### Level 2
Components/artifacts/builds.

### Level 3
Evidence/events.

## Performance rules

- never fetch the complete tenant graph;
- root every graph exploration;
- default bounded depth;
- progressive expansion;
- paged/high-degree neighborhoods;
- incremental layout;
- WebGL/WebGPU renderer where justified;
- server-side graph query budgets;
- client-side virtualization.

## Alternate representations

Every graph Lens needs:
- tree or outline;
- table;
- path list;
- accessible keyboard traversal.

## Path to Production

A flagship lineage view:

```text
Requirement
 → Work Item
 → Commit
 → Review
 → Build
 → Test
 → Artifact
 → SBOM/Attestation
 → Release
 → Approval
 → Deployment
```

Each hop is backed by a graph edge and provenance.
