# STRIDE/Data-Flow: Agent Context

**Bounded Context**: Agent (internal/application/agent, internal/domain/agent)
**Wave**: W8.18
**Author**: GOLEM architecture team
**Date**: 2026-08-18
**ADR Reference**: ADR-065, ADR-066, ADR-067, ADR-068

---

## Context Description

The Agent context implements autonomous LLM-powered agents that reason, plan, and execute tools within GOLEM. Agents receive proposals, evaluate them, and execute approved plans. Agent behaviors are constrained by Lens policies and tool allowlists.

## Data Flow

```
[Proposal] ──approve──► [Agent Executor]
    │                         │
    │                         ▼
    │                  [LLM Orchestrator]
    │                         │
    │          ┌──────────────┼──────────────┐
    │          ▼              ▼              ▼
    │     [Tool Policy]  [Lens Filter]  [Redactor]
    │          │              │              │
    │          └──────────────┼──────────────┘
    │                         │
    │                         ▼
    │                  [Tool Execution]
    │                         │
    │                         ▼
    └──────────────────► [Journal Events]
```

## STRIDE Analysis

### S — Spoofing
| Threat | Likelihood | Impact | Mitigation |
|--------|------------|--------|------------|
| Malicious proposal impersonates user | Low | High | Proposals signed; principal verified |
| Agent impersonates another agent | Low | Medium | Agent identity from token |

### T — Tampering
| Threat | Likelihood | Impact | Mitigation |
|--------|------------|--------|------------|
| Proposal tampered after approval | Very Low | Critical | Immutable proposal record in Journal |
| Agent output manipulated | Very Low | High | All agent actions journaled with evidence |

### R — Repudiation
| Threat | Likelihood | Impact | Mitigation |
|--------|------------|--------|------------|
| Agent denies taking action | Very Low | Critical | All actions logged with principal and correlation_id |
| User denies approving proposal | Very Low | Critical | Approval recorded in Journal |

### I — Information Disclosure
| Threat | Likelihood | Impact | Mitigation |
|--------|------------|--------|------------|
| Agent exposes sensitive context in LLM prompt | Medium | High | Lens minimization; prompt templates reviewed |
| Tool results leak sensitive data | Medium | High | Redactor removes secrets from all LLM input/output |
| Agent reveals internal state in response | Low | Medium | Output validation; sanitization |

### D — Denial of Service
| Threat | Likelihood | Impact | Mitigation |
|--------|------------|--------|------------|
| Infinite agent loop exhausts resources | Low | High | Budget limits; deadline enforcement; max iterations |
| Tool spam causes resource exhaustion | Low | Medium | Per-tool rate limits; tool policy enforced |

### E — Elevation of Privilege
| Threat | Likelihood | Impact | Mitigation |
|--------|------------|--------|------------|
| Prompt injection in user input | Medium | Critical | Static templates; Redactor strips injection attempts |
| Agent uses tools beyond allowlist | Very Low | Critical | Tool policy enforced; no dynamic tool loading |
| Agent escalates via privilege inheritance | Very Low | Critical | Agent runs with minimal necessary privileges |

## Mitigations Summary

| Control | Implementation |
|---------|----------------|
| Prompt injection detection | Static templates + Redactor (ADR-066) |
| Tool policy | Explicit allowlist; no dynamic loading |
| Budget limits | Agent budget tracking per proposal |
| Secrets redaction | Redactor on all LLM I/O (ADR-066) |
| Audit trail | agent.* events emitted for all actions |
| Proposal approval | All agent actions require approved proposal |
