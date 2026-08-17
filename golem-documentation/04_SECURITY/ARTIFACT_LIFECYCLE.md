# Artifact Lifecycle

```mermaid
stateDiagram-v2
  [*] --> Observed
  Observed --> Built
  Built --> Verified
  Verified --> Candidate
  Candidate --> Released
  Released --> Promoted
  Promoted --> Deployed
  Deployed --> Retired
  Built --> Quarantined
  Verified --> Quarantined
  Released --> Revoked
```

## Identity
`ArtifactID = algorithm + digest`; tags/names son aliases.

## Relations
PRODUCED_BY, HAS_SBOM, ATTESTED_BY, SIGNED_BY, CONTAINS, PART_OF y DEPLOYED_AS.

## Promotion
No cambia identity: cambia eligibility/location/state y genera evidence.

## Revocation
Evento + reason/evidence + recalculation de releases/deployments + notifications/policies.
