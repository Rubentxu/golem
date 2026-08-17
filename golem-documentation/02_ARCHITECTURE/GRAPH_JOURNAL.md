# Graph Journal

## Propósito
Historia causal autoritativa. Engineering Graph es su proyección semántica principal.

## Event envelope

```json
{
  "event_id": "01J...",
  "tenant_id": "t_...",
  "stream_id": "artifact:sha256:...",
  "event_type": "artifact.sbom.attached.v1",
  "schema_version": 1,
  "occurred_at": "2026-08-17T12:00:00Z",
  "actor": {"type": "service", "id": "sbom-ingestor"},
  "correlation_id": "01J...",
  "causation_id": "01J...",
  "command_id": "01J...",
  "frame_id": null,
  "payload": {},
  "evidence_refs": []
}
```

## Invariantes
- event_id único;
- payload inmutable;
- schemas versionados;
- actor y tenant obligatorios;
- causation explícita cuando exista;
- no secrets;
- digests antes que URLs mutables;
- event acceptance independiente de sinks externos.

## Snapshots
Optimizaciones verificables con stream position, checksum, schema/projection version. Nunca sustituyen Journal.

## Projections
Engineering Graph, Search, Analytics, Audit, notification intents y release views. Checkpoint + replay idempotente.

## Retention
Audit/release/security evidence: alta retención. Raw logs: object storage + references. High-volume derived facts: policy explícita de compaction.

## Causal Graph
Eventos pueden proyectarse como nodos `Event --CAUSED--> Event --MUTATED--> Entity` sin hacer obligatorio que el Journal físico sea una graph DB.
