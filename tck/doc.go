// Package tck contains the Technology Compatibility Kits for GOLEM ports
// (ADR-046: every critical port owns a conformance TCK).
//
// A TCK is a black-box suite that any adapter of a port must pass, proving
// semantic equivalence between providers and enabling replaceability by
// contract (ADR-052). Adapter conformance tests import this package and run
// it against their real provider (with testcontainers) or a fake.
//
// Planned kits, in implementation order (08_DELIVERY/IMPLEMENTATION_SEQUENCE):
//
//   - GraphStoreTCK: apply/neighborhood semantics, tenant isolation,
//     bounded traversal limits, capabilities negotiation.
//   - JournalStoreTCK: append/idempotency/replay semantics.
//   - EventTransportTCK, ObjectStoreTCK, PolicyEvaluatorTCK, ...
//
// The GraphStoreTCK harness lands together with the first in-memory
// adapter (M1, weeks 3–4) so no kit ships unexercised.
package tck
