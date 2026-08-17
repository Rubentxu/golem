# Provider Migration Framework

## Flujo
`Source → canonical snapshot/export → target bulk load → journal tail/dual projection → shadow reads → semantic diff → tenant/cell cutover → observe → retire`.

## Requisitos
Checkpoint resumable, checksums/counts, semantic sample queries, target TCK, rollback window y audit events.

## Canonical graph export
Nodes/edges Parquet o JSONL + ontology/schema + Journal position + manifest/checksums.

## Replaceability Level
- R0 vendor coupled
- R1 port exists
- R2 second adapter exists
- R3 TCK passes
- R4 migration exercised
- R5 failover/chaos migration exercised

Objetivo para providers estratégicos en producción: **R4+**.
