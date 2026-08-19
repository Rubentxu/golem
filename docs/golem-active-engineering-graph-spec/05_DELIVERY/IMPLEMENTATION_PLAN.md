# Implementation Plan

> **Status:** Proposed reference specification  
> **Project:** GOLEM — Go Open Lifecycle & Engineering Manager  
> **Baseline reviewed:** `main` around commit `113c868…` (2026-08-19)  
> **Goal:** evolve GOLEM into an **Active Engineering Graph** / auditable engineering digital twin.  

## Workstream A — Kernel correctness

### A1 ProjectionRunner
Create one safe apply contract:
- project all chunks;
- apply;
- record result;
- advance checkpoint on success/no-op.

### A2 Command transaction
Extend JournalStore with transactional command append capability or define explicit optional interface:
`CommandJournal`.

### A3 Tests
Inject crash points and replay.

## Workstream B — Modularization

### B1 Projection Registry
Move event→projector mapping out of central switch.

### B2 API registration
Each bounded context exposes:
```go
type HTTPModule interface {
    Register(*http.ServeMux, Dependencies)
}
```
or equivalent without leaking concrete runtime.

### B3 Context ports
Replace broad GraphStore injection in use cases with narrow interfaces.

## Workstream C — Graph schema/provenance

1. Define schema metadata model.
2. Register current core ontology.
3. Validate mutations in reference adapter/TCK.
4. Add assertion/provenance envelope.
5. Migrate existing projection helpers.

## Workstream D — GraphDelta

1. Define deterministic delta derivation.
2. Attach journal position.
3. Persist/publish delta.
4. Add consumer checkpoint.
5. Build kind/relation dependency index.

## Workstream E — Pattern engine

1. Freeze DSL v1 grammar.
2. Parser + AST.
3. Semantic validation.
4. Dependency signature.
5. Portable evaluator.
6. Match state store.
7. transition generation.
8. behavior integration.
9. native adapter planner spike.

## Workstream F — Scenario engine

1. temporal base view;
2. overlay storage;
3. scenario query composition;
4. diff;
5. impact;
6. policy run;
7. proposal generation.

## Workstream G — UI

1. app shell;
2. entity routing;
3. inspector;
4. Lens SDK;
5. universal actions;
6. Path to Production;
7. live GraphDelta refresh;
8. pack UI registry.

## Workstream H — Packs

1. manifest v2;
2. contribution validation;
3. activation transaction;
4. declarative UI;
5. WASM runtime;
6. sample packs;
7. TCK and compatibility matrix.

## Suggested implementation discipline

Every workstream must produce:
- ADR status update;
- contract tests;
- benchmark budget;
- migration note;
- observability;
- security review;
- example/demo.
