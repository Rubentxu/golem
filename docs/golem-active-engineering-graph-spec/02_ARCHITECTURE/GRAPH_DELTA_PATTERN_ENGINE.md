# GraphDelta and Declarative Pattern Engine

> **Status:** Proposed reference specification  
> **Project:** GOLEM — Go Open Lifecycle & Engineering Manager  
> **Baseline reviewed:** `main` around commit `113c868…` (2026-08-19)  
> **Goal:** evolve GOLEM into an **Active Engineering Graph** / auditable engineering digital twin.  

## GraphDelta

```go
type GraphDelta struct {
    TenantID        TenantID
    JournalPosition StreamPosition
    EventID         string

    AddedNodes      []NodeChange
    UpdatedNodes    []NodeChange
    RemovedNodes    []NodeChange
    AddedEdges      []EdgeChange
    UpdatedEdges    []EdgeChange
    RemovedEdges    []EdgeChange
}
```

Deltas are emitted only after graph mutation success.

## Why delta-driven matching

Without GraphDelta, every incoming event can force broad Lens execution. With GraphDelta:

1. determine changed kinds/edge types;
2. find patterns whose dependency signature intersects the delta;
3. evaluate only those patterns;
4. compute transition against prior match state.

## Pattern transitions

```text
FALSE -> TRUE   MATCHED
TRUE  -> FALSE  UNMATCHED
TRUE  -> TRUE   CHANGED (bound values materially changed)
```

## Pattern state

Persist:
- pattern ID/version;
- binding identity;
- prior fingerprint;
- last transition;
- journal position;
- debounce/cooldown state.

## DSL requirements

Support bounded constructs:
- typed nodes;
- directed edges;
- property predicates;
- `NOT EXISTS`;
- bounded path length;
- parameterized roots/scopes.

Disallow by default:
- unbounded variable length;
- cross-tenant patterns;
- arbitrary mutation;
- full scans without explicit privilege/budget.

## Compilation

```text
DSL
 → AST
 → semantic validation against Schema Registry
 → dependency signature
 → logical plan
 → adapter-specific or portable execution plan
```

## Loop safety

Behaviors carry:
- causation depth;
- origin behavior ID;
- dedupe key;
- max reaction depth;
- cooldown;
- optional cycle breaker.

## SLOs

Track:
- delta-to-match latency;
- candidate pattern count;
- query cost;
- transitions emitted;
- deduped transitions;
- pattern failures.
