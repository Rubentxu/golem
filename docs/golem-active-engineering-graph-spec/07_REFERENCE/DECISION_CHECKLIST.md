# Architecture Decision Checklist

> **Status:** Proposed reference specification  
> **Project:** GOLEM — Go Open Lifecycle & Engineering Manager  
> **Baseline reviewed:** `main` around commit `113c868…` (2026-08-19)  
> **Goal:** evolve GOLEM into an **Active Engineering Graph** / auditable engineering digital twin.  

Before accepting a new subsystem or dependency, answer:

## Domain
- Which bounded context owns the concept?
- Is this canonical fact, projection or cache?
- What is the identity strategy?
- What is the provenance?

## Eventing
- Which event records the accepted fact?
- Is replay deterministic?
- What happens to older/newer schema versions?

## Graph
- Which node/edge types change?
- Is direction semantically defined?
- Is the relation temporal?
- How is the query bounded?

## Active behavior
- Is this event-triggered or graph-state-triggered?
- Can it cause loops?
- What are dedupe/cooldown semantics?

## Extension
- Can this be a Capability Pack?
- Which permissions/budgets are required?
- Does it need backend code or only declarations?

## UX
- Which user intent does it satisfy?
- Which entity page owns the experience?
- Is a graph actually the best view?

## SaaS
- What is the tenant cost?
- How does it scale?
- How is noisy-neighbor behavior controlled?

## Security
- Does secret/PII enter the journal?
- Can agent/plugin mutate external systems?
- What evidence is generated?

## Delivery
- Which TCK/test protects the contract?
- Which metric/SLO confirms success?
- What is the migration/rollback path?
