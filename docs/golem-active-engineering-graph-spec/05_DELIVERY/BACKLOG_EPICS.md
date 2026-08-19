# Implementation Backlog — Epics

> **Status:** Proposed reference specification  
> **Project:** GOLEM — Go Open Lifecycle & Engineering Manager  
> **Baseline reviewed:** `main` around commit `113c868…` (2026-08-19)  
> **Goal:** evolve GOLEM into an **Active Engineering Graph** / auditable engineering digital twin.  

## EPIC-01 Projection correctness
**Priority:** P0

Stories:
- handle no-op checkpoint advancement;
- project all chunks;
- crash/retry tests;
- replay determinism benchmark.

## EPIC-02 Command atomicity
**Priority:** P0

Stories:
- CommandJournal contract;
- adapter implementation;
- duplicate race tests;
- registry downgrade to projection/cache.

## EPIC-03 Architecture boundaries
**Priority:** P0

Stories:
- dependency matrix;
- archtest rules;
- projection registry;
- API module registry;
- narrow context ports.

## EPIC-04 Graph Schema Registry
**Priority:** P1

Stories:
- schema descriptors;
- compatibility;
- mutation validation;
- pack namespaces.

## EPIC-05 Provenance and trust semantics
**Priority:** P1

Stories:
- ASSERTED/INFERRED/PREDICTED;
- evidence refs;
- confidence;
- UI provenance inspector.

## EPIC-06 GraphDelta
**Priority:** P1

Stories:
- delta model;
- publisher;
- durability;
- lag metrics;
- UI stream.

## EPIC-07 Pattern Engine
**Priority:** P1

Stories:
- DSL parser;
- planner;
- transition store;
- delta index;
- loop safety.

## EPIC-08 Scenario Engine
**Priority:** P1

Stories:
- temporal base;
- overlays;
- diff;
- impact;
- promotion to proposal.

## EPIC-09 Entity UX
**Priority:** P1

Stories:
- shell;
- entity router;
- inspector;
- universal actions;
- lens framework.

## EPIC-10 Capability Pack v2
**Priority:** P1

Stories:
- manifest;
- UI contributions;
- schema;
- projectors/patterns;
- WASM.

## EPIC-11 IDP
**Priority:** P2

Stories:
- Catalog Lens;
- Blueprints;
- ownership;
- docs freshness;
- golden path scenario preview.

## EPIC-12 Delivery governance
**Priority:** P2

Stories:
- Environment;
- gate engine;
- UAT evidence;
- supply-chain rules.

## EPIC-13 Continuous Architecture
**Priority:** P2

## EPIC-14 Incident Intelligence
**Priority:** P2

## EPIC-15 FinOps Lens
**Priority:** P3
