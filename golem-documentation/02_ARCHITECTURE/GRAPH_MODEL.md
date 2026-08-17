# Engineering Graph Model

## Node envelope
`id, tenant_id, kind, revision, attributes, created_at, updated_at, provenance`

## Edge envelope
`id, tenant_id, type, source_id, target_id, revision, attributes, valid_from, valid_to?, provenance`

## Ontología inicial

- Work: Project, WorkItem, Requirement, Milestone, Iteration.
- Source: Repository, Commit, Branch, Tag, Review.
- Build: Pipeline, Build, Job, Builder.
- Artifact: Artifact, Package, ContainerImage, Release.
- Verification: TestCase, TestRun, UATSession, Evidence.
- Security: SBOM, Component, Vulnerability, VEXStatement, Attestation, Signature.
- Runtime: Environment, Deployment, ServiceInstance.
- Architecture: System, Container, Component, ADR.
- Governance: Policy, PolicyDecision, Approval, Principal.

## Relaciones
`IMPLEMENTS`, `VERIFIES`, `DEPENDS_ON`, `CONTAINS`, `BUILT_BY`, `PRODUCED`, `DERIVED_FROM`, `HAS_SBOM`, `ATTESTED_BY`, `SIGNED_BY`, `AFFECTED_BY`, `MITIGATED_BY`, `RELEASED_AS`, `DEPLOYED_TO`, `OWNED_BY`, `APPROVED_BY`, `CAUSED_BY`, `EVIDENCED_BY`.

## Identity
Artifacts por digest. External entities usan GOLEM ID + ExternalIdentity(provider, external_id). Mutables usan ID opaco + revision.

## Temporalidad
No sobrescribir history relevante: validity intervals o add/remove events.

## Query safety
Toda traversal especifica tenant, roots, max depth, max nodes/edges y deadline.
