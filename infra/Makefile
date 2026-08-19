# Golem Infrastructure — Benchmark Gate (ADR-086)
#
# Portable across Linux, Windows (WSL2), Mac via docker compose.
#
# Usage:
#   docker compose up -d            # start all
#   docker compose up -d <service>   # start specific
#   docker compose down             # stop all
#   docker compose ps               # status
#   docker compose logs <service>   # logs
#
# Build images:
#   make build           # build all images
#   make build-nats
#   make build-hugegraph
#   make build-dgraph
#   make build-nebula
#
# Benchmark:
#   make bench service=hugegraph
#   make bench service=dgraph
#   make bench service=nebula-allinone
#
# Health:
#   make health

.PHONY: help build build-nats build-hugegraph build-dgraph build-nebula \
        up down ps logs health bench bench-hugegraph bench-dgraph bench-nebula

REPO_ROOT := $(shell git -C . rev-parse --show-toplevel 2>/dev/null || echo ".")

# ── Build ─────────────────────────────────────────────────────────────────

build: build-nats build-hugegraph build-dgraph build-nebula
	@echo "==> All images built"

build-nats:
	@echo "==> Building golem-nats:latest"
	podman build -t golem-nats:latest ./nats

build-hugegraph:
	@echo "==> Building golem-hugegraph:latest"
	podman build -t golem-hugegraph:latest ./hugegraph

build-dgraph:
	@echo "==> Building golem-dgraph:latest"
	podman build -t golem-dgraph:latest ./dgraph

build-nebula:
	@echo "==> Building golem-nebula:latest"
	podman build -t golem-nebula:latest ./nebula

# ── Lifecycle ─────────────────────────────────────────────────────────────

up:
	cd $(REPO_ROOT)/infra && docker compose up -d

down:
	cd $(REPO_ROOT)/infra && docker compose down

ps:
	cd $(REPO_ROOT)/infra && docker compose ps

logs:
	@echo "Usage: make logs service=<name>"
	@echo "Services: nats, hugegraph, dgraph, nebula-allinone"
	@echo "Example: make logs service=hugegraph"

logs-%:
	cd $(REPO_ROOT)/infra && docker compose logs $* -f

# ── Health ───────────────────────────────────────────────────────────────

health:
	@echo "=== Health Checks ==="
	@echo -n "  nats:         "; \
		curl -sf http://localhost:8222/health > /dev/null && echo "UP" || echo "DOWN"
	@echo -n "  hugegraph:    "; \
		curl -sf http://localhost:8080/graphs/hugegraph > /dev/null && echo "UP" || echo "DOWN"
	@echo -n "  dgraph:       "; \
		curl -sf http://localhost:8080/health > /dev/null && echo "UP" || echo "DOWN"
	@echo -n "  nebula:       "; \
		curl -sf http://localhost:19669/status > /dev/null && echo "UP" || echo "DOWN"

# ── Benchmark ────────────────────────────────────────────────────────────

_bench-url = $(if $(findstring hugegraph,$(service)),http://localhost:8080,$(if $(findstring nebula,$(service)),http://localhost:9669,http://localhost:8080))

bench: up
ifneq ($(service),)
	@echo "==> Waiting for $(service) to be healthy..."
	@for i in $$(seq 1 30); do \
		if curl -sf $(call _bench-url)/health > /dev/null 2>&1 || \
		   curl -sf $(call _bench-url)/graphs/hugegraph > /dev/null 2>&1 || \
		   curl -sf $(call _bench-url)/status > /dev/null 2>&1; then \
			echo "==> $(service) is healthy"; break; fi; \
		echo "    waiting... ($$i/30)"; sleep 2; \
	done
	@echo "==> Running benchmark against $(service)"
	cd $(REPO_ROOT) && python3 -m infra.bench.run $(service) $(call _bench-url) --tck --output bench-results-$(service).json
else
	@echo "Usage: make bench service=hugegraph|dgraph|nebula-allinone"
endif

bench-hugegraph:
	$(MAKE) bench service=hugegraph

bench-dgraph:
	$(MAKE) bench service=dgraph

bench-nebula:
	$(MAKE) bench service=nebula-allinone

bench-all: bench-hugegraph bench-dgraph

# ── Help ─────────────────────────────────────────────────────────────────

help:
	@echo "Golem Infrastructure — Docker Compose (ADR-086)"
	@echo ""
	@echo "Lifecycle:"
	@echo "  docker compose up -d          start all services"
	@echo "  docker compose down          stop all"
	@echo "  docker compose ps           status"
	@echo "  make up / make down         wrapper for above"
	@echo ""
	@echo "Build:"
	@echo "  make build                 build all images"
	@echo "  make build-<svc>           build specific image"
	@echo ""
	@echo "Benchmark:"
	@echo "  make bench service=hugegraph         W1-W6 + TCK"
	@echo "  make bench service=dgraph"
	@echo "  make bench service=nebula-allinone"
	@echo ""
	@echo "Make targets:"
	@echo "  make health                check /health endpoints"
	@echo "  make logs service=<svc>    tail logs"
