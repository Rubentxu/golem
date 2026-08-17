# Escala y rendimiento

## Workloads
OLTP work management, SCM/CI/SBOM ingest, graph traversal, blast-radius, search, analytics, agents y rebuild.

## QueryBudget
`deadline, max_nodes, max_edges, max_depth, max_result_bytes, max_db_time`.

## Hot path rules
No sink I/O, LLM calls, vulnerability feed calls, sync search indexing ni large blob upload dentro de graph transaction.

## Ingestion
Batch, idempotent upsert, fingerprint, checkpoint, adaptive backpressure y partition por tenant/provider/repository.

## Graph UI
API entrega neighborhoods/clusters/paged subgraphs.

## Benchmark gate
GraphStore se prueba con 10M/100M y, cuando sea viable, 1B synthetic edges; mixed R/W, bounded traversals, fan-out, tenant partition, bulk load, failover, recovery y coste operativo.
