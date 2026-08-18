// Package canonical provides the canonical graph export and import functionality.
// It produces a portable tar archive (canonical export format v1) containing
// the graph nodes, edges, journal position, and a manifest with SHA-256 checksums.
//
// Pure-stdlib: encoding/json, archive/tar, crypto/sha256.
// No third-party dependencies.
package canonical

import "fmt"

// FormatVersion is the canonical export wire format version.
const FormatVersion = "1"

// ErrUnsupportedFormatVersion is returned when the manifest format_version
// is not "1".
var ErrUnsupportedFormatVersion = fmt.Errorf("canonical: unsupported format version (supported: [\"1\"])")

// OntologySchemaJSON is the embedded JSON-Schema for the domain ontology.
// It documents the closed enumerations for node kinds and edge types.
const OntologySchemaJSON = `{
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
        "Policy", "PolicyDecision", "Approval", "Principal",
        "AgentEval", "AgentRun", "AgentEvalReport"
      ]
    },
    "edge_type_enum": {
      "type": "string",
      "enum": [
        "IMPLEMENTS", "VERIFIES", "DEPENDS_ON", "CONTAINS", "BUILT_BY",
        "PRODUCED", "DERIVED_FROM", "HAS_SBOM", "ATTESTED_BY", "SIGNED_BY",
        "AFFECTED_BY", "MITIGATED_BY", "RELEASED_AS", "DEPLOYED_TO",
        "OWNED_BY", "APPROVED_BY", "CAUSED_BY", "EVIDENCED_BY",
        "EVALUATED", "OBSERVED"
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
}`
