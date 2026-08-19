# SPIKE — WASM Capability Pack Runtime

> **Status:** Proposed reference specification  
> **Project:** GOLEM — Go Open Lifecycle & Engineering Manager  
> **Baseline reviewed:** `main` around commit `113c868…` (2026-08-19)  
> **Goal:** evolve GOLEM into an **Active Engineering Graph** / auditable engineering digital twin.  

## Goal

Validate WASM as a secure extension runtime.

## Prototype capabilities

- deterministic projector;
- pattern helper;
- behavior handler;
- no network by default;
- host graph query through bounded capability;
- event/proposal output;
- time/memory budget.

## Measure

- cold start;
- execution overhead;
- memory;
- host-call overhead;
- isolation;
- developer tooling;
- versioning.

## Security tests

- filesystem attempt;
- network attempt;
- excessive memory;
- infinite loop;
- undeclared capability;
- malformed output.

## Exit

Decide which extension classes should use WASM versus remote protocol.
