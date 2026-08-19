# Golem Infrastructure — Benchmark Gate (ADR-086)
#
# Targets:
#   make infra-up              — NATS (dev profile)
#   make bench-hugegraph-up    — HugeGraph benchmark candidate
#   make bench-nebula-up       — NebulaGraph benchmark candidate
#   make bench-dgraph-up       — Dgraph benchmark candidate
#   make bench-all-up          — All benchmark candidates
#   make bench-all-down        — Stop all
#   make bench-hugegraph       — Run full benchmark suite against HugeGraph
#   make bench-nebula          — Run full benchmark suite against NebulaGraph
#   make bench-dgraph          — Run full benchmark suite against Dgraph
#   make bench-status          — Show health of all containers

.PHONY: help nats-up nats-down nats-logs nats-health
.PHONY: bench-hugegraph-up bench-hugegraph-down bench-hugegraph bench-hugegraph-logs
.PHONY: bench-nebula-up bench-nebula-down bench-nebula bench-nebula-logs
.PHONY: bench-dgraph-up bench-dgraph-down bench-dgraph bench-dgraph-logs
.PHONY: bench-all-up bench-all-down bench-all bench-status bench-tck

PYTHON := python3
PYTHON_DEPS := -r requirements.txt

# ── Helpers ─────────────────────────────────────────────────────────────────

_health_url = $(shell echo "$(1)" | sed 's|http://||' | cut -d: -f1)
_poll = for i in $$(seq 1 30); do \
	if curl -sf "$(1)" > /dev/null 2>&1; then \
		echo "$(2) up after $$i attempts"; break; fi; \
	echo "Waiting for $(2)... ($$i/30)"; sleep 2; \
done

# ── NATS (dev + benchmark transport) ────────────────────────────────────────

