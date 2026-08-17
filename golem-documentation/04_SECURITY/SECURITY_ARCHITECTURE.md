# Security Architecture

## Trust
No hay confianza implícita entre user/API, service/service, tenant/tenant, plugin/host, GOLEM/provider ni agent/tool.

## Identity
OIDC/OAuth2, service identity de corta vida y workload identity donde exista.

## Authorization
RBAC simple + ABAC/policy contextual por tenant, project, environment, risk, actor type, evidence y action.

## Secrets
References únicamente; resolver en el último momento. Nunca en Journal.

## Supply chain
SBOM, provenance, signatures, pinned digests, attestations y pack signing.

## Fail modes
Auth/policy/signature para privileged operations: fail closed. Search/analytics/notification pueden degradar/reintentar.

## Audit
Graph Journal causal + policy decisions + actor identity.
