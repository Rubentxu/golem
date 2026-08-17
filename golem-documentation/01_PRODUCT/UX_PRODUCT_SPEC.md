# UX Product Spec

## Shell

```text
┌──────────────────────────────────────────────────────────────┐
│ Global Search / Command Palette / Project / Environment      │
├──────────────┬──────────────────────────────┬────────────────┤
│ Navigator    │ Workspace                    │ Inspector      │
│ Projects     │ Board/Table/Graph/Timeline   │ Properties     │
│ Work         │ Architecture/Supply Chain    │ Relations      │
│ Releases     │ Tests/UAT/Scenario Diff      │ Evidence       │
│ Security     │                              │ History/Policy │
└──────────────┴──────────────────────────────┴────────────────┘
```

## First-class interactions

- omnibox por entity/digest/CVE/release/commit;
- graph neighborhood con semantic zoom;
- inspector Overview/Relations/Evidence/History/Policies;
- “Why?” sobre states y decisions;
- “Impact” sobre cualquier entity;
- “Scenario” desde contexto actual;
- “Propose change”;
- saved Lenses;
- split views.

## Graph UX

WebGL, subgraphs paginados, clustering, layout incremental y nunca cargar el tenant graph completo.

## Accessibility

Keyboard, alternative tree/table, WCAG AA, focus visible y estados no dependientes sólo de color.
