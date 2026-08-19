# Observability and SaaS Runtime

> **Status:** Proposed reference specification  
> **Project:** GOLEM — Go Open Lifecycle & Engineering Manager  
> **Baseline reviewed:** `main` around commit `113c868…` (2026-08-19)  
> **Goal:** evolve GOLEM into an **Active Engineering Graph** / auditable engineering digital twin.  

## Required telemetry

### Journal
- append latency;
- head position;
- conflicts;
- command dedupe.

### Projections
- journal head vs checkpoint;
- apply latency;
- rebuild progress;
- projection errors.

### Graph
- query latency/cost;
- nodes/edges returned;
- truncation rate;
- mutation latency.

### Active Graph
- GraphDelta lag;
- candidate patterns;
- match latency;
- transition counts;
- dedupe/cooldown counts.

### Behaviors
- execution count;
- success/failure;
- budget exhaustion;
- output events/proposals.

### Agents
- model;
- tokens;
- cost;
- tool calls;
- latency;
- proposal acceptance rate;
- grounding/evidence coverage.

## Cell health

Each cell publishes:
- tenant count;
- journal throughput;
- graph size;
- CPU/memory;
- storage;
- queue lag;
- SLO burn.

## Noisy-neighbor control

Budget per tenant:
- writes/sec;
- graph query cost;
- pattern executions;
- agent tokens/cost;
- background job concurrency.

## Cardinality

Do not put tenant IDs, entity IDs or command IDs into unbounded metric labels. Use traces/logs for high-cardinality dimensions.
