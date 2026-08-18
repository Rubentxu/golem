# Cell Loss Recovery Runbook

> ADR-079 + REQ-DR-002. Runbook for cell data loss scenarios in production.

## Symptom

- `dr.restore.failed.v1` events in journal
- Cell routing `cell.routing.conflict_detected.v1` events spike
- `/healthz` reports cell unhealthy
- Tenant requests 5xx for cell-specific routes

## Triage (5 minutes)

1. **Identify affected cell**: `golemctl cell status` — look for `state: degraded` or `state: lost`.
2. **Check DR snapshot age**: `golemctl dr snapshots list --cell <cell-id>`. If last snapshot < RPO target (15 min), restore is feasible.
3. **Check cell routing**: `golemctl cell routing show --cell <cell-id>`. Look for tenant hash collisions.
4. **Review OTel metrics**: cell.cell.unavailable_gauge, journal.replay.errors.

## Mitigation (15 minutes)

### Option A: Restore from snapshot (preferred if RPO acceptable)

```bash
golemctl dr restore \
  --cell <cell-id> \
  --snapshot <snapshot-id> \
  --target-cell <new-cell-id> \
  --rto-budget 1h
```

This triggers:
- `dr.restore.drill.completed.v1` (or `dr.restore.failed.v1`)
- Cell promotion: `cell.promoted.v1` for new-cell-id
- Cell demotion: `cell.demoted.v1` for old cell-id
- Tenant migration events for affected tenants

### Option B: Cell drain + migrate tenants

```bash
golemctl cell drain --cell <cell-id> --reason "data-loss"
golemctl tenant migrate plan --from <cell-id> --to <target-cell> --dry-run
golemctl tenant migrate execute --plan-id <plan-id>
```

## Post-mortem

- File incident with cell ID, time-to-detect, time-to-mitigate, RTO/RPO observed.
- Update threat model if new vector discovered.
- Schedule retro within 48h.

## Escalation

| Severity | Channel | Escalation time |
|---|---|---|
| Sev1 (production cells down) | PagerDuty + #incidents | 5 min |
| Sev2 (degraded cell) | Slack #ops | 30 min |
| Sev3 (snapshot age warning) | Email | next business day |