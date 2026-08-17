# GOLEM — Go Open Lifecycle & Engineering Manager

> **Graph-native, event-driven, auditable software engineering control plane.**

**Document baseline:** 0.1  
**Architecture style:** Hexagonal + event-sourced + graph-native + evolutionary  
**Primary implementation language:** Go  
**Deployment target:** Multi-tenant SaaS with cell-based horizontal scaling  


## Resumen ejecutivo

GOLEM es una plataforma SaaS para gestionar de extremo a extremo el ciclo de vida de ingeniería: intención, requisitos, planificación, código, revisión, builds, tests, artefactos, dependencias, SBOM, provenance, vulnerabilidades, releases, despliegues, evidencias, aprobaciones, arquitectura y operación.

No es una copia literal de Tuleap. Tuleap aporta una referencia funcional importante —trackers, planificación, colaboración, test management, integración SCM/CI, trazabilidad y gobierno—, pero GOLEM cambia el sustrato arquitectónico:

- **graph-native**: entidades y relaciones forman un Engineering Graph;
- **event-sourced**: Graph Journal conserva historia causal;
- **reactivo**: Behaviors reaccionan a eventos y patrones;
- **supply-chain-native**: artifacts, SBOM, attestations, firmas, VEX y provenance son first-class;
- **agent-ready**: agentes operan mediante Graph Lenses y Change Proposals;
- **provider-independent**: infraestructura tras ports + adapters + TCK;
- **SaaS-first**: cells, aislamiento tenant, cuotas y observabilidad.

## Diferenciadores

1. Software Engineering Digital Twin.
2. Causalidad nativa.
3. Source→build→artifact→release→deployment lineage.
4. Fork / Diff / Promote.
5. Replaceability by contract.
6. Agentic governance.
7. Arquitectura evolutiva.
