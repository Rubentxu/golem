# Capability Packs — Full-stack Extension Platform

> **Status:** Proposed reference specification  
> **Project:** GOLEM — Go Open Lifecycle & Engineering Manager  
> **Baseline reviewed:** `main` around commit `113c868…` (2026-08-19)  
> **Goal:** evolve GOLEM into an **Active Engineering Graph** / auditable engineering digital twin.  

## Objective

Make Capability Packs the single extensibility mechanism for the product.

## Pack contributions

A pack MAY contribute:

### Semantic
- ontology types;
- relation types;
- event schemas;
- validation rules.

### Backend
- command handlers;
- query handlers;
- projectors;
- patterns;
- behaviors;
- workflow definitions;
- policy modules;
- tools.

### Integration
- event ingestion adapters;
- remote connectors;
- webhooks;
- polling jobs.

### UX
- navigation items;
- routes;
- entity tabs;
- entity cards;
- inspector sections;
- actions;
- graph Lenses;
- dashboard widgets;
- search providers.

### Automation
- blueprints;
- agent skills;
- prompt assets;
- proposal templates.

## Runtime types

### WASM
Preferred for portable, sandboxed deterministic extension logic where feasible.

### Remote
For heavy, privileged or ecosystem-specific runtimes.

### Native
Reserved for trusted core modules.

## Security

Manifest declares:
- capabilities;
- permissions;
- budgets;
- entrypoints;
- ontology;
- UI contributions;
- migrations;
- integrity digest;
- optional signature/provenance.

## Activation sequence

1. verify package integrity/signature;
2. validate API compatibility;
3. validate ontology compatibility;
4. validate permissions;
5. validate UI contribution schema;
6. register disabled contributions;
7. run migrations if supported;
8. activate atomically;
9. emit activation event.

## UX contribution safety

Remote packs must not inject arbitrary browser JavaScript by default. Prefer declarative contribution schemas and sandboxed extension surfaces.
