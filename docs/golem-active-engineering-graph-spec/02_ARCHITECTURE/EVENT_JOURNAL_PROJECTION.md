# Event Journal and Projection Architecture

> **Status:** Proposed reference specification  
> **Project:** GOLEM — Go Open Lifecycle & Engineering Manager  
> **Baseline reviewed:** `main` around commit `113c868…` (2026-08-19)  
> **Goal:** evolve GOLEM into an **Active Engineering Graph** / auditable engineering digital twin.  

## Correctness invariants

### Journal
- append-only accepted facts;
- tenant-scoped;
- stable event IDs;
- actor, correlation and causation;
- schema version;
- command ID;
- evidence refs where applicable.

### Projection checkpoint
A checkpoint represents **successfully consumed journal position**, not number of graph mutations.

Unknown/no-op events MUST advance checkpoint when decoding/handling succeeded.

### Large projection events
Projection APIs must not expose a path that silently applies only the first mutation chunk.

Preferred contract:

```go
type EventProjector interface {
    Project(ctx context.Context, event RawEvent) ([]GraphMutation, error)
}

type ProjectionRunner interface {
    ApplyEvent(ctx context.Context, event RawEvent) (ProjectionResult, error)
}
```

`ApplyEvent` applies every mutation chunk before reporting success.

## Projection registry

Replace central switches:

```go
registry.Register(eventType, projector)
```

Registration can come from:
- core bounded contexts;
- Capability Packs.

## Transactional command idempotency

Target invariant:

```text
UNIQUE(tenant_id, command_id)
+ journal events
+ receipt
```

must be enforced in one journal transaction boundary where the selected adapter supports it.

`CommandRegistry` becomes a projection/cache, not a second authority.

## Rebuild

A full rebuild:
1. creates empty target projection;
2. replays all events;
3. validates digest/TCK invariants;
4. atomically cuts over;
5. preserves prior projection for rollback window.
