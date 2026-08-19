# Test Strategy

> **Status:** Proposed reference specification  
> **Project:** GOLEM — Go Open Lifecycle & Engineering Manager  
> **Baseline reviewed:** `main` around commit `113c868…` (2026-08-19)  
> **Goal:** evolve GOLEM into an **Active Engineering Graph** / auditable engineering digital twin.  

## Test pyramid

### Domain tests
Pure invariants and transitions.

### Application tests
Use-case orchestration through fake/narrow ports.

### Adapter TCKs
Every JournalStore, GraphStore and extension runtime adapter passes shared contracts.

### Replay tests
Same journal → same deterministic projection digest.

### Property tests
Useful for:
- graph mutation idempotency;
- pattern transitions;
- ordering;
- scenario diff;
- schema compatibility.

### Fault injection
Inject failures:
- before/after journal commit;
- during projection chunk N;
- before checkpoint;
- after graph apply;
- during behavior output;
- during pack activation.

## Critical regression cases

1. unknown event at end of replay batch;
2. known no-op event;
3. SBOM >500 operations;
4. duplicate command concurrent submissions;
5. behavior loop;
6. pattern changes from matched to unmatched;
7. scenario never leaks to canonical graph;
8. cross-tenant graph query attempt;
9. pack missing permission;
10. agent tool attempts unauthorized write.

## UI
- component tests;
- keyboard navigation;
- accessibility;
- graph/table semantic equivalence;
- large-subgraph performance.
