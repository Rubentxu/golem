# SLO Baseline

| Capability | SLI |
|---|---|
| Command API | success + P95/P99 acceptance latency |
| Query API | success + latency por query class |
| Journal | append success/latency |
| Projection | lag seconds/events |
| Event transport | backlog age/redelivery |
| Graph | query latency/timeout |
| Ingestion | events/sec + checkpoint freshness |
| Agent | success/cost/latency |
| Provider | availability/error budget |

Cada capability tiene degradation mode propio.
