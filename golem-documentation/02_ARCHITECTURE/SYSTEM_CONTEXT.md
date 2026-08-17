# System Context

```mermaid
flowchart LR
  U[Users]
  A[AI Agents]
  G[GOLEM]
  SCM[SCM Providers]
  CI[CI/CD Systems]
  REG[Artifact/OCI Registries]
  VULN[Vulnerability Sources]
  IDP[OIDC IdP]
  OBS[Observability/SIEM]
  CHAT[Notifications]
  CLOUD[Cloud/Kubernetes]

  U --> G
  A --> G
  SCM <--> G
  CI <--> G
  REG <--> G
  VULN --> G
  IDP --> G
  G --> OBS
  G --> CHAT
  CLOUD <--> G
```

## Trust boundaries
Internet/API edge, tenant, cell, plugin/WASM, external provider, privileged administration y artifact/evidence.

## Integration philosophy
GOLEM ingiere facts y emite commands/events mediante adapters. Sistemas externos conservan ownership de repos/blobs/runtime; GOLEM mantiene identity, relations, evidence y lifecycle.
