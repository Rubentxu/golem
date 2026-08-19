# Active Graph Runtime

> **Status:** Proposed reference specification  
> **Project:** GOLEM — Go Open Lifecycle & Engineering Manager  
> **Baseline reviewed:** `main` around commit `113c868…` (2026-08-19)  
> **Goal:** evolve GOLEM into an **Active Engineering Graph** / auditable engineering digital twin.  

## Purpose

Turn the graph from a passive projection into a **reactive knowledge substrate**.

## Pipeline

```text
Accepted Event
    ↓
Projection Registry
    ↓
GraphMutation(s)
    ↓
Graph Store
    ↓
GraphDelta
    ↓
Delta Index
    ↓
Affected Pattern Set
    ↓
Pattern Evaluation
    ↓
Transition
    ↓
Behavior
    ↓
Event / Change Proposal / Notification
```

## Behavior categories

### Deterministic
Pure deterministic computation.

### Relation
Reacts to semantic graph relations.

### Workflow
Coordinates multi-step durable process.

### Agentic
Uses bounded Lens + tools + model and returns evidence/proposals.

## Rules

- external I/O occurs through tools/ports;
- all executions have budgets;
- deterministic behaviors must be replay-safe;
- agentic behavior is not replayed as if deterministic unless recorded output is reused;
- behavior-produced commands carry causation chains;
- loops are bounded and detectable.

## Active Graph examples

### Vulnerability propagation
New CVE → Component affected → Artifact affected → Release blocked → deployed services highlighted.

### Architecture drift
Observed runtime edge appears → no declared architecture relation → Drift pattern MATCHED.

### Stale ADR
Constraint changed → ADR depends on old constraint → stale-decision proposal.

### Release readiness
Evidence changes → release gate pattern CHANGED → readiness recomputed.
