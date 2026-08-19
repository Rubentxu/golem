# SPIKE — Graph UI Renderer

> **Status:** Proposed reference specification  
> **Project:** GOLEM — Go Open Lifecycle & Engineering Manager  
> **Baseline reviewed:** `main` around commit `113c868…` (2026-08-19)  
> **Goal:** evolve GOLEM into an **Active Engineering Graph** / auditable engineering digital twin.  

## Goal

Validate a graph rendering stack for large, interactive, semantic subgraphs.

## Requirements

- WebGL/WebGPU path;
- incremental nodes/edges;
- clustering;
- semantic zoom;
- custom SVG/icon nodes where useful;
- layout stabilization;
- keyboard selection integration;
- 5k visible elements target without unusable interaction;
- React integration if current frontend uses React.

## Test views

- architecture;
- Path to Production;
- vulnerability blast radius;
- incident causality.

## Decision criteria

- frame time;
- bundle size;
- API ergonomics;
- custom rendering;
- layout quality;
- maintenance/license;
- accessibility fallback strategy.

## Important

The chosen renderer does not define the product information architecture.
