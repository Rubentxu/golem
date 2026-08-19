#!/usr/bin/env python3
"""
Workload generators for Golem graph DB benchmark (ADR-086).

Each workload (W1–W6) is a function that:
  - Takes a GraphDB client
  - Executes a specific access pattern
  - Returns a WorkloadResult with timing + throughput metrics

Workload definitions (ADR-086):
  W1: Bulk load       — 100K nodes + 500K edges (simulates migration from journal)
  W2: Neighborhood    — 1000 concurrent queries, depth=3, maxNodes=100
  W3: Traversal       — 100 queries, depth=4, typed edge filter
  W4: Point read      — 10K GetNode by ID
  W5: Mutation rate    — 100 ops/sec for 60 seconds
  W6: Mixed workload  — 80% reads, 20% writes

Node kinds: WorkItem, Project, PackageComponent, Artifact, Requirement,
            Task, Cell, Agent, PolicyRule, SupplyChainNode

Edge types: DEPENDS_ON, IMPLEMENTS, CONTAINS, CHAIN, LINKS,
            TRACKS, AUTHORED_BY, ASSIGNED_TO, VERIFIES
"""

import math
import random
import statistics
import time
import uuid
from dataclasses import dataclass
from typing import Callable

from .domain import NODE_KINDS, EDGE_TYPES, NODE_KIND_DEFS, EDGE_TYPE_DEFS


@dataclass
class WorkloadResult:
    name: str
    duration_ms: float
    ops: int
    throughput: float       # ops/sec
    p50_ms: float
    p95_ms: float
    p99_ms: float
    errors: int
    truncated: bool = False


# ── Data generators ──────────────────────────────────────────────────────────

def new_node_id() -> str:
    return f"n-{uuid.uuid4().hex[:12]}"

def new_edge_id() -> str:
    return f"e-{uuid.uuid4().hex[:12]}"

def rand_kind() -> str:
    return random.choice(NODE_KINDS)

def rand_edge_type() -> str:
    return random.choice(EDGE_TYPES)

def make_node_attrs(kind: str) -> dict:
    """Generate random attributes for a node kind."""
    kd = NODE_KIND_DEFS[kind]
    attrs = {}
    for p in kd.properties:
        if "status" in p:
            attrs[p] = random.choice(["open", "in_progress", "done", "blocked"])
        elif "priority" in p or "risk" in p:
            attrs[p] = random.choice(["low", "medium", "high", "critical"])
        elif "size" in p or "estimate" in p:
            attrs[p] = str(random.randint(1, 40))
        elif "version" in p:
            attrs[p] = f"{random.randint(0,5)}.{random.randint(0,20)}.{random.randint(0,99)}"
        elif "name" in p or "title" in p:
            attrs[p] = f"{kind.lower()}-{uuid.uuid4().hex[:8]}"
        else:
            attrs[p] = uuid.uuid4().hex[:8]
    return attrs

def make_edge_attrs(etype: str) -> dict:
    ed = EDGE_TYPE_DEFS[etype]
    attrs = {}
    for p in ed.properties:
        if "weight" in p:
            attrs[p] = str(random.randint(1, 10))
        elif "position" in p:
            attrs[p] = str(random.randint(1, 100))
        elif "context" in p:
            attrs[p] = uuid.uuid4().hex[:8]
        elif "role" in p:
            attrs[p] = random.choice(["owner", "reviewer", "approver"])
        elif "assigned_at" in p:
            attrs[p] = str(int(time.time()))
        elif "method" in p:
            attrs[p] = random.choice(["test", "review", "inspection"])
    return attrs


# ── Bulk loader ─────────────────────────────────────────────────────────────

def w1_bulk_load(client, *, nodes: int = 100_000, edges: int = 500_000) -> WorkloadResult:
    """
    W1: Bulk load — creates `nodes` nodes + `edges` edges.
    Ratio: 1 node : 5 edges (representative of Golem's dense graph).
    """
    print(f"    generating {nodes} nodes + {edges} edges... ", end="", flush=True)
    t0 = time.perf_counter()

    # Generate nodes in batches
    batch_size = 500
    all_node_ids = []
    ops = []

    for i in range(nodes):
        nid = new_node_id()
        all_node_ids.append(nid)
        ops.append({
            "kind": "upsert_node",
            "target": nid,
            "data": {"kind": rand_kind(), "attributes": make_node_attrs(rand_kind())},
        })
        if len(ops) >= batch_size:
            client.apply_ops(ops)
            ops = []

    if ops:
        client.apply_ops(ops)

    # Generate edges (graph is connected via chain edges)
    ops = []
    edge_pairs = []
    for i in range(edges):
        src = random.choice(all_node_ids)
        tgt = random.choice(all_node_ids)
        etype = rand_edge_type()
        eid = new_edge_id()
        edge_pairs.append((src, tgt))
        ops.append({
            "kind": "upsert_edge",
            "target": eid,
            "data": {"type": etype, "source": src, "target": tgt, "attributes": make_edge_attrs(etype)},
        })
        if len(ops) >= batch_size:
            client.apply_ops(ops)
            ops = []
            if i % 50_000 == 0:
                print(f"  {i}/{edges} edges... ", end="", flush=True)

    if ops:
        client.apply_ops(ops)

    duration_ms = (time.perf_counter() - t0) * 1000
    throughput = (nodes + edges) / (duration_ms / 1000)
    print(f"done in {duration_ms/1000:.1f}s")

    return WorkloadResult(
        name="W1_BulkLoad",
        duration_ms=duration_ms,
        ops=nodes + edges,
        throughput=throughput,
        p50_ms=0, p95_ms=0, p99_ms=0,
        errors=0,
    )


