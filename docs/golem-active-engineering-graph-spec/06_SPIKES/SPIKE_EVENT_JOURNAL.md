# SPIKE — Journal Transaction and Idempotency

> **Status:** Proposed reference specification  
> **Project:** GOLEM — Go Open Lifecycle & Engineering Manager  
> **Baseline reviewed:** `main` around commit `113c868…` (2026-08-19)  
> **Goal:** evolve GOLEM into an **Active Engineering Graph** / auditable engineering digital twin.  

## Goal

Prove an adapter contract that atomically stores:
- command ID uniqueness;
- events;
- receipt.

## Test cases

- duplicate command same process;
- duplicate concurrent process;
- crash before commit;
- crash after commit before response;
- retry after unknown client outcome;
- conditional stream version conflict.

## Candidate contract

```go
type CommandJournal interface {
    AppendCommand(ctx context.Context, req CommandAppend) (CommandReceipt, error)
}
```

## Exit

No execution path can produce two accepted event sets for the same `(tenant, command_id)`.
