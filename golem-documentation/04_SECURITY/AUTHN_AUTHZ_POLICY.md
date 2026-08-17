# Authentication, Authorization & Policy

## Principal
`subject, type(human|service|agent), tenant memberships, groups, claims, assurance`.

## Authorization layers
Route capability → bounded-context rule → object/project scope → policy para privileged action.

## Canonical policy result
`ALLOW`, `DENY(reason)`, `APPROVAL_REQUIRED`, `ADDITIONAL_EVIDENCE_REQUIRED`.

## Agents
Actor type habilita policies específicas: proposal-only, no production direct write, max cost, allowed tools y human approval.

## Portability
Core depende de `PolicyEvaluator`; OPA es reference adapter.
