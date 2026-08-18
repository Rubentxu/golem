# Tenant Cutover Runbook

> ADR-075 + REQ-TENANT-MIG-001. Runbook for tenant migration online cutover.

## Pre-cutover Checklist

- [ ] Dry-run executed successfully: `golemctl tenant migrate plan --tenant <id> --target-cell <cell> --dry-run`
- [ ] Conflict detection clean (no `cell.routing.conflict_detected.v1`)
- [ ] Source cell healthy: `golemctl cell status --cell <source-cell>`
- [ ] Target cell healthy: `golemctl cell status --cell <target-cell>`
- [ ] SLO budget available (no `slo.budget.exhausted.v1` in last 7 days)
- [ ] Off-peak window selected (typically 02:00-04:00 UTC)
- [ ] Human approval recorded (audit log entry)

## Cutover Steps

1. **Shadow writes** (10 min):
   ```bash
   golemctl tenant migrate shadow --tenant <id> --target-cell <cell>
   ```
   Emits `tenant.migration.shadowed.v1`. Both cells receive writes; source still primary.

2. **Verify shadow consistency** (5 min):
   ```bash
   golemctl tenant migrate diff --tenant <id>
   ```
   Must show 0 divergent events.

3. **Atomic cutover** (atomic):
   ```bash
   golemctl tenant migrate cutover --tenant <id> --target-cell <cell>
   ```
   Emits `tenant.migration.cutover.v1` then `tenant.migration.completed.v1` (or `failed.v1`).

4. **Cleanup** (10 min):
   ```bash
   golemctl tenant migrate cleanup --tenant <id>
   ```
   Removes shadow state from source cell.

## Rollback

If `tenant.migration.failed.v1` fires:

```bash
golemctl tenant migrate rollback --tenant <id> --to-source
```

Source cell reactivated, target cell drained.

## Post-cutover

- Monitor SLO `tenant.migration.success_rate` for 24h.
- File post-mortem if any R4+ violations occurred.
- Update tenant-catalog with new cell assignment.

## Escalation

Tenant cutover is always Sev2. Escalate to Sev1 only if multiple tenants fail cutover simultaneously.