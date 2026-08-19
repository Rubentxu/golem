# Agent Architecture — Grounded and Proposal-first

> **Status:** Proposed reference specification  
> **Project:** GOLEM — Go Open Lifecycle & Engineering Manager  
> **Baseline reviewed:** `main` around commit `113c868…` (2026-08-19)  
> **Goal:** evolve GOLEM into an **Active Engineering Graph** / auditable engineering digital twin.  

## Principle

Agents are consumers of the same engineering model as humans.

## Agent context

An agent receives:

```text
Principal
Tenant
Frame
Lens
Graph query budget
Tool catalog
Policy context
Token/time/cost budget
Correlation/causation
```

## Default action model

```text
Observe
  ↓
Explain
  ↓
Investigate
  ↓
Simulate
  ↓
Propose
  ↓
Policy/Human
  ↓
Command
```

## Grounding

Agent responses SHOULD return:
- referenced graph entity IDs;
- evidence references;
- journal positions/events;
- query/path explanation;
- uncertainty.

## Write safety

LLM tool calls that alter external systems are disabled by default. Instead:
1. agent creates Change Proposal;
2. deterministic validation runs;
3. policy is evaluated;
4. approval obtained when required;
5. command executes via normal application port.

## Memory

Do not put uncontrolled conversational memory into the canonical graph.

Separate:
- canonical facts;
- agent working memory;
- scenario assumptions;
- learned heuristics.

Only promoted, validated facts enter canonical graph.

## Agent behaviors

Use agentic behavior only when deterministic logic is insufficient. Prefer deterministic rules for:
- known policy;
- graph propagation;
- state machines;
- explicit matching.

Use agents for:
- explanation;
- ambiguous classification;
- summarization;
- hypothesis generation;
- remediation proposals.
