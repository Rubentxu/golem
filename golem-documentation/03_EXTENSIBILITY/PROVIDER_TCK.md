# Provider TCK

## Objetivo
Demostrar sustituibilidad semántica, no sólo compilación.

## GraphStoreTCK
Round-trip nodes/edges, revision conflict, tenant isolation, bounded neighborhood, pagination, edge properties, bulk ingestion, declared transaction semantics, cancellation, retries y capability reporting.

## EventTransportTCK
Publish, redelivery, ordering contract, consumer restart, backpressure, DLQ, tenant partition y metrics.

## ObjectStoreTCK
Put/get, digest verification, range, multipart, conditional write, metadata, delete/retention y tenant scope.

## PolicyTCK
Allow/Deny/ApprovalRequired/EvidenceRequired, reason codes, policy version, timeout y failure mode.

## LLMTCK
Request normalization, structured output contract, timeout/cancel, tool calling normalization, usage/cost metadata, redaction hooks y unsupported capability reporting.

## Merge rule
Un adapter no es “supported” hasta pasar su TCK en CI.
