# Tenant Migration Runbook

## Overview

This runbook covers the procedure for migrating a tenant from one cell to another.

## Prerequisites

- `golang` CLI ≥ 1.23
- Access to source and destination cells
- Migration plan approved (dry-run passed)

## Procedure

### Step 1: Dry-Run Migration

```bash
go run ./cmd/migrate/main.go \
  --tenant tenant-123 \
  --from cell-a \
  --to cell-b \
  --dry-run
```

Expected output: Diff report with 0 discrepancies.

### Step 2: Plan Cutover Window

Set the cutover window based on tenant activity:
- Minimum: 5 minutes
- Maximum: 1 hour
- Default: 15 minutes

### Step 3: Execute Cutover

```bash
go run ./cmd/migrate/main.go \
  --tenant tenant-123 \
  --from cell-a \
  --to cell-b \
  --cutover-window 15m
```

### Step 4: Verify

Check that:
1. Tenant events are routing to new cell
2. No data loss (event count match)
3. All events have correct tenant scope

### Rollback

If issues are detected during the observation window:

```bash
go run ./cmd/migrate/main.go \
  --tenant tenant-123 \
  --rollback
```

## Events

| Event | Description |
|-------|-------------|
| `tenant.migration.started.v1` | Migration initiated |
| `tenant.migration.shadowed.v1` | Shadow reads completed |
| `tenant.migration.cutover.v1` | Cutover began |
| `tenant.migration.completed.v1` | Migration succeeded |
| `tenant.migration.failed.v1` | Migration failed |

## References

- ADR-075: Tenant Migration
- REQ-MIG-001 through REQ-MIG-004
