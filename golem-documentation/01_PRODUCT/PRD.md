# GOLEM — Go Open Lifecycle & Engineering Manager

> **Graph-native, event-driven, auditable software engineering control plane.**

**Document baseline:** 0.1  
**Architecture style:** Hexagonal + event-sourced + graph-native + evolutionary  
**Primary implementation language:** Go  
**Deployment target:** Multi-tenant SaaS with cell-based horizontal scaling  


# Product Requirements Document

## Problema

El ciclo de ingeniería está fragmentado entre trackers, SCM, CI, test management, registries, scanners, documentación, observabilidad e IAM. GOLEM debe convertir esos hechos dispersos en un **Engineering Graph causal** sin perder las capabilities ALM de uso diario.

## Objetivos

- Lifecycle completo.
- Trazabilidad profunda.
- Supply-chain security integrada.
- SaaS masivo.
- Extensibilidad segura.
- Agent-ready.
- Sustituibilidad real de providers.

## No objetivos iniciales

- reemplazar Git, un registry OCI o Kubernetes;
- ejecutar builds arbitrarios dentro del API process;
- crear un IDE completo;
- active-active global de escritura en v1;
- query arbitraria sin budgets.

## Capabilities

### Portfolio & Planning
Organizations, projects, roadmaps, milestones, backlogs, boards, sprints/Kanban y dependencies.

### Work Management
Tipos configurables, campos, estados, workflows, relations, comments, history, search y permisos.

### Requirements
Hierarchy, baselines, versions, code/test/release links, coverage e impact.

### Test & UAT
Cases, suites, campaigns, manual/automated runs, guided UAT, evidence y human validation.

### SCM & Review
Repositories, commits, branches, tags, reviews y webhook ingestion.

### CI/CD
Pipelines, builds, jobs, outputs, evidence, environments y deployments.

### Artifact Lifecycle
Digest identity, provenance, promotion, signatures, retention y release composition.

### Supply Chain
SPDX/CycloneDX, SLSA/in-toto, vulnerability, VEX, attestations, signatures, blast radius y policies.

### Architecture
Systems, containers, components, ADRs, dependencies, drift e impact.

### Agents
Lenses, Frames, tools, proposal-only default, fork/diff evaluation y evidence.

## NFR

### Performance
Query budgets, pagination, async fan-out, batch ingestion y no sink/LLM I/O en hot path.

### Reliability
Idempotency, inbox/outbox, replay, checkpoints, DLQ, quotas y controlled degradation.

### Security
OIDC, policy, tenant isolation, secret references, signature verification, immutable audit y least privilege.

### Operability
OpenTelemetry, SLOs, metering, backpressure y runbooks.

## Métricas

- tiempo para responder traceability;
- % releases con lineage completo;
- % artifacts con SBOM/provenance;
- % providers críticos R4+;
- P95/P99 por clase;
- projection lag;
- rebuild time;
- agent proposal acceptance/conflict rate.
