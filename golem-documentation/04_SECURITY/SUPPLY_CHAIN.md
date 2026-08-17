# Software Supply Chain Security

## Cadena
`SourceRevision → Build → Artifact(digest) → SBOM → Provenance → Signature → Release → Deployment`

## Standards
SPDX y CycloneDX; SLSA/in-toto; Sigstore/Cosign como adapter de referencia; OCI para distribución.

## Entities
Artifact, Component, SBOM, Attestation, Signature, Builder, Build, Vulnerability, VEXStatement, Release y Deployment.

## Ingest
Observe artifact → parse/normalize SBOM → component identity → attach provenance → verify signatures → ingest vulnerability facts → affected paths → policy.

## Policy examples
- deny si falta provenance;
- require verified signature en production;
- require SBOM;
- block critical exploitable vulnerability;
- allow con signed VEX `not_affected` aceptado.

## Evidence
Conservar policy version, input digests, signature result, SBOM/provenance digest, feed version y reason.
