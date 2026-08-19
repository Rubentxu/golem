# Graph Ontology and Schema Registry

> **Status:** Proposed reference specification  
> **Project:** GOLEM — Go Open Lifecycle & Engineering Manager  
> **Baseline reviewed:** `main` around commit `113c868…` (2026-08-19)  
> **Goal:** evolve GOLEM into an **Active Engineering Graph** / auditable engineering digital twin.  

## Why a registry is required

A growing `map[string]any` model is flexible but eventually causes:
- schema drift;
- inconsistent relation semantics;
- weak validation;
- incompatible plugin assumptions;
- hard-to-optimize queries.

## Graph Schema Registry

Each type declares:

```yaml
kind: Service
version: 1
properties:
  name: {type: string, required: true}
  criticality: {type: enum, values: [low, medium, high, critical]}
identity:
  strategy: opaque
```

Edges declare:

```yaml
type: DEPENDS_ON
sourceKinds: [Service, Component]
targetKinds: [Service, Resource]
direction: directed
temporal: true
cardinality: many-to-many
```

## Assertion types

Every material assertion is one of:

### ASSERTED
Observed from an authoritative/external source.

### INFERRED
Derived deterministically from rules.

### PREDICTED
Heuristic or AI-generated inference.

## Provenance

Recommended fields:

```text
source_system
source_identity
source_event
observed_at
valid_from
valid_to
assertion_type
confidence?
evidence_refs[]
derived_by?
```

## Ontology governance

- core namespaces are stable;
- packs use namespaced kinds when appropriate;
- migration is expand → backfill → contract;
- schema changes require compatibility checks;
- relation direction and meaning are immutable once public.
