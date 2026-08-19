# Hexagonal Architecture and Modularization

> **Status:** Proposed reference specification  
> **Project:** GOLEM — Go Open Lifecycle & Engineering Manager  
> **Baseline reviewed:** `main` around commit `113c868…` (2026-08-19)  
> **Goal:** evolve GOLEM into an **Active Engineering Graph** / auditable engineering digital twin.  

## Goal

Move from a broadly hexagonal repository to **bounded-context-owned hexagons**.

## Recommended structure

```text
internal/
  platform/
    journal/
    identity/
    tenancy/
    policy/
    observability/
    extension/

  contexts/
    plan/
    work/
    scm/
    build/
    verification/
    supplychain/
    release/
    runtime/
    governance/

  graph/
    model/
    projection/
    query/
    pattern/
    reactive/
    scenario/
    analytics/

  automation/
    behavior/
    agent/
    proposal/

  composition/
```

## Dependency rules

```text
context/domain        -> shared kernel value types only
context/application   -> context/domain + owned ports
context/api           -> inbound application ports
adapter               -> outbound ports
graph projection      -> canonical events + graph mutation contract
composition           -> may know implementations
context A             -X-> context B internals
```

## Port ownership

Prefer:

```go
type ReleaseEvidenceReader interface { ... }
```

owned by Release application code over injecting a global `GraphStore`.

## Architecture fitness functions

Add CI checks for:
- domain cannot import application/api/adapters;
- context internals are isolated;
- application cannot import HTTP;
- no adapter type crosses a port boundary;
- composition is the only unrestricted wiring layer.

## Third-party dependency rule

Keep vendor SDKs behind adapters. Do **not** equate hexagonal architecture with stdlib-only code. Neutral implementation libraries can be allowed when they do not leak vendor semantics into domain contracts.
