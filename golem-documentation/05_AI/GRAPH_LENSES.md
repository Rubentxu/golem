# Graph Lenses

## Spec
```yaml
roots: [artifact:sha256:...]
node_types: [Artifact, Component, Vulnerability, Attestation]
edge_types: [CONTAINS, AFFECTED_BY, ATTESTED_BY]
max_depth: 4
max_nodes: 5000
max_edges: 10000
time_window: P90D
evidence: true
```

## Reglas
Read-only, tenant-bound, budgeted, deterministic, serializable e inspectable.

## Families
ReleaseEvidenceLens, VulnerabilityImpactLens, RequirementTraceLens, ArchitectureImpactLens, UATContextLens y AgentChangeLens.
