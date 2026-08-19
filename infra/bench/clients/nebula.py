#!/usr/bin/env python3
"""
NebulaGraph client for Golem benchmark (ADR-086).

NebulaGraph is a distributed graph DB with nGQL query language.
API: HTTP console on port 9669 (Thrift) or HTTP API.

For benchmark we use the HTTP API (nebula-graph-httpapi) or the Python client.
Ref: https://docs.nebula-graph.io/
"""

import json
import uuid
from typing import Any

import requests

from ..domain import NODE_KIND_DEFS, EDGE_TYPE_DEFS


class NebulaGraphClient:
    """NebulaGraph nGQL client mapped to Golem graph operations."""

    def __init__(self, url: str):
        # url is http://host:9669
        self.url = url.rstrip("/")
        self.session = requests.Session()
        self.session.headers["Content-Type"] = "application/json"
        self.space = "golem_bench"

    # ── Health check ──────────────────────────────────────────────────────────

    def health(self) -> bool:
        try:
            r = self.session.get(f"{self.url}/status", timeout=5)
            return r.status_code == 200
        except Exception:
            return False

    # ── Schema management ─────────────────────────────────────────────────────

    def clear_schema(self) -> None:
        """Drop the space and recreate it — benchmark fresh start."""
        self._ngql(f"DROP SPACE IF EXISTS {self.space}")
        self._ngql(
            f"CREATE SPACE {self.space}"
            f"(partition_num=1, replica_factor=1, vid_type=FIXED_STRING(32))"
        )
        self._ngql(f"USE {self.space}")
        self._create_schema()

    def _create_schema(self) -> None:
        """Create node kinds as tags and edge types as edge types."""
        for name, kind_def in NODE_KIND_DEFS.items():
            props = ["id string"] + [f"{p} string" for p in kind_def.properties]
            props_str = ",".join(props)
            self._ngql(f'CREATE TAG IF NOT EXISTS {name}({props_str})')

        for name, edge_def in EDGE_TYPE_DEFS.items():
            props = ["id string"] + [f"{p} string" for p in edge_def.properties]
            props_str = ",".join(props)
            self._ngql(f'CREATE EDGE IF NOT EXISTS {name}({props_str})')

    # ── nGQL executor ─────────────────────────────────────────────────────────

    def _ngql(self, query: str) -> Any:
        """Execute nGQL query via HTTP API."""
        payload = {"query": query, "params": {}}
        r = self.session.post(
            f"{self.url}/api/v1/cypher",
            json=payload,
            timeout=30,
        )
        r.raise_for_status()
        data = r.json()
        if errors := data.get("errors"):
            raise RuntimeError(errors)
        return data.get("results", [{}])[0].get("rows", [])

    # ── Graph operations ──────────────────────────────────────────────────────

    def apply_ops(self, ops: list[dict]) -> int:
        revision = 0
        for op in ops:
            kind = op.get("kind")
            target = op.get("target")
            data = op.get("data", {})

            if kind == "upsert_node":
                revision = self._upsert_node(target, data)
            elif kind == "upsert_edge":
                revision = self._upsert_edge(target, data)
            elif kind == "remove_node":
                self._remove_node(target)
            elif kind == "remove_edge":
                self._remove_edge(target)
        return revision

    def _upsert_node(self, node_id: str, data: dict) -> int:
        kind = data.get("kind", "WorkItem")
        attrs = data.get("attributes", {})
        revision = int(attrs.get("revision", 1))
        props = [f'"{node_id}"'] + [f'"{attrs.get(p, "")}"' for p in NODE_KIND_DEFS[kind].properties]
        vals = ",".join(props)
        self._ngql(f'INSERT VERTEX {kind}(id,{"".join([p for p in NODE_KIND_DEFS[kind].properties])}) VALUES "{node_id}":({vals})')
        return revision

    def _upsert_edge(self, edge_id: str, data: dict) -> int:
        etype = data.get("type", "DEPENDS_ON")
        src = data.get("source")
        tgt = data.get("target")
        attrs = data.get("attributes", {})
        revision = int(attrs.get("revision", 1))
        self._ngql(
            f'INSERT EDGE {etype}(id) VALUES "{src}"->"{tgt}":("{edge_id}")'
        )
        return revision

    def _remove_node(self, node_id: str) -> None:
        self._ngql(f'DELETE VERTEX "{node_id}"')

    def _remove_edge(self, edge_id: str) -> None:
        # NebulaGraph doesn't support delete by edge ID directly — use index or MATCH
        self._ngql(f'DELETE EDGE {edge_id}')

    # ── Queries ───────────────────────────────────────────────────────────────

    def neighborhood(self, root_id: str, depth: int, max_nodes: int) -> dict:
        """BFS neighborhood query."""
        # nGQL doesn't have native Gremlin-style repeat+limit on paths
        # Use GET SUBGRAPH instead
        result = self._ngql(
            f"GET SUBGRAPH 1 STEPS FROM \"{root_id}\" YIELD VERTICES, EDGES"
        )
        return self._subgraph_result(result)

    def traversal(
        self,
        root_id: str,
        edge_types: list[str],
        kinds: list[str],
        depth: int,
        max_nodes: int,
        max_edges: int,
    ) -> dict:
        """Typed traversal with filters."""
        et_list = "|".join(edge_types) if edge_types else "*"
        result = self._ngql(
            f"MATCH (start)-[e:{et_list}*1..{depth}]-(n) "
            f"WHERE id(start) == '{root_id}' "
            f"RETURN start, e, n LIMIT {max_nodes}"
        )
        return self._match_result(result)

    def get_node(self, node_id: str) -> dict | None:
        result = self._ngql(f'LOOKUP ON WorkItem WHERE WorkItem.id == "{node_id}" YIELD properties(Vertex)')
        if not result:
            return None
        return self._row_to_node(result[0])

    def _subgraph_result(self, rows: list) -> dict:
        nodes = []
        edges = []
        for row in rows:
            if len(row) >= 2:
                vertices = row[0] if isinstance(row[0], list) else [row[0]]
                edge_list = row[1] if isinstance(row[1], list) else [row[1]]
                for v in vertices:
                    nodes.append(self._vertex_to_node(v))
                for e in edge_list:
                    edges.append(self._edge_to_edge(e))
        return {"nodes": nodes, "edges": edges, "truncated": False}

    def _match_result(self, rows: list) -> dict:
        nodes = {}
        edges = {}
        for row in rows:
            if len(row) >= 3:
                start = row[0]
                edge_list = row[1] if isinstance(row[1], list) else [row[1]]
                end = row[2]
                nodes[start.get("id", "")] = self._vertex_to_node(start)
                nodes[end.get("id", "")] = self._vertex_to_node(end)
                for e in edge_list:
                    edges[e.get("id", "")] = self._edge_to_edge(e)
        return {
            "nodes": list(nodes.values()),
            "edges": list(edges.values()),
            "truncated": len(nodes) >= 100 or len(edges) >= 100,
        }

    @staticmethod
    def _vertex_to_node(v: dict) -> dict:
        props = v.get("properties", {})
        return {
            "id": v.get("id", ""),
            "kind": v.get("label", "WorkItem"),
            "revision": int(props.get("revision", 1)),
            "attributes": {k: val for k, val in props.items()
                          if k not in ("id", "kind", "revision")},
        }

    @staticmethod
    def _edge_to_edge(e: dict) -> dict:
        props = e.get("properties", {})
        return {
            "id": e.get("id", ""),
            "type": e.get("label", "DEPENDS_ON"),
            "source": e.get("src", ""),
            "target": e.get("dst", ""),
            "revision": int(props.get("revision", 1)),
            "attributes": props,
        }

    @staticmethod
    def _row_to_node(row: dict) -> dict:
        props = row.get("properties(Vertex)", {})
        return {
            "id": props.get("id", ""),
            "kind": props.get("kind", "WorkItem"),
            "revision": int(props.get("revision", 1)),
            "attributes": props,
        }

    def close(self) -> None:
        self.session.close()
