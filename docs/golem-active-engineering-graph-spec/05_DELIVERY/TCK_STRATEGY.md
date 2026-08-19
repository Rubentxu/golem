# Technology Compatibility Kit Strategy

> **Status:** Proposed reference specification  
> **Project:** GOLEM — Go Open Lifecycle & Engineering Manager  
> **Baseline reviewed:** `main` around commit `113c868…` (2026-08-19)  
> **Goal:** evolve GOLEM into an **Active Engineering Graph** / auditable engineering digital twin.  

## Journal TCK

Must verify:
- append ordering;
- conditional append;
- command idempotency transaction;
- tenant isolation;
- replay;
- head;
- crash consistency expectations.

## Graph portable TCK

Must verify:
- mutation semantics;
- immutable kind/type rules;
- revisions;
- tenant isolation;
- directional traversal;
- query truncation;
- deterministic export.

## Advanced graph TCKs

Separate suites:
- temporal;
- pattern;
- vector;
- distributed traversal;
- algorithms.

## Pack runtime TCK

- permission enforcement;
- budget;
- deterministic clock/random where required;
- no undeclared network;
- activation rollback;
- compatibility negotiation.

## Pattern TCK

Given graph + delta:
- candidate pattern set;
- binding results;
- MATCHED;
- UNMATCHED;
- CHANGED;
- no duplicate transition;
- loop/cooldown semantics.
