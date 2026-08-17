# Risk Register

| Riesgo | Impacto | Prob. | Mitigación |
|---|---|---|---|
| Graph DB no escala | Alto | Media | SP-001 + TCK + migration |
| Event sourcing complejidad | Alto | Media | vertical slice + tooling |
| Scope ALM completo | Alto | Alta | milestones por exit criteria |
| Multi-tenancy leak | Crítico | Media | TenantContext + fuzz/TCK |
| Abstracción lowest-common-denominator | Alto | Media | capability negotiation |
| Graph UX no escala | Medio | Media | neighborhood + SP-011 |
| Agent action peligrosa | Crítico | Media | proposal-only + policy |
| Event schema drift | Alto | Media | compatibility CI |
| Projection lag | Alto | Alta | backpressure/partition/autoscale |
| Tuleap custom migration | Alto | Alta | staging + reconciliation |
| Supply-chain false positives | Medio | Alta | VEX + evidence |
| Malicious Pack | Crítico | Media | signing + sandbox |
| Premature microservices | Alto | Media | modular-first |
| Vendor lock-in | Alto | Alta | R-level metric |
