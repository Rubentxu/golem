# Product and Engineering Principles

> **Status:** Proposed reference specification  
> **Project:** GOLEM — Go Open Lifecycle & Engineering Manager  
> **Baseline reviewed:** `main` around commit `113c868…` (2026-08-19)  
> **Goal:** evolve GOLEM into an **Active Engineering Graph** / auditable engineering digital twin.  

## P1 — Graph-native, not graph-themed

The graph is the canonical semantic model behind product capabilities. The UI must not reduce this to a decorative node-link viewer.

## P2 — Journal-first auditability

Accepted state-changing facts are journaled. Rebuildable projections are derived. Provenance and causality are preserved.

## P3 — Evidence over assertions

Every important state should be explainable by evidence, source and causal chain.

## P4 — Humans and agents share the same model

Agents consume bounded Lenses and Graph Queries; humans see the same evidence graph. There is no hidden AI-only state model.

## P5 — Agents observe, explain, simulate and propose

Agents do not directly mutate reality. External mutations flow through validated commands, policy checks and approvals.

## P6 — Progressive disclosure

Most users should not need to understand graph theory. Graph views appear when they best answer a question.

## P7 — Entity-first interaction

Important engineering entities have stable pages, actions and context: Service, Release, Artifact, Requirement, Deployment, Component, Vulnerability, Environment, ADR, etc.

## P8 — Open/Closed ecosystem

New domains should arrive through Capability Packs instead of editing kernel switches.

## P9 — Portable core, capability-aware optimization

The graph abstraction supports a mandatory portable contract plus optional advanced capability contracts.

## P10 — Bounded execution everywhere

Queries, traversals, patterns, agents and plugin execution must have explicit budgets.

## P11 — SaaS by design

Multi-tenancy, cell routing, quotas, usage metering, SLOs and noisy-neighbor isolation are first-class.

## P12 — Evolutionary architecture

Start with modular monoliths and independently scalable workers. Extract services only with evidence.
