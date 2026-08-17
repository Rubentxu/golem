# Migration from Tuleap

## Filosofía
Migrar semántica, no tablas internas.

## Pipeline
`Tuleap API/export → staging canonical records → identity mapping → validate → graph import → relation reconstruction → reconciliation → delta sync → cutover`

## Mapping inicial
Project→Project; Tracker→WorkType; tracker item→WorkItem; Field→FieldDefinition; Workflow→WorkflowDefinition; Milestone/Sprint→Milestone/Iteration; tests→Test context; Git links→SCM relations; users/groups→Principal/Group.

## Migration provenance
Source=tuleap, external id, source project, import batch, timestamp y checksum cuando exista.

## Reconciliation
Counts, missing relations, unsupported customizations y manual actions antes de cutover.
