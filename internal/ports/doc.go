// Package ports defines the hexagonal boundary contracts of GOLEM.
//
// A port expresses a stable capability, never a vendor API (ADR-002,
// ADR-045). Domain and application code depend only on the types and
// interfaces in this tree; concrete providers live under adapters/ and are
// wired at the composition root (cmd/). Vendor data types never cross
// adapter boundaries (ADR-047).
//
// Every critical port owns a conformance TCK under tck/ (ADR-046).
package ports
