# Vision — GOLEM as an Active Engineering Graph

> **Status:** Proposed reference specification  
> **Project:** GOLEM — Go Open Lifecycle & Engineering Manager  
> **Baseline reviewed:** `main` around commit `113c868…` (2026-08-19)  
> **Goal:** evolve GOLEM into an **Active Engineering Graph** / auditable engineering digital twin.  

## Problem

Engineering information is fragmented across planning systems, source control, CI/CD, test management, artifact registries, SBOM scanners, runtime platforms, observability systems, architecture documentation and policy engines. Existing platforms usually solve one of three subproblems:

- developer portal/catalog;
- ALM/DevOps workflow suite;
- operational/security point solutions.

The result is duplicated data, weak lineage, disconnected evidence and decisions based on partial context.

## Vision

GOLEM becomes the **semantic control plane** over those systems. Source systems remain authoritative for their own operational concerns; GOLEM builds a durable, temporal and explainable engineering model across them.

GOLEM should answer questions such as:

- Why is this release blocked?
- What will be impacted by upgrading this component?
- Which requirement caused this production change?
- Which evidence proves that this control passed?
- What architecture drift appeared after yesterday's deployments?
- What would happen if we promote this release into production?
- Which team can resolve this vulnerability?
- Which change most plausibly caused this incident?

## Product differentiation

### Backstage-inspired
Use entity-oriented UX, catalog semantics, extensibility and golden paths.

### Azure DevOps-inspired
Provide end-to-end workflow cohesion across planning, build, test, release, environments, approvals and governance.

### ActiveGraph-inspired
Treat the graph as a reactive model: graph deltas trigger declarative patterns, behaviors, scenarios and proposals.

## Non-goals

GOLEM is not intended to:

- replace Git, artifact registries, Kubernetes or observability backends;
- become a generic graph database;
- become a mandatory execution engine for every CI/CD pipeline;
- mutate external systems directly from unconstrained LLM output;
- require microservices for internal modularity.

## Strategic moat

The moat is the accumulated temporal graph of:

```text
intent → change → verification → artifact → release → runtime → evidence → outcome
```

plus the ability to **reason and react over the complete path**.
