# Capability Map v2

> **Status:** Proposed reference specification  
> **Project:** GOLEM — Go Open Lifecycle & Engineering Manager  
> **Baseline reviewed:** `main` around commit `113c868…` (2026-08-19)  
> **Goal:** evolve GOLEM into an **Active Engineering Graph** / auditable engineering digital twin.  

```mermaid
mindmap
  root((GOLEM))
    Home
      Attention
      Risks
      Approvals
      Proposals
      Incidents
    Plan
      Portfolio
      Requirements
      Work
      Boards
      Milestones
    Catalog
      Domains
      Systems
      Services
      APIs
      Resources
      Ownership
      Architecture
    Delivery
      SCM
      CI
      Tests
      Artifacts
      Releases
      Environments
      Deployments
    Govern
      SBOM
      Vulnerabilities
      VEX
      Attestations
      Signatures
      Policies
      Evidence
      Audit
    Operate
      Runtime topology
      SLO
      Incidents
      Telemetry links
      Cost
    Automate
      Behaviors
      Patterns
      Agents
      Scenarios
      Proposals
      Blueprints
      Packs
    Explain
      Why
      Impact
      Path
      Evidence
      Cause
      Diff
```

## Capability grouping rule

Navigation groups user intent. Internal bounded contexts do not dictate top-level navigation.

## Cross-cutting capabilities

The following are available from entity context rather than as isolated products:

- Search / omnibox
- Why
- Impact
- Simulate
- Propose Change
- Evidence
- History
- Policies
- Graph Lens
