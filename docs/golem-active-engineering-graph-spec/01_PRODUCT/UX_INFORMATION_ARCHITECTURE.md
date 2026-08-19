# UX — Information Architecture and Shell

> **Status:** Proposed reference specification  
> **Project:** GOLEM — Go Open Lifecycle & Engineering Manager  
> **Baseline reviewed:** `main` around commit `113c868…` (2026-08-19)  
> **Goal:** evolve GOLEM into an **Active Engineering Graph** / auditable engineering digital twin.  

## Navigation

```text
Home
Plan
Catalog
Delivery
Operate
Govern
Automate
```

Avoid top-level items named after low-level technologies or view types.

## Application shell

```text
┌─────────────────────────────────────────────────────────────────────┐
│ Global Search / Command Palette / Scope / Environment / Time       │
├───────────────┬─────────────────────────────────┬───────────────────┤
│ Navigation    │ Workspace                       │ Context Inspector │
│               │                                 │                   │
│ Home          │ Entity / Board / Timeline       │ Overview          │
│ Plan          │ Lens / Diff / Scenario          │ Relations         │
│ Catalog       │                                 │ Evidence          │
│ Delivery      │                                 │ History           │
│ Operate       │                                 │ Policies          │
│ Govern        │                                 │ Provenance        │
│ Automate      │                                 │                   │
└───────────────┴─────────────────────────────────┴───────────────────┘
```

## Global command palette

Search by:

- name/id;
- digest;
- commit SHA;
- CVE;
- pURL;
- release;
- environment;
- service;
- owner/team;
- ADR.

Commands include:

- Open entity
- Why?
- Impact
- Simulate change
- Propose change
- Switch Lens
- Jump to time
- Compare

## UX rule: views are projections

Users choose a question or Lens; GOLEM chooses an appropriate visual projection. Node-link graphs are never the default for every problem.

## Home / Attention model

Home ranks actionable items:

- blocked releases;
- new critical exposures;
- pending approvals;
- failing policies;
- scenario proposals awaiting review;
- architecture drift;
- SLO risk;
- incident hypotheses.

Each card must answer **why it is here** and link to evidence.
