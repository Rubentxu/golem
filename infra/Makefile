# Golem Infrastructure — Benchmark Gate (ADR-086)
#
# Usa quadlets (systemd unit files para containers).
# Gestión via `systemctl --user`, no `podman run` directo.
#
# Imágenes locales (build con `make build`):
#   golem-nats:latest
#   golem-hugegraph:latest
#   golem-dgraph:latest
#
# Targets:
#   make build                — build todas las imágenes locales
#   make up                   — levantar todos via quadlets (systemd)
#   make down                 — detener todos via quadlets
#   make status               — estado de todos los quadlets
#   make logs service=s     — logs de un servicio (e.g. service=nats)
#
# Benchmark (requiere infra corriendo):
#   make bench service=       — ejecutar W1-W6 contra un candidate
#                             (service=hugegraph|nebula-allinone|dgraph)
#
# Instalación de quadlets:
#   mkdir -p ~/.config/containers/systemd/
#   ln -s infra/quadlets/*.container ~/.config/containers/systemd/
#   systemctl --user daemon-reload

.PHONY: help build up down status logs bench

REPO_ROOT := $(shell git -C . rev-parse --show-toplevel 2>/dev/null || echo ".")
SYSTEMD_DIR := $(HOME)/.config/containers/systemd

# ── Build ─────────────────────────────────────────────────────────────────

build: build-nats build-hugegraph build-dgraph
	@echo "==> Build complete"

build-nats:
	@echo "==> Building golem-nats:latest"
	podman build -t golem-nats:latest ./nats

build-hugegraph:
	@echo "==> Building golem-hugegraph:latest"
	podman build -t golem-hugegraph:latest ./hugegraph

build-dgraph:
	@echo "==> Building golem-dgraph:latest"
	podman build -t golem-dgraph:latest ./dgraph

# ── Lifecycle via systemd quadlets ─────────────────────────────────────────

# Asegurar que los symlinks de quadlets existen
_ensure-quadlets:
	@for svc in nats hugegraph dgraph nebula-allinone; do \
		link="$(SYSTEMD_DIR)/$$svc.container"; \
		if [ ! -e "$$link" ]; then \
			echo "==> Missing quadlet: $$link"; \
			echo "   Run: ln -s infra/quadlets/$$svc.container $$link"; \
			echo "   Then: systemctl --user daemon-reload"; \
		fi; \
	done

up: _ensure-quadlets
	@echo "==> Starting all golem services via quadlets"
	systemctl --user start nats
	systemctl --user start hugegraph
	systemctl --user start dgraph
	systemctl --user start nebula-allinone
	@echo "==> All services started. Check with: make status"

down:
	@echo "==> Stopping all golem services"
	-systemctl --user stop nebula-allinone 2>/dev/null
	-systemctl --user stop dgraph 2>/dev/null
	-systemctl --user stop hugegraph 2>/dev/null
	-systemctl --user stop nats 2>/dev/null
	@echo "==> All services stopped"

status:
	@echo "=== Quadlet Status ==="
	@for svc in nats hugegraph dgraph nebula-allinone; do \
		status=$$(systemctl --user is-active $$svc 2>/dev/null || echo "inactive"); \
		echo "  $$svc: $$status"; \
	done

logs:
ifneq ($(service),)
	@echo "==> Logs for $(service):"
	journalctl --user -u $(service) -n 50 --no-pager
else
	@echo "Usage: make logs service=<svc>"
	@echo "  services: nats, hugegraph, dgraph, nebula-allinone"
endif

# ── Health check ──────────────────────────────────────────────────────────

health:
	@echo "=== Health Checks ==="
	@echo -n "  nats:         "; \
		curl -sf http://localhost:8222/health > /dev/null && echo "OK" || echo "DOWN"
	@echo -n "  hugegraph:    "; \
		curl -sf http://localhost:8080/graphs/hugegraph > /dev/null && echo "OK" || echo "DOWN"
	@echo -n "  dgraph:       "; \
		curl -sf http://localhost:8080/health > /dev/null && echo "OK" || echo "DOWN"
	@echo -n "  nebula:       "; \
		curl -sf http://localhost:19669/status > /dev/null && echo "OK" || echo "DOWN"

# ── Benchmark ──────────────────────────────────────────────────────────────

_bench-url = $(if $(findstring hugegraph,$(service)),http://localhost:8080,$(if $(findstring nebula,$(service)),http://localhost:9669,$(if $(findstring dgraph,$(service)),http://localhost:8080,http://localhost:8080)))

bench: up
ifneq ($(service),)
	@echo "==> Running benchmark against $(service) ($(call _bench-url))"
	@# Ensure service is healthy before running
	@sleep 5 && make health
	cd $(REPO_ROOT) && python3 -m infra.bench.run $(service) $(call _bench-url) --tck --output bench-results-$(service).json
else
	@echo "Usage: make bench service=hugegraph|nebula-allinone|dgraph"
	@echo "Example: make bench service=hugegraph"
endif

bench-all: bench-hugegraph bench-dgraph
bench-hugegraph:
	@echo "==> Benchmark: HugeGraph"
	$(MAKE) bench service=hugegraph
bench-dgraph:
	@echo "==> Benchmark: Dgraph"
	$(MAKE) bench service=dgraph
bench-nebula:
	@echo "==> Benchmark: NebulaGraph (all-in-one)"
	$(MAKE) bench service=nebula-allinone

# ── Clean ─────────────────────────────────────────────────────────────────

clean-data:
	-podman volume rm golem-nats-data golem-hugegraph-data \
		golem-nebula-data golem-dgraph-data 2>/dev/null || true

clean-results:
	-rm -f bench-results-*.json

# ── Help ──────────────────────────────────────────────────────────────────

help:
	@echo "Golem Infrastructure — Quadlets (ADR-086)"
	@echo ""
	@echo "Setup (one-time):"
	@echo "  mkdir -p ~/.config/containers/systemd/"
	@echo "  ln -s infra/quadlets/*.container ~/.config/containers/systemd/"
	@echo "  systemctl --user daemon-reload"
	@echo ""
	@echo "Lifecycle:"
	@echo "  make build          build local images"
	@echo "  make up             start all services (systemd)"
	@echo "  make down           stop all services"
	@echo "  make status         show service status"
	@echo "  make health         check /health endpoints"
	@echo "  make logs service=<svc>  tail logs"
	@echo ""
	@echo "Benchmark:"
	@echo "  make bench service=hugegraph       # W1-W6 + TCK"
	@echo "  make bench service=nebula-allinone # W1-W6 + TCK"
	@echo "  make bench service=dgraph          # W1-W6 + TCK"
	@echo "  make bench-all                    # HugeGraph + Dgraph"
	@echo ""
	@echo "Cleanup:"
	@echo "  make down && make clean-data       # stop + delete volumes"
