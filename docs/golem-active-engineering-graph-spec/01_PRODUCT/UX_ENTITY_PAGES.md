# UX — Entity Pages

> **Status:** Proposed reference specification  
> **Project:** GOLEM — Go Open Lifecycle & Engineering Manager  
> **Baseline reviewed:** `main` around commit `113c868…` (2026-08-19)  
> **Goal:** evolve GOLEM into an **Active Engineering Graph** / auditable engineering digital twin.  

## Standard entity page

```text
┌─────────────────────────────────────────────────────────────────┐
│ payments-api    Service   Critical   Team Payments              │
│ [Why?] [Impact] [Simulate] [Propose Change] [•••]               │
├─────────────────────────────────────────────────────────────────┤
│ Overview | Delivery | Dependencies | Security | Runtime | ...   │
├───────────────────────────────────────────────┬─────────────────┤
│ Main content                                  │ Inspector       │
│                                               │ Overview        │
│                                               │ Relations       │
│                                               │ Evidence        │
│                                               │ History         │
│                                               │ Policies        │
└───────────────────────────────────────────────┴─────────────────┘
```

## Stable universal tabs

Every entity kind SHOULD support, when meaningful:

- Overview
- Relations
- Evidence
- History
- Policies

Domain-specific tabs are contributed by core modules or Capability Packs.

## Universal actions

### Why?
Explain current state with causal/evidence paths.

### Impact
Compute bounded impact graph using typed relation semantics.

### Simulate
Fork context into scenario overlay.

### Propose change
Create an auditable proposal attached to current context.

## Evidence UI

Every derived/important status includes:
- source;
- observed time;
- journal event;
- policy or behavior that derived it;
- assertion type;
- confidence if non-deterministic.

## Extension slots

Entity page slots:
- header badge;
- overview card;
- tab;
- side inspector section;
- action;
- graph Lens;
- attention rule.
