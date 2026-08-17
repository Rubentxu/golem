# Arquitectura evolutiva

## Etapa 0 — Kernel
Domain primitives, event envelope, Journal port, GraphStore port, command/query boundary, tenant context y TCK framework.

## Etapa 1 — Modular cell
API + workers + journal + graph projection + object store + broker + OTel.

## Etapa 2 — High-throughput ingestion
Workers separados para SCM, CI, SBOM y vulnerability feeds; batch + checkpoints.

## Etapa 3 — Hot contexts
Extraer sólo contexts con perfil de carga/ownership/aislamiento que lo justifique.

## Etapa 4 — Multi-cell
Placement, rebalance y tenant cell migration.

## Etapa 5 — Multi-region
DR/read replicas primero. Active-active writes sólo tras spike.

## Fitness functions
Forbidden imports, TCK, replay digest, schema compatibility, tenant isolation, query budgets, performance y context dependency cycles.
