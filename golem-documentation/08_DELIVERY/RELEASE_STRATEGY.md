# Release Strategy

- SemVer para APIs, packs y SDKs.
- Trunk + feature flags.
- Artifacts por immutable digest.
- Releases de GOLEM con SBOM + provenance + signature.
- Expand→migrate→contract para schemas/projections.
- Packs con compatibility range y canary activation.
- Nunca editar eventos históricos para “rollback”; usar nuevos eventos/forward fix.
