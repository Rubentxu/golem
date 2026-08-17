# Disaster Recovery

## Recovery hierarchy
1. Journal durability.
2. Canonical export/snapshots.
3. Object/evidence store.
4. Rebuild graph/search/analytics.

## Exercises
Delete search→rebuild; delete graph projection→rebuild; broker loss→republish; provider outage→migration/failover; cell loss→replacement cell.

## Verification
Event positions/counts, graph digest/sample invariants, object digests, tenant isolation y policy config.
