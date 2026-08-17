# Deployment Architecture

## Reference cell
Ingress/API, API instances, workers, behavior workers, projection workers, graph provider, Journal provider, event transport, object store, search y analytics.

## Deployment target
Kubernetes es un target natural para SaaS, pero el core no depende de sus APIs. Dev/local puede ejecutar una topología reducida.

## Stateless services
API y workers son stateless salvo provider state/checkpoints.

## Config
Immutable config + Provider Profile + secret references + feature flags + per-cell limits.

## Upgrade
Canary, backward-compatible event consumers, projection versioning, worker drain y migrations con dry-run.
