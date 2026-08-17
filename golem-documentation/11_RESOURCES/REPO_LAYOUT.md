# Suggested Repository Layout

```text
golem/
├── cmd/{golem-api,golem-worker,golemctl}
├── internal/
│   ├── domain/
│   ├── application/
│   ├── ports/
│   └── {work,requirements,test,scm,ci,artifact,supplychain,release,behavior,scenario}/
├── adapters/{graph,journal,events,object,policy,identity,scm,ci,llm}/
├── tck/
├── api/{openapi,asyncapi,proto}/
├── schemas/
├── packs/
├── web/
├── testdata/
├── benchmarks/
├── deployments/
└── docs/
```

Evitar packages globales `common`, `utils` y `models`.
