# Golem Infrastructure — Benchmark Gate (ADR-086)

Two deployment options: **Docker Compose** (recommended, portable) or **Quadlets** (Linux/systemd only).

## Docker Compose — Portable (Linux / Windows / Mac)

**Recommended** — works everywhere.

```bash
# Start all services
docker compose up -d

# Start specific service
docker compose up -d hugegraph

# Stop all
docker compose down

# Check status
docker compose ps

# Tail logs
docker compose logs nats -f

# Run benchmark
python3 -m infra.bench.run hugegraph http://localhost:8080 --tck --output bench-results-hugegraph.json
```

Make targets also work:

```bash
make up
make health
make bench service=hugegraph
make bench service=dgraph
make bench service=nebula-allinone
make down
```

## Quadlets — Linux/systemd only

For users with systemd on Linux:

```bash
mkdir -p ~/.config/containers/systemd/
ln -s infra/quadlets/*.container ~/.config/containers/systemd/
systemctl --user daemon-reload
systemctl --user start nats hugegraph dgraph nebula-allinone
systemctl --user status nats hugegraph dgraph nebula-allinone
```

## Services

| Service | Port | Benchmark URL | Purpose |
|---------|------|--------------|---------|
| nats | 4222 | — | Dev transport + benchmark messaging |
| hugegraph | 8080 | http://localhost:8080 | Candidate #1 |
| dgraph | 8080 | http://localhost:8080 | Candidate #3 |
| nebula-allinone | 9669 | http://localhost:9669 | Candidate #2 |

## Benchmark Results

Each `run` produces `bench-results-<service>.json`. R4 assessment (ADR-086):

- W2 p99 latency < 100ms AND W5 throughput > 500 ops/sec → **PASS**
- Results feed into ADR-087 (graph DB decision)
