# GOLEM — Go Open Lifecycle & Engineering Manager

> **Control plane de ingeniería de software: graph-native, event-driven y auditable.**

**Idioma del README:** [English](README.md) | Español

[![Go Reference](https://pkg.go.dev/badge/github.com/Rubentxu/golem.svg)](https://pkg.go.dev/github.com/Rubentxu/golem)
[![Go Report Card](https://goreportcard.com/badge/github.com/Rubentxu/golem)](https://goreportcard.com/report/github.com/Rubentxu/golem)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

GOLEM es una plataforma SaaS para gestionar de extremo a extremo el ciclo de
vida de ingeniería de software: intención, requisitos, planificación, código,
revisión, builds, tests, artefactos, dependencias, SBOM, provenance,
vulnerabilidades, releases, despliegues, evidencias, aprobaciones,
arquitectura y operación.

**No** es un clon literal de Tuleap y **no** es otro issue tracker. GOLEM
cambia el sustrato arquitectónico:

- **graph-native** — entidades y relaciones forman un Engineering Graph;
- **event-sourced** — un Graph Journal conserva la historia causal;
- **reactivo** — Behaviors reaccionan a eventos y patrones del grafo;
- **supply-chain-native** — artefactos, SBOM, attestations, firmas, VEX y
  provenance son first-class;
- **agent-ready** — los agentes operan mediante Graph Lenses y Change
  Proposals;
- **provider-independent** — toda dependencia externa vive tras un port con
  TCK de conformidad;
- **SaaS-first** — cells, aislamiento por tenant, cuotas y observabilidad.

```text
The Graph is the Model.      The Journal is the History.
The Behavior is the Reaction. The Evidence is the Proof.
The Port is the Boundary.
```

## Estado

**Fase 0 — bootstrap** (M0 del roadmap). Este repositorio contiene de
momento el esqueleto del módulo, los ports hexagonales y las fitness tests
de arquitectura. La especificación completa vive en
[`golem-documentation/`](golem-documentation/11_RESOURCES/START_HERE.md) —
empieza por ahí.

## Principios no negociables

- El código de dominio nunca importa SDKs de vendors (lo impone
  `internal/archtest`).
- El broker de eventos es transporte, nunca la fuente de verdad.
- Los stores derivados (proyección de grafo, search, analytics) se pueden
  reconstruir desde el Journal.
- Cada port crítico tiene un TCK de conformidad.
- El contexto de tenant es obligatorio de extremo a extremo.
- La seguridad del supply chain (SBOM, provenance, firmas) es core, no un
  añadido.

## Primeros pasos

### Prerrequisitos

- Go 1.26+
- [just](https://github.com/casey/just) — runner de comandos para tareas locales

### Compilar y testear

```sh
git clone https://github.com/Rubentxu/golem.git
cd golem
just check   # fmt-check + vet + test — el gate local es la fuente de verdad
just build   # compila cmd/*
```

### Ejecutar

```sh
just build
./golem-api          # esqueleto del API edge, sirve GET /healthz en :8080
```

## Estructura del proyecto

```text
cmd/{golem-api,golem-worker,golemctl}   composition root (binarios)
internal/domain                         modelo de dominio del kernel
internal/application                    comandos/queries (CQRS)
internal/ports                          contratos de ports hexagonales
internal/archtest                       fitness functions de arquitectura
adapters/                               implementaciones de providers tras ports
tck/                                    kits de conformidad por port
api/                                    contratos de API (fuente: golem-documentation/06_API_SPECS)
schemas/                                event schemas (fuente: golem-documentation/06_API_SPECS)
docs/adr/                               ADRs de decisiones del código
golem-documentation/                    especificación completa de producto y arquitectura
```

Evitar packages globales `common`, `utils` y `models`.

## Flujo de desarrollo

- **Gate local:** `just check` debe pasar antes de cualquier merge. GitHub
  Actions queda reservado al release gate (tag-driven).
- **Cambios arquitectónicos:** ADR → fitness test → docs → código (ver
  [`golem-documentation/10_GOVERNANCE/CONTRIBUTING.md`](golem-documentation/10_GOVERNANCE/CONTRIBUTING.md)).
- **Commits:** Conventional Commits.
- **Añadir un provider:** implementa el port en `adapters/`, mapea los
  tipos del vendor a los canónicos en el boundary y pasa el TCK del port.

## Roadmap

| Hito | Foco | Criterio de salida |
|---|---|---|
| M0 | Discovery & architecture fitness | CI bloquea vendor imports; benchmarks planificados |
| M1 | Kernel (Journal, proyección, replay) | replay reconstruye el grafo; digest reproducible |
| M2 | MVP Work / Requirements / Planning | flujo usable; traza Requirement→Work |
| M3 | Lineage Test / SCM / CI / Artifact | traza Requirement→…→Artifact→Test |
| M4 | Seguridad del supply chain | gate de producción basado en evidence |
| M5 | Extensibilidad e independencia de providers | segunda implementación de ports críticos |
| M6 | Grafo reactivo y escenarios | what-if reproducible |
| M7 | Capa agéntica | escrituras de agentes proposal-only |
| M8 | Escala SaaS y GA | cells, DR, SLOs, revisión de seguridad |

Roadmap completo: [`golem-documentation/08_DELIVERY/ROADMAP.md`](golem-documentation/08_DELIVERY/ROADMAP.md).

## Documentación

- [Empieza aquí](golem-documentation/11_RESOURCES/START_HERE.md)
- [Visión](golem-documentation/00_OVERVIEW/VISION.md)
- [Arquitectura](golem-documentation/02_ARCHITECTURE/ARCHITECTURE.md)
- [Catálogo de ADRs (ADR-001..052)](golem-documentation/07_ADR/ADR-CATALOG.md)
- [Especificaciones de API](golem-documentation/06_API_SPECS/API_GUIDELINES.md)

## Contribuir

Las contribuciones son bienvenidas. Lee primero
[`golem-documentation/10_GOVERNANCE/CONTRIBUTING.md`](golem-documentation/10_GOVERNANCE/CONTRIBUTING.md)
— los cambios de dominio requieren revisión del owner del context, los
cambios de adapter requieren revisión del owner del port, y todo cambio
arquitectónico empieza con un ADR.

## Licencia

Publicado bajo la [Licencia MIT](LICENSE).
