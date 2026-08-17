# Contributing

## Architectural change
Si cambia boundary, source of truth, provider contract, event compatibility, security model o topology: ADR → fitness test → docs → code.

## Ownership
Owners por bounded context y port/TCK.

## Dependency policy
SDKs de vendor sólo dentro de adapters.

## Review
Domain changes requieren owner del context; adapter changes, owner del port.
