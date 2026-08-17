# Event Model

## Naming
`<context>.<entity>.<verb>.v<major>`

Ejemplos:
`work.item.created.v1`, `scm.commit.observed.v1`, `ci.build.completed.v1`, `supplychain.sbom.ingested.v1`, `scenario.promoted.v1`.

## Compatibility
Cambios aditivos dentro de major; eliminación/cambio semántico ⇒ nueva major. Consumer compatibility CI.

## Idempotencia externa
Provider + external_event_id/content fingerprint + observed_at + dedup key.

## Ordering
No orden global. Contrato explícito per stream/aggregate/provider partition y causal links.

## Delivery
Broker puede redeliver; consumers son idempotentes.
