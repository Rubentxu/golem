#!/usr/bin/env python3
"""
HugeGraph client for Golem benchmark (ADR-086).

HugeGraph is an Apache graph DB with Gremlin traversal support.
API: HTTP REST + Gremlin over HTTP (POST to /gremlin).

Ref: https://hugegraph.github.io/hugegraph-doc/
"""

import json
import time
import uuid
from typing import Any

import requests

from .domain import (
    NODE_KIND_DEFS,
    EDGE_TYPE_DEFS,
    NodeKind,
    EdgeType,
    SCHEMA_STRATEGY,
)


class HugeGraphClient:
    """HugeGraph Gremlin client mapped to Golem graph operations."""

    BASE_URL: str

    def __init__(self, base_url: str):
        self.BASE_URL = base_url.rstrip("/")
        self.session = requests.Session()
        self.session.headers["Content-Type"] = "application/json"

    # ── Health check ──────────────────────────────────────────────────────────

    def health(self) -> bool:
        try:
            r = self.session.get(f"{self.BASE_URL}/graphs/hugegraph")
            return r.status_code == 200
        except Exception:
            return False

    # ── Schema management ─────────────────────────────────────────────────────

    def clear_schema(self) -> None:
        """Drop all vertices and edges — benchmark fresh start."""
        gremlin = """
        g.V().drop().iterate();
        """
        self._gremlin(gremlin)

    def create_schema(self) -> None:
        """Create Golem graph schema: property keys + vertex labels + edge labels."""
        # Property keys first
        all_props = set()
        for kind in NODE_KIND_DEFS.values():
            all_props.update(kind.properties)
        for edge in EDGE_TYPE_DEFS.values():
            all_props.update(edge.properties)
        all_props.update(["tenant_id", "id", "kind", "type", "revision"])

        for prop in sorted(all_props):
            if not prop:
                continue
            payload = {
                "name": prop,
                "data_type": "TEXT",
                "cardinality": "SINGLE",
            }
            try:
                self.session.post(
                    f"{self.BASE_URL}/graphs/hugegraph/schema/propertykeys",
                    json=payload,
                    timeout=10,
                )
            except Exception:
                pass  # already exists

        # Vertex labels
        for name in NODE_KIND_DEFS:
            payload = {"name": name, "id_strategy": "PRIMARY_KEY", "primary_keys": ["id"]}
            try:
                self.session.post(
                    f"{self.BASE_URL}/graphs/hugegraph/schema/vertexlabels",
                    json=payload,
                    timeout=10,
                )
            except Exception:
                pass  # already exists

        # Edge labels (bidirectional by default for HugeGraph)
        for name in EDGE_TYPE_DEFS:
            payload = {
                "name": name,
                "source_label": name,
                "target_label": name,
                "frequency": "SINGLE",
            }
            try:
                self.session.post(
                    f"{self.BASE_URL}/graphs/hugegraph/schema/edgelabels",
                    json=payload,
                    timeout=10,
                )
            except Exception:
                pass  # already exists

    # ── Gremlin helpers ───────────────────────────────────────────────────────

    def _gremlin(self, script: str, bindings: dict | None = None) -> Any:
        payload = {"gremlin": script}
        if bindings:
            payload["bindings"] = bindings
        r = self.session.post(
            f"{self.BASE_URL}/gremlin",
            json=payload,
            timeout=60,
        )
        r.raise_for_status()
        return r.json().get("result", {}).get("data", [])

    # ── Graph operations (Golem interface) ────────────────────────────────────

    def apply_ops(self, ops: list[dict]) -> int:
        """
        Apply a batch of Golem graph ops.
        Returns the final revision (count of ops applied).
        revision starts at 1.
        """
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
        props = {"id": node_id, "kind": kind, "tenant_id": "bench", **attrs}
        script = f'g.mergeV({self._props(props)}).option(T.id, "{node_id}").option(T.label, "{kind}")'
        self._gremlin(script)
        # fetch revision (attribute merge increments counter — simplified here)
        return revision_from_props(attrs)

    def _upsert_edge(self, edge_id: str, data: dict) -> int:
        etype = data.get("type", "DEPENDS_ON")
        src = data.get("source")
        tgt = data.get("target")
        attrs = data.get("attributes", {})
        props = {"id": edge_id, "type": etype, **attrs}
        script = (
            f'g.V("{src}").as("s").V("{tgt}").as("t")'
            f'.mergeE({self._props(props)})'
            f'.option(T.id, "{edge_id}")'
            f'.option(T.from, "s").option(T.to, "t")'
        )
        self._gremlin(script)
        return revision_from_props(attrs)

    def _remove_node(self, node_id: str) -> None:
        self._gremlin(f'g.V("{node_id}").drop()')

    def _remove_edge(self, edge_id: str) -> None:
        self._gremlin(f'g.E("{edge_id}").drop()')

    @staticmethod
    def _props(props: dict) -> str:
        """Serialize dict to Gremlin map string."""
        entries = [f'"{k}":"{v}"' if isinstance(v, str) else f'"{k}":{v}'
                   for k, v in props.items()]
        return "{" + ",".join(entries) + "}"

    # ── Queries ───────────────────────────────────────────────────────────────

    def neighborhood(self, root_id: str, depth: int, max_nodes: int) -> dict:
        """
        Neighborhood query: bounded BFS from root.
        Returns subgraph as dict with nodes/edges.
        """
        script = (
            f'g.V("{root_id}")'
            f'.repeat(__.bothE().bothV().dedup())'
            f".times({depth})"
            f".limit({max_nodes})"
            f'.path()'
        )
        result = self._gremlin(script)
        return self._paths_to_subgraph([result[-1]] if result else [])

    def traversal(
        self,
        root_id: str,
        edge_types: list[str],
        kinds: list[str],
        depth: int,
        max_nodes: int,
        max_edges: int,
    ) -> dict:
        """
        Typed bounded traversal with edge-type and node-kind filters.
        Returns subgraph with Truncated flag.
        """
        et_filter = (
            f'.filter{{it.get().label() in {edge_types}}}'
            if edge_types
            else ""
        )
        script = (
            f'g.V("{root_id}")'
            f'.repeat('
            f'  __.bothE(){et_filter}.bothV()'
            f'  .filter{{it.property("kind").value() in {kinds} if {bool(kinds)} else true}}'
            f')'
            f".times({depth})"
            f".limit({max_nodes})"
            f'.path()'
        )
        raw = self._gremlin(script)
        truncated = len(raw) >= max_nodes or len(raw) >= max_edges
        return self._paths_to_subgraph(raw, truncated=truncated)

    def get_node(self, node_id: str) -> dict | None:
        script = f'g.V("{node_id}").valueMap(true)'
        try:
            result = self._gremlin(script)
            if not result:
                return None
            return self._map_to_node(result[0])
        except Exception:
            return None

    def _paths_to_subgraph(self, paths: list, truncated: bool = False) -> dict:
        """Convert Gremlin path results to Golem Subgraph format."""
        nodes = {}
        edges = {}
        for path in paths:
            for item in path.objects if hasattr(path, "objects") else path:
                label = item.get("label", "")
                props = item if isinstance(item, dict) else {}
                if label in NODE_KIND_DEFS:
                    nodes[props.get("id", "")] = self._map_to_node(props)
                elif label in EDGE_TYPE_DEFS:
                    edges[props.get("id", "")] = self._map_to_edge(props)
        return {
            "nodes": list(nodes.values()),
            "edges": list(edges.values()),
            "truncated": truncated,
        }

    @staticmethod
    def _map_to_node(props: dict) -> dict:
        kind = props.get("kind", "WorkItem")
        revision = props.get("revision", 1)
        return {
            "id": props.get("id", ""),
            "kind": kind,
            "revision": revision,
            "attributes": {k: v for k, v in props.items()
                          if k not in ("id", "kind", "revision", "tenant_id")},
        }

    @staticmethod
    def _map_to_edge(props: dict) -> dict:
        return {
            "id": props.get("id", ""),
            "type": props.get("type", "DEPENDS_ON"),
            "source": props.get("source", ""),
            "target": props.get("target", ""),
            "revision": props.get("revision", 1),
            "attributes": {k: v for k, v in props.items()
                          if k not in ("id", "type", "source", "target", "revision")},
        }

    def close(self) -> None:
        self.session.close()


# ── Helpers ─────────────────────────────────────────────────────────────────

def revision_from_props(attrs: dict) -> int:
    """Extract or generate revision from node/edge attributes."""
    return int(attrs.get("revision", 1))
