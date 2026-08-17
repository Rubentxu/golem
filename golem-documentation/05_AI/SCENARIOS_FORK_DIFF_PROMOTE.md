# Scenarios — Fork / Diff / Promote

## Scenario
Base Journal position + base projection revision + overlay events + optional provider/behavior overrides + budget.

## Uses
Dependency upgrade, policy change, architecture refactor, agent model comparison, behavior shadow y provider migration.

## Storage
Base snapshot/projection + overlay delta; evitar copiar graph completo.

## Diff
Nodes/edges, policy decisions, affected releases/deployments, risk, cost y latency.

## Promote
Verify lineage → parent recheck → conflicts → policy → approval → atomic accepted delta → `scenario.promoted.v1`.

No semantic auto-merge por defecto.
