# Migration Plan

> **Status:** Proposed reference specification  
> **Project:** GOLEM — Go Open Lifecycle & Engineering Manager  
> **Baseline reviewed:** `main` around commit `113c868…` (2026-08-19)  
> **Goal:** evolve GOLEM into an **Active Engineering Graph** / auditable engineering digital twin.  

## Strategy

Use strangler-style internal migration while keeping public behavior stable.

## 1. Projector

### Before
Central `switch event.EventType`.

### Transition
- wrap existing switch as fallback projector;
- add registry;
- migrate one event family at a time;
- remove fallback once coverage complete.

## 2. Graph ports

Introduce narrow interfaces without deleting GraphStore.
Migrate application consumers incrementally.
GraphStore remains adapter-facing composite until callers are migrated.

## 3. Provenance

Use expand → backfill → contract:
1. add optional provenance/assertion fields;
2. populate for new events;
3. replay/backfill projection;
4. make required for selected kinds.

## 4. UI

Keep existing routes while new entity shell is introduced behind feature flag.
Deep links must remain stable or redirect.

## 5. Capability Packs

Manifest v1 remains readable.
Manifest v2 adds contribution sections.
Activation negotiates supported versions.

## 6. Active Graph

Behaviors can coexist:
- legacy event subscription;
- new graph pattern subscription.

Migrate behavior-by-behavior.

## 7. Scenario

Do not mutate current canonical graph format initially. Add overlay query composition at application/query layer.

## Rollback

Every migration phase must define:
- previous binary compatibility;
- projection rebuild strategy;
- feature flag;
- data downgrade/ignore behavior.
