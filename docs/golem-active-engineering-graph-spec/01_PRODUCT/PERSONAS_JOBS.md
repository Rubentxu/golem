# Personas and Jobs-to-be-Done

> **Status:** Proposed reference specification  
> **Project:** GOLEM — Go Open Lifecycle & Engineering Manager  
> **Baseline reviewed:** `main` around commit `113c868…` (2026-08-19)  
> **Goal:** evolve GOLEM into an **Active Engineering Graph** / auditable engineering digital twin.  

## Developer

### Jobs
- understand change status;
- debug blocked delivery;
- inspect ownership/dependencies;
- prove that a change reached a target environment.

### Primary surfaces
Service, Repository, Commit, Build, Release, Deployment.

## Platform Engineer

### Jobs
- provide golden paths;
- standardize services;
- enforce platform policy;
- assess blast radius.

### Primary surfaces
Catalog, Blueprints, Policies, Impact, Packs.

## Security Engineer

### Jobs
- identify affected software;
- trace vulnerable component to runtime;
- manage VEX and attestations;
- validate supply-chain provenance.

### Primary surfaces
Component, Vulnerability, Artifact, SBOM, Release, Deployment.

## Architect

### Jobs
- maintain architecture intent;
- identify drift;
- understand decisions and constraints;
- assess architecture change.

### Primary surfaces
System, Container, Component, ADR, Dependency Lens, Scenario.

## QA/UAT

### Jobs
- trace requirement to verification;
- execute guided UAT;
- attach evidence;
- assess release readiness.

## SRE

### Jobs
- connect incidents to recent changes;
- inspect runtime topology;
- identify likely causal candidates;
- understand ownership and rollback options.

## Manager

### Jobs
- identify attention/risk;
- understand delivery flow;
- see bottlenecks without provider-specific dashboards.

## Agent

Agents are modeled as principals with permissions, budgets, tools, frame and provenance. They never receive implicit global graph access.
