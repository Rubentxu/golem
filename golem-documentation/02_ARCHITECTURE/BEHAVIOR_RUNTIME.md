# Behavior Runtime

## Behavior
`id, version, subscriptions, filters, optional_pattern, lens_spec, execution_policy, budget, handler`

## Kinds
1. Deterministic Behavior.
2. Relation Behavior.
3. Workflow Behavior.
4. Agentic Behavior.

## Activation
`Accepted Event → subscription index → cheap predicates → candidate set → graph pattern → Lens → execute → events/proposals`

## Pattern cost
DSL compilado a AST/plan. Prohibidos por defecto variable-length paths sin max, cross-tenant, full scans no autorizados y mutations desde DSL.

## Relation Behavior
Usar cuando la coordinación pertenece al edge, por ejemplo `Artifact -[CONTAINS]-> Component` ante nueva CVE.

## Determinismo
Clock/random inyectados; external I/O sólo mediante Tool ports; replay distingue cache/reinvocation.

## Failure model
Fallos esperables son events/observable outcomes. Invariantes rotas o inputs inválidos son errors.
