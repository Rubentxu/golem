# Tooling Baseline

## Go
go test, fuzzing, benchmarks, go vet y linting aprobado por el equipo.

## Contracts
OpenAPI 3.1, AsyncAPI 3, JSON Schema 2020-12 y Protobuf sólo donde proceda.

## Tests
Testcontainers para adapters/TCK, property/fuzz y synthetic graph generator.

## Architecture fitness
Crear `golem-archlint` o equivalente: forbidden imports, context cycles, vendor type leakage, event versioning y ADR checks.

## Supply chain
SBOM, provenance, signatures y dependency/security scans para releases de GOLEM.

## Docs
Markdown + Mermaid en el mismo version control.
