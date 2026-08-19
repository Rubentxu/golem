# API and Query Architecture

> **Status:** Proposed reference specification  
> **Project:** GOLEM — Go Open Lifecycle & Engineering Manager  
> **Baseline reviewed:** `main` around commit `113c868…` (2026-08-19)  
> **Goal:** evolve GOLEM into an **Active Engineering Graph** / auditable engineering digital twin.  

## API layers

### Commands
Stable action/use-case API. Returns command receipt and journal position.

### Entity queries
Stable read DTOs around core concepts.

### Graph queries
Advanced bounded query API.

### Streaming
GraphDelta / attention / long-running job updates where appropriate.

## Avoid exposing storage dialects as public API

Cypher/Gremlin/native DB syntax must not become the long-lived public contract.

GOLEM owns:
- Pattern DSL;
- Lens spec;
- path/impact query DTOs.

Adapters translate to native engines where supported.

## Semantic query endpoints

Examples:

```text
POST /api/v1/explain/why
POST /api/v1/impact
POST /api/v1/path
POST /api/v1/evidence
POST /api/v1/graph/diff
POST /api/v1/scenarios
POST /api/v1/proposals
```

## Read-your-write

Command receipt includes journal position.

Clients may request:
- eventual read;
- wait until projection ≥ receipt position with bounded timeout.

## API module registration

HTTP routes are contributed by bounded-context modules into the edge composition root. A monolithic `routes()` switch should not grow indefinitely.

## Graph query safety

Every advanced query has:
- tenant derived from authenticated context;
- explicit cost budget;
- maximum result size;
- deadline;
- server-enforced defaults/caps.
