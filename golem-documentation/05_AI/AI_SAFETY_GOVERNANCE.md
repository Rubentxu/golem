# AI Safety & Governance

## Guardrails
Proposal-only, tool allowlist, environment permissions, budget, egress policy, secret redaction, evidence refs, deterministic validation y human approval por risk.

## High-risk actions
Production deploy, credentials/policies, deletion, permission grants, release revocation y provider migration requieren policy explícita.

## Prompt injection
Tratar repos/issues/docs como datos no confiables; tools y permissions son independientes del prompt.

## Evaluation record
Agent/behavior version, normalized model/provider, Lens digest, tool calls, proposal, policy result y human outcome.

## Privacy
LLMProvider declara data-handling capabilities; tenant policy puede exigir provider interno/no-retention.
