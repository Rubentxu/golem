# Architecture and Product Risks

> **Status:** Proposed reference specification  
> **Project:** GOLEM — Go Open Lifecycle & Engineering Manager  
> **Baseline reviewed:** `main` around commit `113c868…` (2026-08-19)  
> **Goal:** evolve GOLEM into an **Active Engineering Graph** / auditable engineering digital twin.  

| Risk | Impact | Mitigation |
|---|---|---|
| Graph becomes universal dumping ground | High | Schema Registry + provenance + bounded contexts |
| Pattern engine causes reaction storms | High | dependency index, dedupe, cooldown, max causation depth |
| Event sourcing complexity leaks to UX | Medium | stable entity/query APIs and progressive disclosure |
| Plugin system becomes unsafe | High | signed packs, declarative UI, permissions, sandbox |
| Vendor-neutral graph API blocks performance | High | portable core + advanced capabilities |
| Agent hallucinations pollute canonical facts | High | assertion types + proposal-first writes |
| Scenarios become expensive graph clones | Medium | sparse overlays |
| UI becomes node-link graph-centric | Medium | entity-first/Lens approach |
| SaaS noisy neighbor | High | cell routing + per-tenant budgets |
| Ontology fragments across packs | High | schema governance/namespaces |
| Over-architecture delays usable features | Medium | milestone exit criteria and vertical demos |
