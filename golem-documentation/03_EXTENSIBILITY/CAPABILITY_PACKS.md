# Capability Packs

## Contenido
```text
manifest.json
ontology/
schemas/
behaviors/
patterns/
policies/
lenses/
prompts/
ui/
migrations/
docs/
```

## Ejemplos
`tuleap-compatibility`, `supply-chain`, `kubernetes`, `github`, `gitlab`, `safe`, `aspice`, `dora`, `uat`, `architecture`.

## Manifest
Name/version, GOLEM compatibility, capabilities required, node/edge types, entrypoints, permissions, budgets, UI contributions, migrations e integrity digest.

## Distribution
OCI artifact + digest + signature + SBOM/provenance.

## Activation
Resolve → verify → validate → capability check → migrations → activate → `extension.pack.activated`.

## Isolation
Declarative config validada in-process; executable code por WASM o remote plugin.