# ── Neighborhood queries ───────────────────────────────────────────────────

def _neighborhood_op(client, root: str, depth: int, max_nodes: int) -> tuple[float, int]:
    """Single neighborhood query — returns (latency_ms, result_node_count)."""
    t0 = time.perf_counter()
    result = client.neighborhood(root, depth=depth, max_nodes=max_nodes)
    latency_ms = (time.perf_counter() - t0) * 1000
    return latency_ms, len(result.get("nodes", []))

def w2_neighborhood(client, *, queries: int = 1000, depth: int = 3, max_nodes: int = 100) -> WorkloadResult:
    """
    W2: Neighborhood query — bounded BFS from random root nodes.
    Runs `queries` concurrent-equivalent queries (sequential for benchmark).
    """
    # Get a sample of node IDs to use as roots
    all_nodes = _sample_node_ids(client, min(queries, 500))
    if not all_nodes:
        return _empty_result("W2_Neighborhood")

    root_ids = [random.choice(all_nodes) for _ in range(queries)]
    latencies = []
    errors = 0

    for root in root_ids:
        try:
            lat_ms, _ = _neighborhood_op(client, root, depth, max_nodes)
            latencies.append(lat_ms)
        except Exception:
            errors += 1

    return _latency_result("W2_Neighborhood", latencies, queries, errors)

def _sample_node_ids(client, n: int) -> list[str]:
    """Sample N node IDs from the graph using ListNodes."""
    try:
        nodes = client.list_nodes("bench") if hasattr(client, "list_nodes") else []
        if len(nodes) >= n:
            return random.sample(nodes, n)
        return [n["id"] for n in nodes]
    except Exception:
        return []


# ── Typed traversal ────────────────────────────────────────────────────────

def _traversal_op(
    client, root: str, edge_types: list[str], kinds: list[str],
    depth: int, max_nodes: int, max_edges: int
) -> tuple[float, int, bool]:
    """Single traversal query — returns (latency_ms, node_count, truncated)."""
    t0 = time.perf_counter()
    result = client.traversal(root, edge_types=edge_types, kinds=kinds,
                              depth=depth, max_nodes=max_nodes, max_edges=max_edges)
    latency_ms = (time.perf_counter() - t0) * 1000
    return latency_ms, len(result.get("nodes", [])), result.get("truncated", False)

def w3_traversal(client, *, queries: int = 100, depth: int = 4) -> WorkloadResult:
    """
    W3: Typed traversal — depth=4, edge-type filter, node-kind filter.
    Represents the real GOLEM ADR-056 traversal workloads.
    """
    edge_type_filters = [
        ["DEPENDS_ON"],
        ["IMPLEMENTS", "CONTAINS"],
        ["CHAIN", "LINKS"],
    ]
    kind_filters = [["WorkItem"], ["Artifact"], ["WorkItem", "Task"]]

    all_nodes = _sample_node_ids(client, min(queries, 200))
    if not all_nodes:
        return _empty_result("W3_Traversal")

    latencies = []
    truncated_count = 0
    errors = 0

    for i in range(queries):
        root = random.choice(all_nodes)
        et_filter = random.choice(edge_type_filters)
        k_filter = random.choice(kind_filters)
        try:
            lat_ms, _, trunc = _traversal_op(
                client, root, et_filter, k_filter,
                depth=depth, max_nodes=100, max_edges=200
            )
            latencies.append(lat_ms)
            if trunc:
                truncated_count += 1
        except Exception:
            errors += 1

    result = _latency_result("W3_Traversal", latencies, queries, errors)
    result.truncated = truncated_count > 0
    return result


# ── Point read ─────────────────────────────────────────────────────────────

def w4_point_read(client, *, reads: int = 10_000) -> WorkloadResult:
    """
    W4: Point read — 10K GetNode by ID.
    Represents the read-heavy workload of projection queries.
    """
    all_nodes = _sample_node_ids(client, min(reads, 5000))
    if not all_nodes:
        return _empty_result("W4_PointRead")

    ids = [random.choice(all_nodes) for _ in range(reads)]
    latencies = []
    errors = 0

    for node_id in ids:
        t0 = time.perf_counter()
        try:
            client.get_node(node_id)
            latencies.append((time.perf_counter() - t0) * 1000)
        except Exception:
            errors += 1

    return _latency_result("W4_PointRead", latencies, reads, errors)


# ── Mutation rate ──────────────────────────────────────────────────────────

