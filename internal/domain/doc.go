// Package domain hosts the kernel domain model of GOLEM.
//
// The Engineering Graph is the canonical domain model (ADR-004) and the
// Graph Journal is its authoritative causal history (ADR-005, ADR-030).
// Bounded contexts (work, requirements, test, scm, ci, artifacts,
// supplychain, release, behavior, scenario, ...) own their entities and
// cross boundaries only through stable IDs, events and contracts — never
// by importing another context's internals (ADR-003).
//
// This package and everything under internal/ is forbidden from importing
// vendor SDKs; that rule is enforced by internal/archtest.
package domain
