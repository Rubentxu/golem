# Source Inspirations and Design Boundaries

> **Status:** Proposed reference specification  
> **Project:** GOLEM — Go Open Lifecycle & Engineering Manager  
> **Baseline reviewed:** `main` around commit `113c868…` (2026-08-19)  
> **Goal:** evolve GOLEM into an **Active Engineering Graph** / auditable engineering digital twin.  

## GOLEM current design

This specification builds on the project's existing:
- Graph Journal;
- Engineering Graph;
- Behavior Runtime;
- Lenses;
- scenarios/proposals;
- capability packs;
- supply-chain entities;
- SaaS cell architecture;
- architecture fitness tests.

## Backstage

Ideas deliberately reused:
- entity-oriented software catalog;
- ownership;
- plugin/extensibility model;
- software templates/golden paths.

Not copied:
- a separate catalog becoming the master database;
- frontend plugin model as the only extension layer.

## Azure DevOps

Ideas deliberately reused:
- cohesive lifecycle navigation;
- Boards/Repos/Pipelines/Test/Artifacts/Environment concepts;
- approvals/checks;
- UAT/test management.

Not copied:
- provider-specific pipelines as GOLEM's canonical execution model;
- siloed domain stores.

## ActiveGraph / Active Graph ideas

Ideas reused/evolved:
- event history + graph state;
- reactive graph relationships;
- scoped views;
- forks/scenarios;
- graph state driving automation.

GOLEM specialization:
- multi-tenant SaaS;
- engineering lifecycle ontology;
- supply-chain evidence;
- policy/gate controls;
- full extension packs;
- grounded engineering agents.
