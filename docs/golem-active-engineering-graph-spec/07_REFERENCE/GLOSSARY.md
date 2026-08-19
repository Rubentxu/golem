# Glossary

> **Status:** Proposed reference specification  
> **Project:** GOLEM — Go Open Lifecycle & Engineering Manager  
> **Baseline reviewed:** `main` around commit `113c868…` (2026-08-19)  
> **Goal:** evolve GOLEM into an **Active Engineering Graph** / auditable engineering digital twin.  

**Active Engineering Graph** — GOLEM's reactive, temporal engineering knowledge model plus behaviors and scenarios.

**Graph Journal** — append-only authoritative history of accepted engineering events.

**Engineering Graph** — semantic graph projection derived from the journal and external canonical events.

**GraphDelta** — normalized description of graph changes produced by a successfully projected journal event.

**Pattern** — declarative graph condition compiled and evaluated within budgets.

**MATCHED** — pattern binding transitioned from absent to present.

**UNMATCHED** — pattern binding transitioned from present to absent.

**CHANGED** — pattern binding remains present but material bound state changed.

**Behavior** — deterministic/workflow/agentic reaction to event or pattern transition.

**Lens** — bounded semantic view/query and visual projection for a user question.

**Scenario** — isolated graph overlay based on a specific historical base position.

**Change Proposal** — auditable proposed mutation that requires validation/policy/approval before commands.

**Capability Pack** — versioned, permissioned extension package contributing semantics, backend capabilities, automation and UI.

**Blueprint** — parameterized desired-change plan evaluated first as a scenario.

**ASSERTED** — fact obtained from a source/authority.

**INFERRED** — deterministic derived fact.

**PREDICTED** — heuristic/agentic fact with uncertainty.

**Evidence** — reference that supports a fact, state or decision.

**Cell** — horizontally scalable SaaS deployment unit containing engineering data/workloads for assigned tenants.
