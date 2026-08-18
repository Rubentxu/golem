# Runbook M5 — Demo Walkthrough GOLEM

> **Objetivo**: Ejecutar de forma manual los 5 escenarios de smoke test para el ciclo M5-extensibility.
> **Entorno**: var/home/rubentxu/Proyectos/golang/golem (raíz del repo)
> **Usuario**: operador local con acceso de lectura/escritura al filesystem

## Precondiciones

- Binario `golem-api` compilado: `go build -o golem-api ./cmd/golem-api`
- Puerto 8080 libre
- Permisos de escritura en `/tmp/golem.db`

---

## Escenario 1 — Perfil por defecto (dev)

```bash
# Sin variable de entorno → perfil dev
unset GOLEM_PROFILE
./golem-api &
API_PID=$!
sleep 2

# Verificar en los logs la línea profile=dev
# (buscar la cadena "profile=dev" en stderr o stdout)
kill $API_PID 2>/dev/null
wait $API_PID 2>/dev/null
```

**Esperado**: en la salida de logs aparece `profile=dev`.

---

## Escenario 2 — Perfil durable con journal bbolt

```bash
# Perfil durable → journal=bbolt, path=/tmp/golem.db
GOLEM_PROFILE=durable ./golem-api &
API_PID=$!
sleep 2

# Verificar en los logs:
#   profile=durable
#   journal=bbolt
#   path=/tmp/golem.db
kill $API_PID 2>/dev/null
wait $API_PID 2>/dev/null
```

**Esperado**: los tres valores aparecen en los logs.

---

## Escenario 3 — Health check

```bash
# Iniciar el servidor en segundo plano
./golem-api &
API_PID=$!
sleep 2

# curl healthz
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/healthz)

echo "HTTP status: $HTTP_CODE"

kill $API_PID 2>/dev/null
wait $API_PID 2>/dev/null
```

**Esperado**: `HTTP_CODE=200`.

---

## Escenario 4 — R4 E2E (100 nodos, seed determinista)

```bash
# Ejecutar el test E2E con observación reducida (100 ms)
export GOLEM_MIGRATION_OBSERVE_WINDOW=100ms

go test -race -v -run TestR4 ./tck/...

unset GOLEM_MIGRATION_OBSERVE_WINDOW
```

**Esperado**:

- `TestR4HappyPath` → PASS
  - 4 eventos en orden: `started`, `diffed`, `cutover`, `completed`
  - `runtime.Graph` apunta a target tras `SwapGraph`
- `TestR4StructuralDivergence` → PASS (3 subtests)
  - `extra_node_in_target`: CutoverSafe=false, CountsMatch=false
  - `different_node_counts`: CutoverSafe=false, CountsMatch=false
  - `identical_graphs`: CutoverSafe=true, NodeDiffs=0, EdgeDiffs=0

---

## Escenario 5 — Journal replay (5 eventos migration.harness.*)

```bash
# El test R4HappyPath ya populó el journal.
# Verificar que hay exactamente 5 eventos en la secuencia harness.

# Opción A: mediante el test (comprobaciónprogramática)
go test -race -v -run TestR4HappyPath ./tck/... 2>&1 | grep -E "migration harness|audit event"

# Opción B: verificar directamente el journal via código
# (requiere un pequeño script o use del paquete journalmem)
#
# Los 5 eventos esperados en orden:
#   1. migration.harness.started.v1
#   2. migration.harness.diffed.v1
#   3. migration.harness.cutover.v1
#   4. migration.harness.completed.v1
#
# No debe aparecer migration.harness.rolled_back.v1
```

**Esperado**: 4 eventos (started → diffed → cutover → completed), en orden, sin rollback.

---

## Checklist final

| Escenario | Verificación                              | Estado |
|-----------|-------------------------------------------|--------|
| 1         | logs muestran `profile=dev`               | ⬜     |
| 2         | logs muestran `profile=durable journal=bbolt path=/tmp/golem.db` | ⬜     |
| 3         | `curl /healthz` → 200                     | ⬜     |
| 4a        | `TestR4HappyPath` PASS                    | ⬜     |
| 4b        | `TestR4StructuralDivergence` PASS          | ⬜     |
| 5         | 5 eventos en orden, sin rollback          | ⬜     |

---

## Rollback manual (opcional)

Para forzar un escenario de rollback manualmente:

```bash
export GOLEM_MIGRATION_OBSERVE_WINDOW=100ms

# Seedear target con datos divergentes antes de ejecutar el harness
# (requiere un cliente que escriba directamente en el graph memstore
#  o un adapter de test que populé el target antes de Run())
```

> **Nota**: el rollback real se activa cuando el harness detecta divergencia
> semántica durante la ventana de observación (diffing step). Con la
> implementación actual de full-reload, source y target son byte-idénticos
> tras la carga, por lo que el rollback automático no se dispara a menos
> que haya mutaciones durante la observe-window. El test de estructura
> (`TestR4StructuralDivergence`) verifica que la función `Diff()` detecta
> correctamente la divergencia.

---

## Comandos útiles de debug

```bash
# Verbin de logs del harness
RUST_LOG=debug ./golem-api

# Limpiar estado entre ejecuciones
rm -f /tmp/golem.db

# Re-ejecutar test con salida detallada
go test -race -v -run TestR4 ./tck/... 2>&1 | tee /tmp/r4-output.txt
```
