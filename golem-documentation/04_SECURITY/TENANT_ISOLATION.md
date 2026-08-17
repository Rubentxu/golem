# Tenant Isolation Specification

## MUST
TenantContext validado en command/query, graph tenant scope obligatorio, object namespace tenant, adapters reciben tenant explícito, TCK negative tests, background jobs preservan tenant y agents no cambian tenant dentro de Frame.

## SHOULD
Dedicated keys/cell para tiers regulados y tenant export/delete workflows.

## Isolation test
Dos tenants con external IDs idénticos, ingest concurrente, cross-read/write attempts, search/event/log leakage inspection y load test.
