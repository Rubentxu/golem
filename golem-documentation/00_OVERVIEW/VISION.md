# GOLEM — Go Open Lifecycle & Engineering Manager

> **Graph-native, event-driven, auditable software engineering control plane.**

**Document baseline:** 0.1  
**Architecture style:** Hexagonal + event-sourced + graph-native + evolutionary  
**Primary implementation language:** Go  
**Deployment target:** Multi-tenant SaaS with cell-based horizontal scaling  


## Tesis

El ciclo de ingeniería es una red temporal y causal, no una colección de tablas:

```text
Requirement
    │ IMPLEMENTED_BY
    ▼
Commit ──BUILT_BY──> Build ──PRODUCED──> Artifact
                                      │
                                      ├─HAS_SBOM──────> SBOM
                                      ├─ATTESTED_BY───> Attestation
                                      └─RELEASED_AS───> Release
                                                           │
                                                           ▼
                                                      Deployment
```

## Misión

> Hacer que cualquier decisión, cambio, artefacto o despliegue pueda comprenderse, verificarse, gobernarse y evolucionarse a partir de un único modelo causal y extensible.

## North Star

Seleccionar cualquier nodo y obtener procedencia, dependencias, impacto, evidence, owners, historia, policies, riesgos, acciones permitidas y explicación reproducible.

## Qué es

- ALM / Engineering Lifecycle Manager.
- Engineering Knowledge Graph operativo.
- Software Supply Chain control plane.
- Plataforma de workflow y políticas.
- Motor de trazabilidad.
- Runtime reactivo basado en eventos y patterns.
- Plataforma de extensiones.
- Sustrato seguro para agentes.

## Qué NO es

- un clon visual de Tuleap;
- otro Jira;
- un data lake sin semántica;
- una graph DB expuesta directamente;
- un framework de agentes genérico;
- un sustituto obligatorio del SCM/CI existente.

## Principio

```text
The Graph is the Model.
The Journal is the History.
The Behavior is the Reaction.
The Evidence is the Proof.
The Port is the Boundary.
```
