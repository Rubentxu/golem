# Agentic Architecture

## Principio
Los agentes son actores sujetos a los mismos contracts que humanos y automatizaciones.

## Flow
`Goal → Execution Frame → Graph Lens → Agent/Model → Tools → Change Proposal → Policy → Approval? → Apply → Journal`

## Agent roles
Investigator, release reviewer, security analyst, architecture analyst, UAT author, documentation assistant y remediation proposer.

## Tool boundary
Tools son ports normalizados; el modelo no recibe SDK credentials.

## Persistence
Conservar proposals, evidence, structured findings, model metadata y template/version digest según policy. No almacenar chain-of-thought privada; sí rationale estructurado suficiente.

## Evaluation
Fixtures, held-out scenarios, fork/diff, cost/latency, policy violation rate y proposal quality.
