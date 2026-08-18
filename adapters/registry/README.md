# Command Registry Adapters

This directory groups adapters for **registry-shaped ports**: the
`ports.CommandRegistry` interface defined in ADR-020 (transactional outbox +
idempotent inbox). The port provides at-least-once command delivery by
recording whether a given `command_id` has already been processed.

## Adapters

| Adapter | Description |
|---------|-------------|
| `memstore` | In-memory reference adapter. Dev/CI and single-instance use. |

## What This Is NOT

This is **not** the OCI capability registry (M5.1). no es el OCI capability registry;
that role belongs to the future `adapters/registry/oci/` adapter introduced in M5.1.
The OCI registry will handle `golemctl capability install oci://…` for WASM and OCI
artifact capability packs.

## Forward-Compatibility Note

A future split is planned that will not break existing importers:

```
adapters/
  command-inbox/   ← current adapters/registry/* (CommandRegistry)
  registry/oci/     ← future OCI capability registry client (M5.1)
```

Importers using `adapters/registry/memstore` by path are unaffected by this
split. The package name and public API remain identical.
