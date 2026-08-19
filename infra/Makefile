# Golem Infrastructure
#
# Directorio de contenedores reproducibles para desarrollo y benchmark gate.
#
# ## Estructura
#
# - `nats/`       — NATS Server con JetStream (dev + CI)
# - `hugegraph/` — Apache HugeGraph (benchmark candidate #1)
# - `nebula/`     — NebulaGraph (benchmark candidate #2)
# - `dgraph/`    — Dgraph (benchmark candidate #3)
# - `bench/`     — Scripts de benchmark workloads (W1-W6)
#
# ## Uso
#
# Desarrollo local (solo NATS):
#   just infra-up       # levanta NATS
#   just infra-down     # lo derriba
#
# Benchmark gate (graph DB candidates):
#   make bench-hugegraph   # levanta HugeGraph
#   make bench-nebula      # levanta NebulaGraph
#   make bench-dgraph       # levanta Dgraph
#
# ## Requisitos
#
# - podman o docker
# - make (GNU make)
#
# ## Notas
#
# - Los Containerfiles usan imagenes oficiales
# - Los candidates de graph DB son para el benchmark gate de ADR-086
# - Los volumenes son persistentes (no se pierden al restart)
# - Puertos expuestos:
#   nats:     4222 (client), 8222 (monitoring), 6222 (cluster)
#   hugegraph: 8080 (HTTP), 8090 (HAProxy), 8081 (Gremlin)
#   nebula:    9669 (Nebula), 19669 (HTTP)
#   dgraph:    8080 (HTTP), 9080 (Grpc), 8000 (ratel)

.PHONY: help nats-up nats-down nats-logs
.PHONY: hugegraph-up hugegraph-down hugegraph-logs
.PHONY: nebula-up nebula-down nebula-logs
.PHONY: dgraph-up dgraph-down dgraph-logs

help:
	@echo "Golem Infrastructure"
	@echo ""
	@echo "Desarrollo local:"
	@echo "  just infra-up      — levantar NATS (dev profile)"
	@echo "  just infra-down   — detener NATS"
	@echo ""
	@echo "Benchmark gate (ADR-086):"
	@echo "  make hugegraph-up  — HugeGraph para benchmark"
	@echo "  make nebula-up     — NebulaGraph para benchmark"
	@echo "  make dgraph-up     — Dgraph para benchmark"
	@echo ""
	@echo "Todos los candidatos:"
	@echo "  make bench-all-up   — levantar todos"
	@echo "  make bench-all-down — detener todos"

# NATS
nats-up:
	podman build -t golem-nats ./nats
	podman run -d --name golem-nats \
		-p 4222:4222 -p 8222:8222 -p 6222:6222 \
		-v golem-nats-data:/data \
		golem-nats

nats-down:
	podman stop golem-nats 2>/dev/null; podman rm golem-nats 2>/dev/null; true

nats-logs:
	podman logs golem-nats

# HugeGraph (benchmark candidate #1)
hugegraph-up:
	podman build -t golem-hugegraph ./hugegraph
	podman run -d --name golem-hugegraph \
		-p 8080:8080 -p 8090:8090 -p 8081:8081 \
		-v golem-hugegraph-data:/opt/hugegraph \
		golem-hugegraph

hugegraph-down:
	podman stop golem-hugegraph 2>/dev/null; podman rm golem-hugegraph 2>/dev/null; true

hugegraph-logs:
	podman logs golem-hugegraph

# NebulaGraph (benchmark candidate #2)
nebula-up:
	podman build -t golem-nebula ./nebula
	podman run -d --name golem-nebula \
		-p 9669:9669 -p 19669:19669 \
		-v golem-nebula-data:/data \
		golem-nebula

nebula-down:
	podman stop golem-nebula 2>/dev/null; podman rm golem-nebula 2>/dev/null; true

nebula-logs:
	podman logs golem-nebula

# Dgraph (benchmark candidate #3)
dgraph-up:
	podman build -t golem-dgraph ./dgraph
	podman run -d --name golem-dgraph \
		-p 8080:8080 -p 9080:9080 -p 8000:8000 \
		-v golem-dgraph-data:/dgraph \
		golem-dgraph

dgraph-down:
	podman stop golem-dgraph 2>/dev/null; podman rm golem-dgraph 2>/dev/null; true

dgraph-logs:
	podman logs golem-dgraph

# Todos los benchmark candidates
bench-all-up: nats-up hugegraph-up nebula-up dgraph-up

bench-all-down:
	-nats-down; hugegraph-down; nebula-down; dgraph-down

# Volumenes (limpieza completa)
clean-volumes:
	podman volume rm golem-nats-data golem-hugegraph-data golem-nebula-data golem-dgraph-data 2>/dev/null; true

# Status
status:
	@echo "=== Contenedores ==="
	@podman ps -a --format "{{.Names}}\t{{.Status}}\t{{.Ports}}" | grep -E "golem|hugegraph|nebula|dgraph|nats" || echo "(ninguno corriendo)"
