# Ports & Adapters

## Regla
`internal/domain` y `internal/application` no importan SDKs de vendors.

```text
internal/{domain,application,ports}
adapters/{graph,events,search,analytics,policy,object,identity,scm,ci,llm}
```

## Port design
El port representa una capability estable, no la API del vendor.

```go
type GraphStore interface {
    Apply(ctx context.Context, tx GraphMutation) (Revision, error)
    Neighborhood(ctx context.Context, q NeighborhoodQuery) (Subgraph, error)
    Capabilities(ctx context.Context) GraphCapabilities
}
```

## Ports iniciales
JournalStore, GraphStore, EventTransport, ObjectStore, SearchIndex, AnalyticsSink, PolicyEvaluator, IdentityProvider, SecretResolver, RegistryClient, SCMProvider, CIProvider, ArtifactScanner, SBOMGenerator, SignatureVerifier, LLMProvider, EmbeddingProvider, NotificationSink, Clock e IDGenerator.

## DTO rule
Vendor types no cruzan boundaries.

## Composition root
Adapters se seleccionan en bootstrap por Provider Profile; no service locator en domain.