nats-up: nats-build
	@echo "==> Starting NATS (dev profile transport)"
	podman run -d --name golem-nats \
		-p 4222:4222 -p 8222:8222 -p 6222:6222 \
		-v golem-nats-data:/data \
		golem-nats:latest
	@$(call _poll,http://localhost:8222/health,NATS)

nats-build:
	podman build -t golem-nats:latest ./nats

nats-down:
	-podman stop golem-nats; podman rm golem-nats

nats-logs:
	podman logs golem-nats

nats-health:
	@curl -sf http://localhost:8222/health && echo "NATS: healthy" || echo "NATS: DOWN"

# ── HugeGraph ────────────────────────────────────────────────────────────────

bench-hugegraph-up: bench-hugegraph-build
	@echo "==> Starting HugeGraph (benchmark candidate #1)"
	podman run -d --name golem-hugegraph \
		-p 8080:8080 -p 8090:8090 -p 8081:8081 \
		-v golem-hugegraph-data:/opt/hugegraph \
		golem-hugegraph:latest
	@$(call _poll,http://localhost:8080/graphs/hugegraph,HugeGraph)

bench-hugegraph-build:
	podman build -t golem-hugegraph:latest ./hugegraph

bench-hugegraph-down:
	-podman stop golem-hugegraph; podman rm golem-hugegraph

bench-hugegraph-logs:
	podman logs golem-hugegraph

bench-hugegraph: bench-hugegraph-up
	@echo "==> Running benchmark suite against HugeGraph"
	cd $(CURDIR)/bench && \
		$(PYTHON) run.py hugegraph http://localhost:8080 --tck --output ../../bench-results-hugegraph.json
	@echo "==> Benchmark complete: bench-results-hugegraph.json"

# ── NebulaGraph ──────────────────────────────────────────────────────────────

bench-nebula-up: bench-nebula-build
	@echo "==> Starting NebulaGraph (benchmark candidate #2)"
	podman run -d --name golem-nebula \
		-p 9669:9669 -p 19669:19669 -p 7689:7689 \
		-v golem-nebula-data:/data \
		golem-nebula:latest
	@$(call _poll,http://localhost:19669/status,NebulaGraph)

bench-nebula-build:
	podman build -t golem-nebula:latest ./nebula

bench-nebula-down:
	-podman stop golem-nebula; podman rm golem-nebula

bench-nebula-logs:
	podman logs golem-nebula

bench-nebula: bench-nebula-up
	@echo "==> Running benchmark suite against NebulaGraph"
	cd $(CURDIR)/bench && \
		$(PYTHON) run.py nebula http://localhost:9669 --tck --output ../../bench-results-nebula.json
	@echo "==> Benchmark complete: bench-results-nebula.json"

# ── Dgraph ──────────────────────────────────────────────────────────────────

bench-dgraph-up: bench-dgraph-build
	@echo "==> Starting Dgraph (benchmark candidate #3)"
	podman run -d --name golem-dgraph \
		-p 8080:8080 -p 9080:9080 -p 8000:8000 \
		-v golem-dgraph-data:/dgraph \
		golem-dgraph:latest
	@$(call _poll,http://localhost:8080/health,Dgraph)

bench-dgraph-build:
	podman build -t golem-dgraph:latest ./dgraph

bench-dgraph-down:
	-podman stop golem-dgraph; podman rm golem-dgraph

bench-dgraph-logs:
	podman logs golem-dgraph

bench-dgraph: bench-dgraph-up
	@echo "==> Running benchmark suite against Dgraph"
	cd $(CURDIR)/bench && \
		$(PYTHON) run.py dgraph http://localhost:8080 --tck --output ../../bench-results-dgraph.json
	@echo "==> Benchmark complete: bench-results-dgraph.json"

# ── All candidates ──────────────────────────────────────────────────────────

bench-all-up: nats-up bench-hugegraph-up bench-nebula-up bench-dgraph-up
	@echo "==> All benchmark infrastructure running"

bench-all-down: bench-hugegraph-down bench-nebula-down bench-dgraph-down nats-down
	@echo "==> All containers stopped"

# Run all candidates sequentially (requires infra running)
bench-all: bench-all-up
	@echo "==> Running all candidates..."
	@$(MAKE) bench-hugegraph
	@$(MAKE) bench-nebula
	@$(MAKE) bench-dgraph
	@echo "==> All candidates benchmarked. See bench-results-*.json"

# ── Status ──────────────────────────────────────────────────────────────────

bench-status:
	@echo "=== Benchmark Infrastructure Status ==="
	@echo "NATS:        $$([ $$(podman ps --format '{{.Names}}' 2>/dev/null | grep -c golem-nats) -gt 0 ] && echo 'UP' || echo 'DOWN')"
	@echo "HugeGraph:   $$([ $$(podman ps --format '{{.Names}}' 2>/dev/null | grep -c golem-hugegraph) -gt 0 ] && echo 'UP' || echo 'DOWN')"
	@echo "NebulaGraph: $$([ $$(podman ps --format '{{.Names}}' 2>/dev/null | grep -c golem-nebula) -gt 0 ] && echo 'UP' || echo 'DOWN')"
	@echo "Dgraph:      $$([ $$(podman ps --format '{{.Names}}' 2>/dev/null | grep -c golem-dgraph) -gt 0 ] && echo 'UP' || echo 'DOWN')"

# ── TCK only ───────────────────────────────────────────────────────────────

bench-tck-%:
	@echo "==> Running TCK validation on $*"
	cd $(CURDIR)/bench && \
		$(PYTHON) run.py $* http://localhost:$(if $(findstring hugegraph,$*),8080,$(if $(findstring nebula,$*),9669,8080)) --tck

# ── Clean ────────────────────────────────────────────────────────────────────

clean-data:
	-podman volume rm golem-nats-data golem-hugegraph-data \
		golem-nebula-data golem-dgraph-data 2>/dev/null || true

clean-results:
	-rm -f bench-results-*.json

# ── Help ─────────────────────────────────────────────────────────────────────

help:
	@echo "Golem Infrastructure — Benchmark Gate (ADR-086)"
	@echo ""
	@echo "Dev profile:"
	@echo "  just infra-up         levant NATS (dev profile transport)"
	@echo "  just infra-down       detener NATS"
	@echo ""
	@echo "Benchmark candidates (ADR-086):"
	@echo "  make bench-hugegraph-up   levantar HugeGraph"
	@echo "  make bench-nebula-up      levantar NebulaGraph"
	@echo "  make bench-dgraph-up      levantar Dgraph"
	@echo "  make bench-all-up         levantar todos"
	@echo ""
	@echo "Run benchmarks:"
	@echo "  make bench-hugegraph      levantar + ejecutar W1-W6 contra HugeGraph"
	@echo "  make bench-nebula         levantar + ejecutar W1-W6 contra NebulaGraph"
	@echo "  make bench-dgraph         levantar + ejecutar W1-W6 contra Dgraph"
	@echo "  make bench-all            ejecutar contra los 3 candidates"
	@echo ""
	@echo "Status:"
	@echo "  make bench-status         show container health"
	@echo "  make bench-tck-hugegraph TCK validation only"
	@echo ""
	@echo "Cleanup:"
	@echo "  make bench-all-down       detener todos los containers"
	@echo "  make clean-data           eliminar volúmenes persistentes"
	@echo "  make clean-results        eliminar resultados de benchmark"
