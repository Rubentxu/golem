# Roadmap — From GOLEM Today to Active Engineering Graph

> **Status:** Proposed reference specification  
> **Project:** GOLEM — Go Open Lifecycle & Engineering Manager  
> **Baseline reviewed:** `main` around commit `113c868…` (2026-08-19)  
> **Goal:** evolve GOLEM into an **Active Engineering Graph** / auditable engineering digital twin.  

## Phase 0 — Correctness and architecture guardrails

### Deliver
- projection no-op checkpoint fix;
- apply-all projection contract;
- atomic command idempotency design;
- bounded-context architecture fitness tests;
- regression TCKs.

### Exit
No known replay/checkpoint partial-consumption defect.

---

## Phase 1 — Modular vertical hexagons

### Deliver
- context-owned ports;
- Projection Registry;
- API module registration;
- slimmer runtime supervisors;
- composition root cleanup.

### Exit
A new core domain module can be added without editing central projector/router switches.

---

## Phase 2 — Graph platform v2

### Deliver
- directional traversal;
- Graph Schema Registry;
- provenance/assertion types;
- portable/advanced graph capability contracts;
- Graph query cost budgets.

### Exit
Two graph adapters pass portable TCK; at least one advanced-capability spike passes.

---

## Phase 3 — Active Graph foundation

### Deliver
- GraphDelta;
- delta dependency index;
- Pattern Registry;
- DSL compiler;
- persisted match transitions;
- deterministic relation behaviors.

### Exit
Representative CVE propagation and architecture-drift behaviors run incrementally.

---

## Phase 4 — Temporal scenarios and proposals

### Deliver
- as-of graph reads;
- sparse scenario overlays;
- graph diff;
- impact;
- proposal lifecycle;
- policy evaluation.

### Exit
A release-change scenario can be simulated and converted into approved commands without mutating base state during analysis.

---

## Phase 5 — Product shell v2

### Deliver
- intent-oriented navigation;
- entity pages;
- Why/Impact/Simulate/Propose;
- Lens framework;
- Path to Production;
- accessible alternate views.

### Exit
Primary workflows no longer require raw graph exploration.

---

## Phase 6 — Capability Pack SDK

### Deliver
- schema contributions;
- projectors;
- patterns;
- behaviors;
- UI contributions;
- blueprints;
- signed package provenance;
- WASM sandbox spike → implementation.

### Exit
A Kubernetes or GitHub pack adds meaningful full-stack behavior without core changes.

---

## Phase 7 — IDP + Delivery Governance

### Deliver
- Catalog Lens;
- Blueprints/golden paths;
- environments;
- graph-backed gates;
- UAT/test evidence;
- supply-chain release readiness.

### Exit
A service can be created, verified and promoted with graph-backed evidence end to end.

---

## Phase 8 — Engineering intelligence

### Deliver
- continuous architecture;
- incident causality;
- cost Lens;
- grounded agent investigations;
- stale decision detection.

### Exit
GOLEM produces explainable, evidence-backed engineering insights not available from a single provider suite.
