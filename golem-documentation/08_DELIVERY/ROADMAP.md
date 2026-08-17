# Roadmap evolutivo

> Estimaciones orientativas para un equipo estable de 6–10 ingenieros multidisciplinares. Cada hito se cierra por exit criteria, no por porcentaje de tickets.

## M0 — Discovery & Architecture Fitness (4–6 semanas)
Repo Go, module boundaries, ADR lint, event schema tooling, graph benchmark harness, TCK skeleton, threat model v1, synthetic dataset generator y UX IA.

**Exit:** CI impide vendor imports; event envelope validado; tenant context definido; plan de benchmark aceptado.

## M1 — GOLEM Kernel (6–8 semanas)
Journal port, graph projection, node/edge model, commands/queries, optimistic revision, outbox/inbox, tenant isolation, OTel y API skeleton.

**Exit:** replay reconstruye graph; projection digest reproducible; GraphStoreTCK; isolation suite.

## M2 — Work / Requirements / Planning MVP (8–12 semanas)
Projects, configurable work items, workflow, relations, requirements, milestones/iterations/boards, comments/evidence y search.

**Exit:** flujo usable; Requirement→Work trace; import Tuleap básico.

## M3 — Test / SCM / CI / Artifact lineage (10–14 semanas)
Test/UAT, SCM adapters, CI adapters, Artifact model, build→artifact linkage, release candidate y event sinks.

**Exit:** Requirement→Commit→Build→Artifact→Test trace.

## M4 — Supply Chain Security (8–12 semanas)
SPDX/CycloneDX, SLSA/in-toto, Sigstore verification, vulnerability, VEX, gates y blast radius.

**Exit:** vulnerable component→deployed services; production gate por evidence.

## M5 — Extensibility & Provider Independence (8–10 semanas)
Packs, OCI, WASM host, remote plugins, Provider Profiles, TCK completeness y canonical export.

**Exit:** segunda implementación/simulador para ports críticos y migration rehearsal R4.

## M6 — Reactive Graph & Scenarios (8–12 semanas)
Behaviors, Relation Behaviors, Pattern DSL/compiler, Lenses, Frames, fork/diff/promote y shadow behaviors.

**Exit:** what-if reproducible y behavior v1/v2 diff.

## M7 — Agentic Layer (8–12 semanas)
LLM/tool ports, agent principals, proposals, evaluation harness, UAT/release/security agents y budgets.

**Exit:** no privileged direct writes por defecto; held-out evaluation y audit completo.

## M8 — SaaS Scale & GA (12–16 semanas)
Multi-cell, tenant migration, quotas/metering, DR, hardening, audit/export, SLOs, ops console y security review.

**Exit:** load target, recovery/migration exercises, R4+ estratégico y GA threat-model closure.
