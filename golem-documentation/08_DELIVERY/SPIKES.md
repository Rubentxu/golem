# Spikes obligatorios

## SP-001 GraphStore benchmark
HugeGraph, NebulaGraph y al menos una alternativa adicional. Synthetic Engineering Graph; 10M/100M y escala mayor cuando sea viable; mixed R/W, bounded traversal, fan-out, bulk, failover y ops cost.

## SP-002 Journal persistence
Append throughput, partitioning, snapshots, retention y recovery.

## SP-003 Projection rebuild
Borrar graph projection y reconstruir desde Journal.

## SP-004 Cell routing
Tenant assignment, sticky routing, cell failure y migration.

## SP-005 Pattern compiler
EBNF→AST→candidate index→graph plan; validar cost bounding.

## SP-006 Scenario overlay
Copy-on-write/event overlay/branch namespace.

## SP-007 WASM sandbox
wazero, host capabilities, time/memory/fuel y cold start.

## SP-008 Provider TCK
Segundo fake/reference adapter para demostrar detección de divergencia semántica.

## SP-009 Tuleap migration
Muestra realista de projects/trackers/planning/tests y mapping.

## SP-010 Supply-chain corpus
SPDX, CycloneDX, SLSA/in-toto, signatures y VEX de herramientas distintas.

## SP-011 Massive graph UX
Incremental rendering, clustering y semantic zoom.

## SP-012 Agent prompt-injection boundary
Contenido malicioso, tool policy, egress y proposal-only.

## SP-013 Multi-region
Sólo cuando exista necesidad. Simular latency/conflicts antes de implementar.
