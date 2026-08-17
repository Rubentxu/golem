# Test Strategy

- Domain tests deterministas.
- Application tests con fake ports.
- Provider TCK contra adapters reales.
- OpenAPI/event/plugin contract tests.
- Projection replay → expected graph digest.
- Scenario fork/diff/promote tests.
- Tenant/security matrix y fuzz.
- E2E vertical slices.
- Mixed-load/ingestion tests.
- Property tests para graph invariants, replay, idempotency y non-interference.
- Golden fixtures Tuleap/SBOM/provenance/CI/SCM.
- Clock/random inyectados.
