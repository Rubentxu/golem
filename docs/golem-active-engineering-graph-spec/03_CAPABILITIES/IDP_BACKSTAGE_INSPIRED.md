# IDP Capabilities Inspired by Backstage

> **Status:** Proposed reference specification  
> **Project:** GOLEM — Go Open Lifecycle & Engineering Manager  
> **Baseline reviewed:** `main` around commit `113c868…` (2026-08-19)  
> **Goal:** evolve GOLEM into an **Active Engineering Graph** / auditable engineering digital twin.  

## Adopt concepts, not product duplication

### Catalog
The catalog is a **Lens over the Engineering Graph**, not an independent database.

Core catalog entities:
- Domain
- System
- Service/Component
- API
- Resource
- Team/Owner

## Entity experience

Entity pages aggregate:
- ownership;
- architecture;
- repository;
- CI/CD;
- artifacts;
- SBOM/security;
- runtime;
- SLO;
- ADRs;
- policy state.

## Golden paths

Backstage-style scaffolding evolves into GOLEM Blueprints:
- parameterized;
- previewable;
- scenario-backed;
- policy-aware;
- graph-diffable;
- auditable.

## TechDocs equivalent

Documentation remains source-controlled, while GOLEM links docs to entities and can derive:
- documentation freshness;
- ownership;
- architecture references;
- missing documentation patterns.

## Plugin system

The useful idea is full-stack extensibility. GOLEM's Capability Pack model should become richer than page-level plugins because it can add semantic ontology and graph behaviors too.
