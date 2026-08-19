# Security and Governance Architecture

> **Status:** Proposed reference specification  
> **Project:** GOLEM — Go Open Lifecycle & Engineering Manager  
> **Baseline reviewed:** `main` around commit `113c868…` (2026-08-19)  
> **Goal:** evolve GOLEM into an **Active Engineering Graph** / auditable engineering digital twin.  

## Trust boundaries

- user/browser;
- API edge;
- cell runtime;
- plugin/WASM runtime;
- remote plugin;
- source-system connector;
- LLM provider;
- graph/search stores.

## Core controls

### Tenant isolation
Tenant identity is derived from auth/routing, not blindly accepted from arbitrary request headers in production profiles.

### Authorization
Policy evaluation considers:
- principal;
- tenant;
- action;
- entity/resource;
- environment;
- data classification;
- pack permissions.

### Journal
Secrets never enter event payloads. Store references to secret identities.

### Extensions
- signed manifests/packages;
- explicit permissions;
- resource budgets;
- network access denied unless declared;
- migration privileges separate from runtime privileges.

### Agents
- bounded tools;
- content redaction;
- prompt-injection-aware source classification;
- proposal-first writes;
- audit every external tool call.

## Governance evidence

Policy decisions should produce:
- policy version;
- inputs fingerprint;
- result;
- evidence;
- evaluator identity;
- timestamp.

## Supply-chain security for GOLEM itself

Target:
- reproducible build metadata;
- SBOM;
- signed releases;
- provenance attestation;
- dependency review;
- pack signature verification.
