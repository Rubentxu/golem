# SPIKE — Incremental Pattern Engine

> **Status:** Proposed reference specification  
> **Project:** GOLEM — Go Open Lifecycle & Engineering Manager  
> **Baseline reviewed:** `main` around commit `113c868…` (2026-08-19)  
> **Goal:** evolve GOLEM into an **Active Engineering Graph** / auditable engineering digital twin.  

## Hypothesis

GraphDelta + dependency signatures can reduce pattern evaluations significantly compared with event-triggered full Lens execution.

## Prototype

Implement:
- parser for existing DSL subset;
- AST;
- schema validation;
- dependency signature;
- portable evaluator;
- in-memory match state;
- MATCHED/UNMATCHED/CHANGED.

## Dataset

Synthetic engineering graph with:
- 100k services/components;
- 1M edges;
- high-degree libraries;
- 1k patterns.

## Scenarios

- one CVE affects one shared component;
- deployment changes runtime edge;
- VEX removes exposure;
- ownership edge changes.

## Compare

A. evaluate all event-subscribed behaviors;
B. delta candidate filtering;
C. native graph query execution where supported.

## Exit

Demonstrate bounded latency and quantify candidate reduction.
