# R4+ Execution Runbook

> ADR-075 + ADR-079. Strategic multi-cell R4+ rehearsal.

## What is R4+?

R4 (Migration Rehearsal) is the GOLEM M0 exit criterion. R4+ extends it to multi-cell operations:
- Cell promotion / demotion
- Cross-cell tenant migration
- DR drill (already weekly CI; R4+ validates production scenario)

## When to run R4+

- Before any production cutover (always)
- After major version upgrades (M8+)
- Quarterly GA cadence
- After any incident involving cell routing

## R4+ in staging

```bash
just r4 plan staging-prod
```

This:
- Generates a migration plan for `staging → prod`
- Runs dry-run with conflict detection
- Estimates RTO/RPO
- Outputs plan-id for approval

## R4+ execution

After human approval:

```bash
just r4 exec staging-prod --plan-id <id> --window <ISO-timestamp>
```

This:
- Executes shadow phase (writes to both cells)
- Cuts over atomically
- Emits `r4.completed.v1` or `r4.failed.v1`
- Records RTO/RPO observed vs target

## Acceptance criteria

- RTO observed ≤ 1h (soft, P95)
- RPO observed ≤ 15min
- Zero `cell.routing.conflict_detected.v1` during cutover
- Zero `tenant.migration.failed.v1`
- SLO budgets intact post-cutover

## Post-R4+

- File retro
- Update threat model
- Tune cell routing if hash collisions observed

## Escalation

R4+ in production is Sev2. R4+ rehearsal failures are Sev3 unless they block release.