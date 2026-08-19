# SPIKE — Temporal Graph and Scenario Overlays

> **Status:** Proposed reference specification  
> **Project:** GOLEM — Go Open Lifecycle & Engineering Manager  
> **Baseline reviewed:** `main` around commit `113c868…` (2026-08-19)  
> **Goal:** evolve GOLEM into an **Active Engineering Graph** / auditable engineering digital twin.  

## Goal

Compare implementation strategies for `AsOf` and scenario overlay queries.

## Strategies

A. reconstruct from journal/snapshots;
B. validity intervals in graph;
C. native temporal graph support;
D. sparse overlay composition;
E. temporary materialized scenario graph.

## Workloads

- release state 30 days ago;
- compare before/after deployment;
- scenario adds/removes 500 edges;
- impact query over scenario;
- multiple concurrent scenarios.

## Exit

Select default temporal strategy and thresholds for materialization.
