# DevOps Capabilities Inspired by Azure DevOps

> **Status:** Proposed reference specification  
> **Project:** GOLEM — Go Open Lifecycle & Engineering Manager  
> **Baseline reviewed:** `main` around commit `113c868…` (2026-08-19)  
> **Goal:** evolve GOLEM into an **Active Engineering Graph** / auditable engineering digital twin.  

## Cohesive lifecycle

GOLEM should make these feel connected:
- work planning;
- source changes;
- builds;
- testing;
- artifacts;
- releases;
- environments;
- approvals;
- deployments.

The graph provides the shared model instead of provider-local IDs.

## Environments

Model Environment as first-class:
- ownership;
- resources;
- policies;
- deployment history;
- approvals;
- checks;
- risk classification.

## Graph-backed release gates

A release gate evaluates semantic facts:

```text
artifact signed
AND provenance trusted
AND SBOM present
AND no exploitable critical vulnerability
AND required tests passed
AND UAT accepted
AND policy compliant
AND approval present
```

Every condition links to graph evidence.

## Test Plans / UAT

Model:
- TestPlan
- TestSuite
- TestCase
- TestRun
- UATSession
- Evidence
- AcceptanceDecision

Trace:
`Requirement → TestCase → TestRun/UAT → Evidence → Release`.

## Provider abstraction

Azure DevOps, GitHub, GitLab, Jenkins, etc. are integrations producing canonical events, not alternate domain models.
