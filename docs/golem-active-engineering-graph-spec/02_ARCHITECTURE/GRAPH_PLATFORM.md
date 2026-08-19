# Engineering Graph Platform

> **Status:** Proposed reference specification  
> **Project:** GOLEM — Go Open Lifecycle & Engineering Manager  
> **Baseline reviewed:** `main` around commit `113c868…` (2026-08-19)  
> **Goal:** evolve GOLEM into an **Active Engineering Graph** / auditable engineering digital twin.  

## Core graph model

The graph is a semantic projection, not the journal itself.

### Node envelope

```text
id
tenant_id
kind
revision
attributes
created_at
updated_at
provenance
```

### Edge envelope

```text
id
tenant_id
type
source_id
target_id
revision
attributes
valid_from
valid_to?
provenance
```

## Interface segregation

```go
type GraphWriter interface { Apply(...) }
type GraphReader interface { GetNode(...); Neighborhood(...) }
type PathFinder interface { Paths(...) }
type PatternMatcher interface { Match(...) }
type TemporalReader interface { AsOf(...); Between(...) }
type GraphDiffReader interface { Diff(...) }
type GraphExporter interface { Stream(...) }
```

## Direction

Traversal semantics must support:
- OUT;
- IN;
- BOTH.

`BOTH` is explicit, never implicit.

## Portable vs advanced capabilities

### Mandatory portable contract
- upsert/remove node;
- upsert/remove edge;
- point read;
- bounded neighborhood;
- bounded typed traversal.

### Optional capability contracts
- native pattern execution;
- temporal indexes;
- shortest/all paths;
- distributed traversal;
- graph algorithms;
- vector + graph;
- native change streams.

Adapters publish capabilities and pass capability-specific TCKs.

## Query budgets

Every traversal/pattern query includes:
- tenant;
- roots or indexed predicates;
- max depth;
- max nodes;
- max edges;
- timeout;
- optional estimated-cost ceiling.
