# API Guidelines

## Public API
HTTP/JSON resource-oriented, OpenAPI-first, stable opaque IDs.

## Internal RPC
ConnectRPC/gRPC cuando aporte valor; generated types no entran en domain.

## Commands
Idempotency-Key, optional optimistic revision y command receipt con Journal position.

## Queries
Paginadas, tenant-scoped, budgeted y con filtros explícitos.

## Live updates
SSE primero para browser; WebSocket sólo para bidirectional realtime real.

## Errors
Problem Details style con stable `code` y `correlation_id`.

## Versioning
`/api/v1` para breaking public changes. Event major en nombre/schema.
