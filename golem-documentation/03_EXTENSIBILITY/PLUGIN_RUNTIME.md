# Plugin Runtime

## WASM
Para lógica acotada: sin filesystem/network por defecto, host functions explícitas, time/memory/fuel limits y runtime Go candidato `wazero`.

## Remote
Para integraciones pesadas: ConnectRPC/gRPC, mTLS, service identity, timeout, circuit breaker y version negotiation.

## Prohibido
Cargar Go `.so` arbitrarios en el proceso SaaS.

## Permissions
`graph.read:lens`, `proposal.write`, `object.read`, `scm.read`, `ci.trigger`, egress explícito, etc.

## Observabilidad
Plugin id/version, frame, trace, duration, resource usage, result/error y proposals/events.
