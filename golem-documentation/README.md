# GOLEM — Go Open Lifecycle & Engineering Manager

> **Graph-native, event-driven, auditable software engineering control plane.**

**Document baseline:** 0.1  
**Architecture style:** Hexagonal + event-sourced + graph-native + evolutionary  
**Primary implementation language:** Go  
**Deployment target:** Multi-tenant SaaS with cell-based horizontal scaling  


## Qué contiene este paquete

Este repositorio documental es la especificación inicial de implementación de **GOLEM**. Consolida las decisiones refinadas durante la concepción del producto: reimaginar las capacidades de Tuleap en una plataforma moderna escrita principalmente en Go, modelar el ciclo de vida de ingeniería como un grafo, tratar la trazabilidad y la cadena de suministro como capacidades nativas, y permitir que cualquier dependencia externa sea sustituible.

La idea central es:

```text
Engineering Graph = modelo canónico del mundo de ingeniería
Graph Journal      = historia causal inmutable
Behaviors          = reacciones deterministas o agentic
Evidence           = prueba verificable
Ports              = fronteras anti-lock-in
```

## Empieza por aquí

1. [`11_RESOURCES/START_HERE.md`](11_RESOURCES/START_HERE.md)
2. [`00_OVERVIEW/VISION.md`](00_OVERVIEW/VISION.md)
3. [`01_PRODUCT/PRD.md`](01_PRODUCT/PRD.md)
4. [`02_ARCHITECTURE/ARCHITECTURE.md`](02_ARCHITECTURE/ARCHITECTURE.md)
5. [`07_ADR/README.md`](07_ADR/README.md)
6. [`08_DELIVERY/ROADMAP.md`](08_DELIVERY/ROADMAP.md)
7. [`08_DELIVERY/IMPLEMENTATION_SEQUENCE.md`](08_DELIVERY/IMPLEMENTATION_SEQUENCE.md)

## Principios que no se negocian

- El dominio no importa SDKs de vendors.
- El broker de eventos no es la fuente de verdad.
- Toda acción sensible es trazable y, para agentes, propuesta antes de aplicada.
- Los read models derivados pueden reconstruirse.
- Cada adapter crítico dispone de un contrato y TCK de conformidad.
- El modelo de dominio es graph-native aunque la persistencia física pueda evolucionar.
- Seguridad de supply chain, SBOM, provenance y lifecycle de artefactos son core.
- La escalabilidad SaaS se consigue por cells y particionado.
- La arquitectura se valida con fitness functions automáticas.

## Estado

Esta baseline está preparada para iniciar spikes y kernel. La graph DB definitiva y la estrategia multi-región de escritura quedan **spike-gated**.
