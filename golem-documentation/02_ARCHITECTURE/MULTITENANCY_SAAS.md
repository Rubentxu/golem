# Multi-tenancy SaaS

## Estrategia
Cell-based. Cada tenant pertenece a una cell.

## Tenant context
Obligatorio en auth, commands, events, graph, object keys, background jobs y rate limits.

## Isolation
Application scope + graph partition/namespace + object prefix/policy + broker boundaries + quotas + integration credential isolation.

## Enterprise
Dedicated cell/resources, region pinning, custom IdP/keys/LLM provider mediante Provider Profile, sin fork del producto.

## Control plane
Sólo tenant catalog, cell assignment, entitlement y migration state. No graph queries de negocio.

## Noisy neighbor
Token buckets, concurrent query caps, graph budgets, ingestion quotas, background priorities y circuit breakers.
