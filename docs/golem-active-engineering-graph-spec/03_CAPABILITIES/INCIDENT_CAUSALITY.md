# Incident Causality and Change Intelligence

> **Status:** Proposed reference specification  
> **Project:** GOLEM — Go Open Lifecycle & Engineering Manager  
> **Baseline reviewed:** `main` around commit `113c868…` (2026-08-19)  
> **Goal:** evolve GOLEM into an **Active Engineering Graph** / auditable engineering digital twin.  

## Goal

Use the temporal graph to rank evidence-backed causal candidates.

## Inputs

- Incident;
- alerts;
- SLO changes;
- deployments;
- releases;
- commits;
- runtime topology;
- ownership;
- dependency graph.

## Candidate path

```text
Incident
 ← correlated-with
Telemetry Change
 ← after
Deployment
 ← deploys
Release
 ← contains
Artifact
 ← derived-from
Commit
```

## Deterministic layer

Generate candidate set using:
- temporal window;
- topology reachability;
- changed components;
- service ownership;
- known dependency direction.

## Agentic layer

Agent may summarize/rank hypotheses, but must cite candidate paths and uncertainty.

## Outputs

- hypothesis;
- confidence;
- supporting paths;
- contradicting evidence;
- recommended investigation;
- rollback/change proposal.

## Safety

Never label a change as causal solely because it is the latest deployment.
