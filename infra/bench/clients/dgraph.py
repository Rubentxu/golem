#!/usr/bin/env python3
"""
Dgraph client for Golem benchmark (ADR-086).

Dgraph is a native GraphQL graph DB with low-latency queries.
API: HTTP endpoint at :8080 (admin + mutations).

Dgraph v21+ uses JSON mutations (not N-Quads) and stores
dgraph.type as a list [string].

Ref: https://dgraph.io/docs/
"""

import json
import time
import uuid
from typing import Any

import requests

from ..domain import NODE_KIND_DEFS, EDGE_TYPE_DEFS


class DgraphClient:
    """Dgraph JSON mutation client mapped to Golem graph operations."""

    def __init__(self, url: str, commit_now: bool = True, flush_interval_s: float = 0.0):
        self.url = url.rstrip("/")
        self.session = requests.Session()
        self.session.headers["Content-Type"] = "application/json"
        self.schema = self._build_schema()
        self._commit_now = commit_now
        self._buffer: list[dict] = []
        self._flush_interval_s = flush_interval_s
        self._last_flush_time = time.perf_counter()

    def apply_ops(self, ops: list[dict], commit_now: bool | None = None) -> int:
        """Apply a batch of operations.

        When commit_now is None, uses the client's default (from __init__).
        When commit_now=False, mutations are buffered and flushed
        when buffer reaches 100 items, when flush_interval_s elapsed,
        or when flush() is called.
        """
        use_commit_now = commit_now if commit_now is not None else self._commit_now
        revision = 0

        for op in ops:
            kind = op.get("kind")
            target = op.get("target")
            data = op.get("data", {})

            if kind == "upsert_node":
                node = self._build_node(target, data)
                self._buffer.append({"set": [node]})
                revision = data.get("attributes", {}).get("revision", 1)
            elif kind == "upsert_edge":
                edge = self._build_edge(target, data)
                self._buffer.append({"set": [edge]})
                revision = data.get("attributes", {}).get("revision", 1)
            elif kind == "remove_node":
                self._remove_node(target)
            elif kind == "remove_edge":
                self._remove_edge(target)

        if not use_commit_now:
            elapsed = time.perf_counter() - self._last_flush_time
            if len(self._buffer) >= 100 or (self._flush_interval_s > 0 and elapsed >= self._flush_interval_s):
                self._flush()
        else:
            self._flush()

        return revision

    # ── Health check ──────────────────────────────────────────────────────────

    def health(self) -> bool:
        try:
            r = self.session.get(f"{self.url}/health", timeout=5)
            return r.status_code == 200
        except Exception:
            return False

    # ── Schema ───────────────────────────────────────────────────────────────

    def _build_schema(self) -> str:
        """Generate Dgraph schema from Golem domain model.

        Dgraph v21+ requires:
        - dgraph.type as [string] (list)
        - @index directives for predicates used in queries
        """
        # Deduplicate predicates — multiple node kinds share properties (e.g. "name")
        seen = {"id", "from", "to", "dgraph.type", "revision"}
        predicates = ['id: string @index(exact) .',
                      'from: string @index(exact) .',
                      'to: string @index(exact) .',
                      'dgraph.type: [string] @index(exact) .',
                      'revision: int .']
        for kind_def in NODE_KIND_DEFS.values():
            for p in kind_def.properties:
                if p not in seen:
                    seen.add(p)
                    predicates.append(f"{p}: string .")
        for edge_def in EDGE_TYPE_DEFS.values():
            for p in edge_def.properties:
                if p not in seen:
                    seen.add(p)
                    predicates.append(f"{p}: string .")
        return "\n".join(predicates)

    def clear_schema(self) -> None:
        """Drop all data — benchmark fresh start."""
        self.session.post(
            f"{self.url}/alter",
            json={"drop_all": True},
            timeout=30,
        )
        self.session.post(
            f"{self.url}/alter",
            json={"schema": self.schema},
            timeout=30,
        )

    def create_schema(self) -> None:
        """Create the Golem schema in Dgraph."""
        self.session.post(
            f"{self.url}/alter",
            json={"schema": self.schema},
            timeout=30,
        )

    # ── Graph operations ─────────────────────────────────────────────────────

    # (apply_ops is defined after __init__ above)

    def _flush(self) -> None:
        """Commit all buffered mutations as a single transaction."""
        if not self._buffer:
            return
        payload = {"mutations": self._buffer}
        r = self.session.post(
            f"{self.url}/mutate?commitNow=true",
            json=payload,
            timeout=60,
        )
        r.raise_for_status()
        self._buffer.clear()

    def flush(self) -> None:
        """Public flush — commit buffered mutations."""
        self._flush()

    def _build_node(self, node_id: str, data: dict) -> dict:
        kind = data.get("kind", "WorkItem")
        attrs = data.get("attributes", {})
        revision = int(attrs.get("revision", 1))
        node = {
            "id": node_id,
            "dgraph.type": [kind],
            "revision": revision,
        }
        for p, v in attrs.items():
            if p not in ("kind", "id", "revision"):
                node[p] = str(v)
        return node

    def _build_edge(self, edge_id: str, data: dict) -> dict:
        etype = data.get("type", "DEPENDS_ON")
        src = data.get("source")
        tgt = data.get("target")
        attrs = data.get("attributes", {})
        revision = int(attrs.get("revision", 1))
        edge = {
            "id": edge_id,
            "dgraph.type": [etype],
            "from": src,
            "to": tgt,
            "revision": revision,
        }
        for p, v in attrs.items():
            if p not in ("type", "id", "from", "to", "revision"):
                edge[p] = str(v)
        return edge

    def _remove_node(self, node_id: str) -> None:
        # Flush pending mutations first
        self._flush()
        # Find uid by id predicate, then delete
        r = self.session.post(
            f"{self.url}/query",
            json={"query": f'{{ node(func: eq(id, "{node_id}")) {{ uid }} }}'},
            timeout=10,
        )
        r.raise_for_status()
        data = r.json()
        nodes = data.get("data", {}).get("node", [])
        if not nodes:
            return
        uid = nodes[0]["uid"]
        self.session.post(
            f"{self.url}/mutate",
            json={"delete": [{"uid": uid}]},
            timeout=10,
        )

    def _remove_edge(self, edge_id: str) -> None:
        self._flush()
        r = self.session.post(
            f"{self.url}/query",
            json={"query": f'{{ edge(func: eq(id, "{edge_id}")) {{ uid }} }}'},
            timeout=10,
        )
        r.raise_for_status()
        data = r.json()
        edges = data.get("data", {}).get("edge", [])
        if not edges:
            return
        uid = edges[0]["uid"]
        self.session.post(
            f"{self.url}/mutate",
            json={"delete": [{"uid": uid}]},
            timeout=10,
        )

    # ── Queries ─────────────────────────────────────────────────────────────

    def neighborhood(self, root_id: str, depth: int, max_nodes: int) -> dict:
        """
        Neighborhood query: bounded BFS from root.
        Dgraph doesn't support recursive GQL traversal easily, so we use
        a simplified approach: get root + connected edges + adjacent nodes.
        """
        # Get all edges (from/to) to find neighbors
        # Since Dgraph models edges as nodes with from/to, we query those
        query = {
            "query": f'''
            {{
                root(func: eq(id, "{root_id}")) {{
                    uid
                    id
                    dgraph.type
                    revision
                }}
                # Get edges where this node is source or target
                edges(func: eq(from, "{root_id}")) {{
                    id
                    dgraph.type
                    from
                    to
                }}
            }}
            '''
        }
        r = self.session.post(f"{self.url}/query", json=query, timeout=30)
        r.raise_for_status()
        data = r.json().get("data", {})
        root_nodes = data.get("root", [])
        edges = data.get("edges", [])
        # Collect neighbor IDs from edges
        neighbor_ids = set()
        for e in edges:
            if e.get("from") == root_id and e.get("to"):
                neighbor_ids.add(e.get("to"))
            if e.get("to") == root_id and e.get("from"):
                neighbor_ids.add(e.get("from"))
        # Fetch neighbor nodes
        nodes = {}
        for nid in list(neighbor_ids)[:max_nodes]:
            node = self.get_node(nid)
            if node:
                nodes[nid] = node
        # Add root
        for rn in root_nodes:
            nid = rn.get("id", "")
            if nid and nid not in nodes:
                nodes[nid] = {
                    "id": nid,
                    "kind": rn.get("dgraph.type", ["WorkItem"])[0] if rn.get("dgraph.type") else "WorkItem",
                    "revision": rn.get("revision", 1),
                    "attributes": {},
                }
        return {"nodes": list(nodes.values()), "edges": [], "truncated": len(neighbor_ids) > max_nodes}

    def traversal(
        self,
        root_id: str,
        edge_types: list[str],
        kinds: list[str],
        depth: int,
        max_nodes: int,
        max_edges: int,
    ) -> dict:
        """Typed bounded traversal with edge-type and node-kind filters."""
        kind_filter = " | ".join(kinds) if kinds else "_all_"
        query = {
            "query": f'''
            {{
                node(func: eq(id, "{root_id}")) @recurse(depth: {depth}, first: {max_nodes}) {{
                    dgraph.type @filter(eq(dgraph.type, {" | ".join(f'"{k}"' for k in kinds)})) {{
                        dgraph.type
                    }}
                    expand(_all_) {{
                        dgraph.type @filter(eq(dgraph.type, {" | ".join(f'"{k}"' for k in edge_types)})) {{
                            dgraph.type
                        }}
                    }}
                }}
            }}
            '''
        }
        r = self.session.post(f"{self.url}/query", json=query, timeout=30)
        r.raise_for_status()
        data = r.json()
        truncated = len(data.get("data", {}).get("node", [])) >= max_nodes
        return self._gql_to_subgraph(data.get("data", {}), truncated=truncated)

    def get_node(self, node_id: str) -> dict | None:
        # Use explicit predicate list — expand(_all_) doesn't expand leaf string predicates
        query = {
            "query": f'''
            {{
                node(func: eq(id, "{node_id}")) {{
                    uid
                    id
                    dgraph.type
                    revision
                    title
                    status
                    priority
                    created_at
                    name
                    description
                    version
                    ecosystem
                    digest
                    size_bytes
                    statement
                    risk_level
                    estimate_h
                    cell_type
                    agent_class
                    effect
                    resource
                    node_type
                    url
                    weight
                    role
                    position
                    context
                    assigned_at
                    method
                }}
            }}
            '''
        }
        r = self.session.post(f"{self.url}/query", json=query, timeout=10)
        r.raise_for_status()
        data = r.json()
        result = data.get("data", {}).get("node", [])
        if not result:
            return None
        return self._dict_to_node(result[0])

    def _gql_to_subgraph(self, data: dict, truncated: bool = False) -> dict:
        """Convert Dgraph GQL result to Golem Subgraph format."""
        nodes = {}
        edges = {}
        for item in data.get("node", []):
            nid = item.get("id", "")
            kind = item.get("dgraph.type", ["WorkItem"])
            kind = kind[0] if isinstance(kind, list) else kind
            nodes[nid] = {
                "id": nid,
                "kind": kind,
                "revision": item.get("revision", 1),
                "attributes": {k: v for k, v in item.items()
                              if k not in ("uid", "id", "dgraph.type", "revision")},
            }
        return {"nodes": list(nodes.values()), "edges": list(edges.values()), "truncated": truncated}

    def list_nodes(self, tenant_id: str = "") -> list[dict]:
        """Return all node IDs for sampling. Tenant filter not used in Dgraph."""
        query = {"query": '{ nodes(func: has(id), first: 10000) { id dgraph.type } }'}
        r = self.session.post(f"{self.url}/query", json=query, timeout=30)
        r.raise_for_status()
        data = r.json()
        return data.get("data", {}).get("nodes", [])

    @staticmethod
    def _dict_to_node(d: dict) -> dict:
        kind = d.get("dgraph.type", [])
        kind = kind[0] if isinstance(kind, list) else (kind or "WorkItem")
        return {
            "id": d.get("id", ""),
            "kind": kind,
            "revision": d.get("revision", 1),
            "attributes": {k: v for k, v in d.items()
                          if k not in ("uid", "id", "dgraph.type", "revision")},
        }

    @staticmethod
    def _dict_to_edge(d: dict) -> dict:
        kind = d.get("dgraph.type", [])
        kind = kind[0] if isinstance(kind, list) else (kind or "DEPENDS_ON")
        return {
            "id": d.get("id", ""),
            "type": kind,
            "source": d.get("from", ""),
            "target": d.get("to", ""),
            "revision": d.get("revision", 1),
            "attributes": {k: v for k, v in d.items()
                          if k not in ("uid", "id", "dgraph.type", "from", "to", "revision")},
        }

    def close(self) -> None:
        self._flush()
        self.session.close()
