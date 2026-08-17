# GOLEM — Go Open Lifecycle & Engineering Manager

> **Graph-native, event-driven, auditable software engineering control plane.**

**Document baseline:** 0.1  
**Architecture style:** Hexagonal + event-sourced + graph-native + evolutionary  
**Primary implementation language:** Go  
**Deployment target:** Multi-tenant SaaS with cell-based horizontal scaling  


# Arquitectura de referencia

## Vista lógica

```mermaid
flowchart TB
  UI[Web / CLI / API Clients]
  EDGE[API Edge]
  APP[Application Layer]
  DOM[Domain / Bounded Contexts]
  PORTS[Hexagonal Ports]
  J[Graph Journal]
  GP[Engineering Graph Projection]
  BUS[Event Transport]
  BEH[Behavior Runtime]
  POL[Policy Engine Port]
  OBJ[Object Store Port]
  SEARCH[Search Projection]
  ANA[Analytics Projection]
  AG[Agent Runtime]

  UI --> EDGE --> APP --> DOM
  DOM --> J
  APP --> PORTS
  J --> GP
  J --> BUS
  BUS --> BEH
  BEH --> APP
  APP --> POL
  APP --> OBJ
  J --> SEARCH
  J --> ANA
  AG --> APP
  AG --> GP
```

## Fuente de verdad

GOLEM separa:
1. **modelo canónico:** Engineering Graph;
2. **historia autoritativa:** Graph Journal append-only;
3. **proyecciones:** graph DB materializada, search, analytics, caches.

Una projection debe poder reconstruirse desde Journal + snapshots/checkpoints verificables.

## Evolución

### A — Modular monolith por cell
Un binario API y workers separados donde convenga. Boundaries se mantienen aunque compartan proceso.

### B — Process isolation
Projection workers, ingestion, behavior workers y heavy jobs se separan sin cambiar contracts.

### C — Service extraction
Sólo con evidencia de carga, ownership, escalado independiente o risk isolation.

## Cell architecture

```mermaid
flowchart LR
  CP[Global Control Plane]
  R[Cell Router]
  C1[Cell A]
  C2[Cell B]
  C3[Cell C]
  CP --> R
  R --> C1
  R --> C2
  R --> C3
```

Control plane: tenant catalog, routing, entitlements y operación global mínima. Datos de ingeniería en la cell.

## Write path

`Request → AuthN/AuthZ → Command → Domain validation → Journal append → Accepted Event → Outbox/Projection → Async reactions`

## Read path

`Query → Tenant Scope → Query Budget → Graph/Search/Analytics Projection → Stable DTO`

## Consistencia

- command acceptance fuerte dentro del invariant boundary;
- projections eventuales con lag medido;
- sinks at-least-once e idempotentes;
- command receipt incluye Journal position para read-your-write opcional.

## Extensibilidad

Ports/adapters, Capability Packs, WASM y remote plugin protocol.

## Seguridad

Tenant, actor, correlation, causation y policy result acompañan acciones relevantes. Secrets nunca entran en Journal.
