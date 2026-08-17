# Bounded Contexts

| Context | Responsabilidad | Entidades |
|---|---|---|
| Identity & Tenancy | tenant/org/membership | Tenant, Organization, Principal |
| Projects | espacios/config | Project, Template |
| Work | work items/workflow | WorkItem, WorkType, Workflow |
| Planning | backlog/sprint/board | Plan, Iteration, Board, Milestone |
| Requirements | requisitos/baselines | Requirement, Baseline |
| Test | cases/campaigns/UAT | TestCase, Campaign, TestRun, Evidence |
| SCM | repos/commit/review | Repository, Commit, Review |
| CI | pipelines/builds/jobs | Pipeline, Build, Job |
| Artifacts | identity/lifecycle | Artifact, Promotion |
| SupplyChain | SBOM/provenance/vuln/VEX | SBOM, Component, Attestation, Vulnerability |
| Release | composition/gates | Release, ReleaseCandidate, Gate |
| Deployment | env/deployment | Environment, Deployment |
| Architecture | systems/components/ADRs | System, Component, ArchitectureDecision |
| Policy | evaluations/approvals | PolicyDecision, Approval |
| Journal | events/causality | Event, StreamPosition |
| Behavior | reactions/frames | BehaviorDefinition, ExecutionFrame |
| Scenario | fork/diff/promote | Scenario, ScenarioDelta |
| Extension | packs/providers | CapabilityPack, ProviderProfile |

## Regla
Cruces entre contexts mediante IDs/refs estables, events y contracts; no imports de entidades internas de otro context.
