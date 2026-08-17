# Observability

## OpenTelemetry correlation
trace_id, correlation_id, event_id, command_id, cell y adapter. Tenant ID se trata con cuidado en metrics por cardinalidad.

## Trace path
Command → Journal → outbox → projection → behavior → proposal.

## Metrics
Journal append, projection lag, graph query classes, budget exhaustion, behavior queue, adapter errors, tenant throttles, plugin resource usage y migration state.

## Logs
Structured, redacted y sin event payload completo por defecto.

## Operator views
Cell health, lag heatmap, noisy tenants, provider degradation, failed migrations, stuck approvals y DLQ.
