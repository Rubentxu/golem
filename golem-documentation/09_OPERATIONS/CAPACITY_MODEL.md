# Capacity Model

## Tenant dimensions
Users, projects, work items, repos, commits/day, builds/day, artifacts/day, SBOM components, nodes/edges, event rate, object bytes y agent calls.

## Cell sizing
API CPU, worker concurrency, graph storage/query, Journal throughput, object bandwidth, search indexing y analytics ingestion.

## Placement
Weighted capacity score; rebalance antes de saturación.

## Quotas
Soft warning → throttling → hard protection según tier/capability.
