# Data Lifecycle

| Clase | Ejemplos | Política |
|---|---|---|
| Journal audit-critical | release/approval/provenance | alta retención/legal hold |
| Engineering entities | work/requirements | tenant lifecycle |
| Artifact metadata | digest/SBOM refs | artifact lifecycle |
| Large evidence | logs/screenshots | object storage |
| Search/analytics | derived | rebuildable |
| Secrets | tokens/keys | nunca Journal |
| Agent artifacts | structured outputs | retention/redaction policy |

## Deletion
Freeze → optional export → provider deletion → verify → minimal operational tombstone.

## Legal Hold
Entidad/policy que bloquea retention para evidence/artifacts seleccionados.

## Content addressing
Digest es identidad/integridad; URL es localizador.
