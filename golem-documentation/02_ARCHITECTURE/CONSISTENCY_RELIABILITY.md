# Consistencia y fiabilidad

## Modelo
Single authoritative write + async projections.

## Command
Auth/policy → load relevant state → invariant/revision → Journal append → receipt con position.

## Projection
Eventual consistency, checkpoint y lag. `min_position` opcional para read-your-write.

## Idempotency
Public writes aceptan Idempotency-Key.

## Inbox/outbox
Outbox desde commit autoritativo, retry dispatcher, consumer dedup/inbox y DLQ con evidence.

## Retry
Backoff+jitter sólo para transient errors.

## Concurrency
Optimistic revision. Proposal con revision obsoleta ⇒ CONFLICTED.

## Recovery
Journal → snapshot/checkpoint → replay. Derived stores reconstruibles.