def w5_mutation_rate(client, *, target_rate: int = 100, duration_s: int = 60) -> WorkloadResult:
    """
    W5: Mutation rate — sustains `target_rate` ops/sec for `duration_s` seconds.
    Measures write throughput under sustained load.
    """
    all_nodes = _sample_node_ids(client, 200)
    if not all_nodes:
        return _empty_result("W5_Mutation")

    interval_s = 1.0 / target_rate
    ops_count = 0
    latencies = []
    errors = 0
    t_start = time.perf_counter()
    deadline = t_start + duration_s

    while time.perf_counter() < deadline:
        t_op_start = time.perf_counter()

        # Alternate between node and edge mutations
        op_type = random.choice(["node", "edge"])
        try:
            if op_type == "node":
                nid = new_node_id()
                client.apply_ops([{
                    "kind": "upsert_node",
                    "target": nid,
                    "data": {"kind": rand_kind(), "attributes": make_node_attrs(rand_kind())},
                }])
            else:
                src = random.choice(all_nodes)
                tgt = random.choice(all_nodes)
                etype = rand_edge_type()
                client.apply_ops([{
                    "kind": "upsert_edge",
                    "target": new_edge_id(),
                    "data": {"type": etype, "source": src, "target": tgt,
                            "attributes": make_edge_attrs(etype)},
                }])
            ops_count += 1
            latencies.append((time.perf_counter() - t_op_start) * 1000)
        except Exception:
            errors += 1

        # Throttle to target rate
        elapsed = time.perf_counter() - t_op_start
        sleep_time = interval_s - elapsed
        if sleep_time > 0:
            time.sleep(sleep_time)

    total_duration_ms = (time.perf_counter() - t_start) * 1000
    throughput = ops_count / (total_duration_ms / 1000)
    latencies.sort()
    p50 = latencies[len(latencies)//2] if latencies else 0
    p95 = latencies[int(len(latencies)*0.95)] if latencies else 0
    p99 = latencies[int(len(latencies)*0.99)] if latencies else 0

    return WorkloadResult(
        name="W5_Mutation",
        duration_ms=total_duration_ms,
        ops=ops_count,
        throughput=throughput,
        p50_ms=p50, p95_ms=p95, p99_ms=p99,
        errors=errors,
    )


# ── Mixed workload ─────────────────────────────────────────────────────────

def w6_mixed(client, *, total_ops: int = 10_000, read_ratio: float = 0.8) -> WorkloadResult:
    """
    W6: Mixed workload — 80% reads, 20% writes.
    Simulates realistic production read/write ratio.
    """
    all_nodes = _sample_node_ids(client, 500)
    if not all_nodes:
        return _empty_result("W6_Mixed")

    read_ops = int(total_ops * read_ratio)
    write_ops = total_ops - read_ops
    latencies = []
    errors = 0

    # Interleave reads and writes
    ops_sequence = ["read"] * read_ops + ["write"] * write_ops
    random.shuffle(ops_sequence)

    t0 = time.perf_counter()
    for op in ops_sequence:
        op_t0 = time.perf_counter()
        try:
            if op == "read":
                root = random.choice(all_nodes)
                client.neighborhood(root, depth=2, max_nodes=50)
            else:
                nid = new_node_id()
                client.apply_ops([{
                    "kind": "upsert_node",
                    "target": nid,
                    "data": {"kind": rand_kind(), "attributes": make_node_attrs(rand_kind())},
                }])
            latencies.append((time.perf_counter() - op_t0) * 1000)
        except Exception:
            errors += 1

    duration_ms = (time.perf_counter() - t0) * 1000
    return _latency_result("W6_Mixed", latencies, total_ops, errors)


# ── Helpers ─────────────────────────────────────────────────────────────────

def _latency_result(name: str, latencies: list[float], total_ops: int, errors: int) -> WorkloadResult:
    latencies.sort()
    n = len(latencies)
    p50 = latencies[n//2] if n > 0 else 0
    p95 = latencies[int(n*0.95)] if n > 0 else 0
    p99 = latencies[int(n*0.99)] if n > 0 else 0
    duration_ms = latencies[-1] if latencies else 0
    throughput = total_ops / (duration_ms / 1000) if duration_ms > 0 else 0
    return WorkloadResult(
        name=name,
        duration_ms=duration_ms,
        ops=total_ops,
        throughput=throughput,
        p50_ms=p50, p95_ms=p95, p99_ms=p99,
        errors=errors,
    )

def _empty_result(name: str) -> WorkloadResult:
    return WorkloadResult(
        name=name,
        duration_ms=0, ops=0, throughput=0,
        p50_ms=0, p95_ms=0, p99_ms=0,
        errors=0,
    )


# ── Registry ────────────────────────────────────────────────────────────────

WORKLOADS: dict[str, Callable] = {
    "W1": w1_bulk_load,
    "W2": w2_neighborhood,
    "W3": w3_traversal,
    "W4": w4_point_read,
    "W5": w5_mutation_rate,
    "W6": w6_mixed,
}
