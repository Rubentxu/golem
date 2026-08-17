# Adapters

Implementaciones de providers tras los ports de `internal/ports`. Layout
previsto (PORTS_ADAPTERS.md): `graph/`, `journal/`, `events/`, `object/`,
`policy/`, `identity/`, `scm/`, `ci/`, `llm/`.

Reglas:

- Los SDKs de vendors sólo viven aquí (ADR-045).
- Los tipos de vendor nunca cruzan el boundary: se mapean a los tipos
  canónicos de `internal/ports` (ADR-047).
- Cada adapter crítico pasa el TCK correspondiente en `tck/` (ADR-046).
- Se seleccionan en el composition root (`cmd/`) por Provider Profile; no
  hay service locator.
