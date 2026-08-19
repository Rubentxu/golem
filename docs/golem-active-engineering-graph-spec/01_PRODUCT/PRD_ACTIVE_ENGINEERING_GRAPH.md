# PRD — Active Engineering Graph

> **Status:** Proposed reference specification  
> **Project:** GOLEM — Go Open Lifecycle & Engineering Manager  
> **Baseline reviewed:** `main` around commit `113c868…` (2026-08-19)  
> **Goal:** evolve GOLEM into an **Active Engineering Graph** / auditable engineering digital twin.  

## 1. Objective

Evolve GOLEM from a graph-backed lifecycle manager into a product where the graph is **reactive, temporal, explainable and actionable**.

## 2. Primary outcomes

1. A user can trace any relevant engineering entity to source evidence.
2. A change in the graph can trigger deterministic pattern-based behavior.
3. A user or agent can simulate a prospective change without mutating real state.
4. Capability Packs can extend backend semantics and frontend experiences.
5. Product navigation is organized around user intent rather than internal subsystems.

## 3. Key user stories

### Developer
- As a developer, I want to see the path from my commit to production.
- I want to know why my release is blocked and what action resolves it.

### Platform engineer
- I want to define a golden path that creates all required engineering entities and policies.
- I want to know which teams and services will be impacted by a platform change.

### Security engineer
- I want a new CVE to automatically identify exposed releases and deployments.
- I want VEX and evidence to alter risk state without losing original provenance.

### Architect
- I want to compare declared architecture to observed runtime dependencies.
- I want stale architectural decisions to be surfaced automatically.

### Engineering manager
- I want one risk/attention surface rather than checking many tool-specific dashboards.

## 4. Product pillars

### Know
Catalog, lineage, ownership, architecture, evidence.

### Explain
Why, Impact, Path, Evidence, Cause.

### React
Patterns, behaviors, policies, gates, notifications.

### Simulate
Scenarios, graph overlays, diff, risk preview.

### Change safely
Change Proposals, approvals, audited commands.

### Extend
Capability Packs, WASM/remote executors, UI contributions, ontology.

## 5. Success metrics

- >95% of release-gate states have machine-resolvable evidence paths.
- P95 `Why` response under agreed graph-query budget.
- Pattern false-positive and duplicate-action rate below defined SLO.
- New provider/domain integration does not require kernel switch modification.
- 80% of primary user workflows reachable in ≤3 navigation transitions.
- Full graph rebuild deterministically reproduces canonical digest for reference adapters.

## 6. Functional requirements

### FR-AEG-001 GraphDelta
Every committed graph mutation emits a normalized delta with journal position.

### FR-AEG-002 Pattern lifecycle
Patterns support MATCHED, UNMATCHED and CHANGED transitions.

### FR-AEG-003 Scenario overlays
A scenario can fork from a known journal position and store sparse overlay changes.

### FR-AEG-004 Provenance
Node/edge assertions identify source, evidence, assertion type and confidence where applicable.

### FR-AEG-005 Entity pages
Core entity kinds expose overview, relations, evidence, history and policy context.

### FR-AEG-006 Extension contributions
Capability Packs can contribute ontology, projectors, patterns, behaviors, views, actions and navigation.

### FR-AEG-007 Agent safety
Agentic actions result in proposals by default; mutation requires command path.

## 7. Non-functional requirements

- strict tenant isolation;
- bounded graph queries;
- deterministic replay for deterministic components;
- idempotent at-least-once consumers;
- horizontal scale by cell;
- observability for event lag, graph lag, pattern lag and proposal lifecycle;
- accessible alternative table/tree representations for graph views.
