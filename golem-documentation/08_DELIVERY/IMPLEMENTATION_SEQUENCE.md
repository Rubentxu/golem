# Secuencia de implementación — primeros 90 días

## Semana 1–2
Monorepo; domain/application/ports/adapters; forbidden-import lint; event schema; IDs/Clock; TenantContext; ADR tooling.

## Semana 3–4
JournalPort reference; replay; in-memory GraphStore; GraphStoreTCK; canonical node/edge; command receipt/idempotency.

## Semana 5–6
Graph candidate adapters; benchmark harness; checkpoints; outbox; NATS adapter; OpenTelemetry.

## Semana 7–8
Project + WorkItem vertical slice; REST API; minimal UI; history; neighborhood endpoint.

## Semana 9–10
Requirements + trace; dynamic schemas/workflows; search projection.

## Semana 11–12
Tuleap fixture import; board; trace explorer; performance baseline; threat-model review.

## Vertical slice obligatorio

`HTTP command → domain event → Journal → graph projection → search → UI → replay → same graph digest`

## No empezar todavía
Microservices por context, active-active multi-region, LLM agents, marketplace, advanced analytics ni docenas de adapters.
