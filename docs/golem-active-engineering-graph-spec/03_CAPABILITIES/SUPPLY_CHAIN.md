# Software Supply Chain and Artifact Lifecycle

> **Status:** Proposed reference specification  
> **Project:** GOLEM — Go Open Lifecycle & Engineering Manager  
> **Baseline reviewed:** `main` around commit `113c868…` (2026-08-19)  
> **Goal:** evolve GOLEM into an **Active Engineering Graph** / auditable engineering digital twin.  

## Objective

Make artifact identity, origin and lifecycle traceable end to end.

## Core entities

- Artifact
- Package
- ContainerImage
- SBOM
- Component
- Vulnerability
- VEXStatement
- Attestation
- Signature
- Provenance
- Release
- Deployment

## Identity

Artifacts are content-addressed by digest whenever possible.

## Core lineage

```text
Commit
 → Build
 → Artifact
 → SBOM
 → Components
 → Vulnerabilities

Artifact
 → Attestation
 → Signature
 → Release
 → Deployment
```

## Artifact lifecycle

Track states such as:
- produced;
- scanned;
- attested;
- signed;
- promoted;
- deployed;
- deprecated;
- revoked.

Do not overwrite important lifecycle history; represent facts/events/validity.

## Reactive examples

- new CVE changes blast radius;
- revoked signature invalidates release readiness;
- missing provenance blocks promotion;
- new VEX changes exploitability;
- artifact deployed to prohibited environment triggers policy violation.

## Evidence

Every release gate must be able to show exactly which artifact digest and evidence satisfied the condition.
