# Quality Gates

Un PR no mergea si falla:
1. go test;
2. vet/lint;
3. architecture import rules;
4. schema compatibility;
5. event naming/version;
6. affected TCK;
7. tenant isolation si toca storage/query;
8. security scanning;
9. SBOM generation;
10. release provenance/signature;
11. migration dry-run si aplica;
12. docs/ADR check.

Cambios de graph/Journal/hot path ejecutan benchmark comparativo.
