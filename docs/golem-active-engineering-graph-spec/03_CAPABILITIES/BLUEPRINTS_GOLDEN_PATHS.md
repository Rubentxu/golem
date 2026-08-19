# Blueprints and Golden Paths

> **Status:** Proposed reference specification  
> **Project:** GOLEM — Go Open Lifecycle & Engineering Manager  
> **Baseline reviewed:** `main` around commit `113c868…` (2026-08-19)  
> **Goal:** evolve GOLEM into an **Active Engineering Graph** / auditable engineering digital twin.  

## Definition

A Blueprint is a parameterized desired engineering change package.

Example: `Go API Service`.

## Blueprint may declare

- catalog entities;
- repository;
- CI pipeline;
- artifact type;
- SBOM/provenance requirements;
- runtime resources;
- environments;
- ownership;
- SLO;
- docs;
- policy;
- architecture relationships.

## Execution model

```text
Parameters
 → Plan
 → Scenario
 → Graph Preview
 → Validation
 → Policy
 → Approval
 → Change Proposal
 → Commands
```

## Benefits

Compared with direct scaffolding:
- full preview;
- impact analysis;
- policy before mutation;
- audit;
- partial provider independence;
- agent-assisted customization.

## Blueprint SDK

A pack can contribute:
- schema;
- steps;
- tool requirements;
- preview renderer;
- validation;
- rollback semantics;
- postconditions.

## Postconditions

Blueprint completion is evaluated from graph state, not only from tool exit codes.
