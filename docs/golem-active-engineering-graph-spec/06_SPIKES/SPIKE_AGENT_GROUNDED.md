# SPIKE — Grounded Agent Investigation

> **Status:** Proposed reference specification  
> **Project:** GOLEM — Go Open Lifecycle & Engineering Manager  
> **Baseline reviewed:** `main` around commit `113c868…` (2026-08-19)  
> **Goal:** evolve GOLEM into an **Active Engineering Graph** / auditable engineering digital twin.  

## Goal

Validate that agents can answer engineering questions using bounded graph evidence without uncontrolled repository/global context.

## Questions

- Why is release X blocked?
- Which services are impacted by CVE Y?
- What likely caused incident Z?
- Which ADR explains dependency choice Q?

## Contract

Agent receives:
- Lens result;
- graph paths;
- evidence metadata;
- allowed tools;
- budget.

Agent returns:
- answer;
- referenced entity IDs;
- evidence refs;
- uncertainty;
- optional Change Proposal.

## Evaluation

Measure:
- evidence precision;
- unsupported-claim rate;
- cost/latency;
- proposal usefulness;
- prompt-injection resilience.

## Exit

Define production grounding contract and minimum evidence coverage.
