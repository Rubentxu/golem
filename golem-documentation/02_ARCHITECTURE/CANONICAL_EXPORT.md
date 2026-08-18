# Canonical Graph Export — Wire Format v1

## Overview

The canonical export produces a portable, versioned artefact describing the Engineering Graph of a tenant at a given point in time. The artefact is a `tar` archive named `canonical-export-{tenant}-{ISO8601-timestamp}.tar`.

This doc specifies the wire format v1 (`format_version: "1"`). Any change to this format requires an ADR and a version bump — the format is not negotiable once shipped.

## Archive Layout

```
nodes.jsonl            — one ports.Node per line (JSON Lines UTF-8)
edges.jsonl            — one ports.Edge per line (JSON Lines UTF-8)
journal-position.json  — snapshot of the journal head at export time
ontology.schema.json   — JSON-Schema draft-07 with domain kinds and edge types
manifest.json          — checksums, counts, format version, metadata
```

**Events are not exported.** Exporting them would duplicate state and open a new coherence window. The journal position frozen in `journal-position.json` is sufficient for replay-based reconciliation.

## File Specifications

### `nodes.jsonl`

Each line is a valid JSON object representing a `ports.Node`:

```json
{"id":"...","tenant_id":"...","kind":"...","revision":1,"attributes":{...}}
```

- One node per line (JSON Lines / newline-delimited JSON).
- UTF-8 encoded.
- No trailing comma, no JSON array wrapper.
- Field names match `ports.Node` struct tags (`json:"..."`).

### `edges.jsonl`

Each line is a valid JSON object representing a `ports.Edge`:

```json
{"id":"...","tenant_id":"...","kind":"...","source":"...","target":"...","revision":1,"attributes":{...}}
```

- Same encoding rules as `nodes.jsonl`.

### `journal-position.json`

```json
{
  "head": 12345,
  "tenant_id": "t1",
  "captured_at": "2026-08-18T10:30:00Z"
}
```

- `head`: `uint64` position of the journal head at snapshot time.
- `tenant_id`: the tenant whose journal was snapshotted.
- `captured_at`: ISO-8601 timestamp of the snapshot.

### `ontology.schema.json`

JSON-Schema draft-07 inline. Documents the closed enumerations for kinds and edge types in the domain:

```json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "definitions": {
    "kind_enum": {
      "type": "string",
      "enum": [
        "Project", "WorkItem", "Requirement", "Milestone", "Iteration",
        "Repository", "Commit", "Branch", "Tag", "Review",
        "Pipeline", "Build", "Job",
        "Artifact", "Package", "ContainerImage", "Release",
        "TestCase", "TestRun", "UATSession", "Evidence",
        "SBOM", "Component", "Vulnerability", "VEXStatement", "Attestation", "Signature",
        "Environment", "Deployment", "ServiceInstance",
        "System", "Container", "Component", "ADR",
        "Policy", "PolicyDecision", "Approval", "Principal"
      ]
    },
    "edge_type_enum": {
      "type": "string",
      "enum": [
        "IMPLEMENTS", "VERIFIES", "DEPENDS_ON", "CONTAINS", "BUILT_BY",
        "PRODUCED", "DERIVED_FROM", "HAS_SBOM", "ATTESTED_BY", "SIGNED_BY",
        "AFFECTED_BY", "MITIGATED_BY", "RELEASED_AS", "DEPLOYED_TO",
        "OWNED_BY", "APPROVED_BY", "CAUSED_BY", "EVIDENCED_BY"
      ]
    }
  },
  "type": "object",
  "properties": {
    "Node": {
      "type": "object",
      "properties": {
        "id":    { "type": "string" },
        "tenant_id": { "type": "string" },
        "kind":  { "$ref": "#/definitions/kind_enum" },
        "revision": { "type": "integer", "minimum": 1 },
        "attrs": { "type": "object" }
      },
      "required": ["id", "tenant_id", "kind", "revision", "attrs"]
    },
    "Edge": {
      "type": "object",
      "properties": {
        "id":    { "type": "string" },
        "tenant_id": { "type": "string" },
        "kind":  { "$ref": "#/definitions/edge_type_enum" },
        "source": { "type": "string" },
        "target": { "type": "string" },
        "revision": { "type": "integer", "minimum": 1 },
        "attrs": { "type": "object" }
      },
      "required": ["id", "tenant_id", "kind", "source", "target", "revision", "attrs"]
    }
  }
}
```

The schema is informational for M5 (no consumer validates against it yet). It reserves space for M5.1 tooling that needs to understand domain types without importing the kernel.

### `manifest.json`

```json
{
  "format_version": "1",
  "tenant_id": "t1",
  "created_at": "2026-08-18T10:30:00Z",
  "journal_position": {
    "head": 12345,
    "tenant_id": "t1"
  },
  "files": {
    "nodes.jsonl":           "sha256:...",
    "edges.jsonl":           "sha256:...",
    "journal-position.json": "sha256:...",
    "ontology.schema.json":  "sha256:..."
  },
  "counts": {
    "nodes": 100,
    "edges": 243
  },
  "extensions": {}
}
```

- `format_version`: **always `"1"` in M5**. Any other value causes `ErrUnsupportedFormatVersion` at load time.
- `files`: SHA-256 hex digest of each file's raw content, computed after serialization and before tar assembly. The manifest itself is not hashed (it is the载体).
- `counts`: number of top-level JSON objects in each `.jsonl` file (line count).
- `extensions`: reserved for future use without a version bump. Consumers MUST ignore unknown keys in this map.

## `format_version` Policy

| Value | Meaning |
|-------|---------|
| `"1"`  | M5 wire format. All consumers MUST support this. |
| `"2+"` | Future major version. Requires ADR + version bump. NOT forward-compatible. |

The `format_version` field is the **only** versioning mechanism. There is no MIME type, no magic bytes, no filename convention beyond the tar archive naming scheme.

## Producer Rules (Exporter)

1. Take `Journal.Head()` **once** at the start of `Export()` and freeze it in `journal-position.json`.
2. Serialize `nodes.jsonl` and `edges.jsonl` with `json.Encoder` (one object per `Encode()` call, no trailing whitespace beyond the final newline).
3. Compute SHA-256 of each file **after** serialization, **before** writing into the tar.
4. Write files into the tar in the order: `nodes.jsonl`, `edges.jsonl`, `journal-position.json`, `ontology.schema.json`, `manifest.json`.
5. Use `archive/tar` with `tar.TypeReg` for all files (no special permissions, no symlinks).

## Consumer Rules (Reader)

1. Verify `manifest.format_version == "1"`. If not, return `ErrUnsupportedFormatVersion` and do not mutate the target `GraphStore`.
2. Read `journal-position.json` and note the head position for reconciliation.
3. Read `nodes.jsonl` and `edges.jsonl` via `json.Decoder` (line-by-line).
4. Apply nodes/edges to the target `GraphStore` in batches of ≤ 500 operations per mutation (same chunking as `internal/application/projection/projector.go`).
5. Ignore unknown keys in `manifest.extensions` — they may exist from a future version.

## `extensions` Field Forward-Compatibility

The `extensions` map in `manifest.json` is reserved for ad-hoc metadata that does not require a `format_version` bump. Examples for M5.1:

```json
{
  "extensions": {
    "exported_by": "golemctl/0.5.0",
    "capability_pack": "io.golem/provider-hugegraph/v0.1.0"
  }
}
```

The rule: if a field is needed for **correct interpretation** of the graph data, it belongs in the schema. If it is auxiliary metadata that a consumer can safely ignore, it belongs in `extensions`.
