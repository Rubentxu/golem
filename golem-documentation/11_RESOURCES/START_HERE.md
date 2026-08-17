# START HERE

## Lectura inicial
Vision → PRD → Architecture → Graph Journal → Graph Model → Ports & Adapters → ADR Catalog → Roadmap → Implementation Sequence.

## Primera reunión técnica
Resolver repo/bootstrap, M0 owners, benchmark dataset, Journal reference implementation, tenant isolation y first vertical slice.

No debatir todavía marketplace, active-active multi-region o agents avanzados.

## Primer PR
Crear `cmd/`, `internal/domain`, `internal/application`, `internal/ports`, `adapters`, `tck`, `api`, `schemas`, `docs/adr` y el test de forbidden vendor imports.

## Primer demo
`Project + WorkItem: command → event → Journal → graph projection → query → replay → same graph digest`.
