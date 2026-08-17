# Technology Stack Baseline

## Kernel/backend
- Go stable soportado por el proyecto.
- net/http + router ligero/generated handlers.
- ConnectRPC/gRPC opcional para RPC interno.
- OpenAPI 3.1, AsyncAPI 3, JSON Schema 2020-12.
- Protobuf sólo en RPC boundary.

## Reference providers

| Capability | Reference/candidate | Estado |
|---|---|---|
| GraphStore | Apache HugeGraph | spike-gated |
| Graph alternative | NebulaGraph | benchmark |
| EventTransport | NATS JetStream | reference |
| ObjectStore | S3-compatible | reference |
| Search | OpenSearch | derived |
| Analytics | ClickHouse | derived |
| Policy | OPA | reference |
| Identity | OIDC | protocol |
| Observability | OpenTelemetry | contract |
| Pack distribution | OCI registry | standard |
| WASM runtime | wazero | candidate |
| Signing | Sigstore/Cosign | reference |

## Web
TypeScript + React como baseline pragmática. Graph renderer WebGL elegido por spike. UI es adapter y no accede a providers.

## Por qué no “todo Go”
Go es el kernel y backend; forzar Go/WASM en una UI rica no aporta suficiente valor y no mejora los boundaries.
