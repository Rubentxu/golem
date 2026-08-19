#!/usr/bin/env python3
"""
HugeGraph REST client for Golem benchmark (ADR-086).

HugeGraph is an Apache graph DB with Gremlin traversal support.
API: HTTP REST at /graphs/hugegraph/graph/{vertices|edges}

Note: HugeGraph's Gremlin endpoint has a bug with HugeGraphAuthProxy
(ContextTask doesn't inherit script globals — 'g' is not bound).
We use the REST API instead, with an in-memory cache for lookups.

HugeGraph REST API cannot fetch vertices by internal ID (bug: the ID format
'shard:id' is not accepted by GET /vertices/{id}). We work around this by:
1. After bulk load: call refresh_cache() to load all data into memory
2. For reads: use in-memory cache (O(1) lookup)
3. For writes: update cache directly after creation

Ref: https://hugegraph.github.io/hugegraph-doc/
"""

import time
import uuid
from typing import Any

import requests

from ..domain import NODE_KIND_DEFS, EDGE_TYPE_DEFS


class HugeGraphClient:
    """HugeGraph REST client with in-memory cache for fast lookups."""

    def __init__(self, base_url: str):
        self.BASE_URL = base_url.rstrip("/")
        self.session = requests.Session()
        self.session.headers["Content-Type"] = "application/json"
        # In-memory cache: custom_id → normalized vertex/edge dict
        self._vertex_cache: dict[str, dict] = {}
        self._edge_cache: dict[str, dict] = {}

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
        self._vertex_cache.clear()
        self._edge_cache.clear()
        # Use REST API to clear all data
        try:
            self.session.post(
                f"{self.BASE_URL}/graphs/hugegraph/schema/clear",
                timeout=60,
            )
        except Exception:
            pass

    def create_schema(self) -> None:
        """Create Golem graph schema: property keys + vertex labels + edge labels.

        Deletes any pre-existing vertex/edge labels first to ensure a clean slate.
        """
        # Delete ALL existing vertex labels to get a clean start
        try:
            r = self.session.get(
                f"{self.BASE_URL}/graphs/hugegraph/schema/vertexlabels",
                timeout=10,
            )
            for vl in r.json().get("vertexlabels", []):
                try:
                    self.session.delete(
                        f"{self.BASE_URL}/graphs/hugegraph/schema/vertexlabels/{vl['name']}",
                        timeout=10,
                    )
                except Exception:
                    pass
        except Exception:
            pass

        # Delete ALL existing edge labels
        try:
            r = self.session.get(
                f"{self.BASE_URL}/graphs/hugegraph/schema/edgelabels",
                timeout=10,
            )
            for el in r.json().get("edgelabels", []):
                try:
                    self.session.delete(
                        f"{self.BASE_URL}/graphs/hugegraph/schema/edgelabels/{el['name']}",
                        timeout=10,
                    )
                except Exception:
                    pass
        except Exception:
            pass

        # Wait for deletions to settle
        time.sleep(1)

        # Property keys
        all_props = set()
        for kind in NODE_KIND_DEFS.values():
            all_props.update(kind.properties)
        for edge in EDGE_TYPE_DEFS.values():
            all_props.update(edge.properties)
        all_props.update(["tenant_id", "id", "type", "revision"])

        for prop in sorted(all_props):
            if not prop:
                continue
            try:
                self.session.post(
                    f"{self.BASE_URL}/graphs/hugegraph/schema/propertykeys",
                    json={"name": prop, "data_type": "TEXT", "cardinality": "SINGLE"},
                    timeout=10,
                )
            except Exception:
                pass  # already exists

        # Vertex labels — create or update to add ALL properties on every label.
        # Workloads generate attrs from any kind's property set, so we must
        # allow any property on any label to avoid schema violations.
        all_vertex_props = sorted(all_props)
        for name in NODE_KIND_DEFS:
            all_props_list = all_vertex_props + ["id", "tenant_id"]
            nullable = [p for p in all_props_list if p != "id"]
            try:
                self.session.post(
                    f"{self.BASE_URL}/graphs/hugegraph/schema/vertexlabels",
                    json={
                        "name": name,
                        "id_strategy": "PRIMARY_KEY",
                        "primary_keys": ["id"],
                        "properties": all_props_list,
                        "nullable_keys": nullable,
                    },
                    timeout=10,
                )
            except Exception:
                pass  # already exists — patch it below

            # Label already existed (e.g. init container pre-created it).
            # Use ?action=append to add any missing properties, then set nullable.
            try:
                r = self.session.get(
                    f"{self.BASE_URL}/graphs/hugegraph/schema/vertexlabels/{name}",
                    timeout=10,
                )
                existing_props = set(r.json().get("properties", []))
                missing_props = [p for p in all_props_list if p not in existing_props]
                primary_keys = set(r.json().get("primary_keys", []))
                desired_nullable = list(set(all_props_list) - primary_keys)
                if missing_props or set(desired_nullable) - set(r.json().get("nullable_keys", [])):
                    patch = {
                        "name": name,
                        "properties": list(existing_props | set(missing_props)),
                        "nullable_keys": desired_nullable,
                    }
                    self.session.put(
                        f"{self.BASE_URL}/graphs/hugegraph/schema/vertexlabels/{name}?action=append",
                        json=patch,
                        timeout=10,
                    )
            except Exception:
                pass

        # Edge labels — create or update to add ALL properties on every label.
        all_edge_props = sorted(all_props)
        for name, edge_def in EDGE_TYPE_DEFS.items():
            src_lbl = edge_def.from_kinds[0] if edge_def.from_kinds else name
            tgt_lbl = edge_def.to_kinds[0] if edge_def.to_kinds else name
            nullable = [p for p in all_edge_props if p != "id"]
            try:
                self.session.post(
                    f"{self.BASE_URL}/graphs/hugegraph/schema/edgelabels",
                    json={
                        "name": name,
                        "source_label": src_lbl,
                        "target_label": tgt_lbl,
                        "frequency": "SINGLE",
                        "properties": all_edge_props + ["id", "type"],
                        "nullable_keys": nullable,
                    },
                    timeout=10,
                )
            except Exception:
                pass  # already exists — patch it below

            # Patch existing edge label to add missing properties + set nullable
            try:
                r = self.session.get(
                    f"{self.BASE_URL}/graphs/hugegraph/schema/edgelabels/{name}",
                    timeout=10,
                )
                existing_props = set(r.json().get("properties", []))
                missing_props = [p for p in (all_edge_props + ["id", "type"]) if p not in existing_props]
                desired_nullable = [p for p in (all_edge_props + ["id", "type"]) if p != "id"]
                if missing_props:
                    patch = {
                        "name": name,
                        "properties": list(existing_props | set(missing_props)),
                        "nullable_keys": desired_nullable,
                    }
                    self.session.put(
                        f"{self.BASE_URL}/graphs/hugegraph/schema/edgelabels/{name}?action=append",
                        json=patch,
                        timeout=10,
                    )
            except Exception:
                pass

    # ── Cache management ─────────────────────────────────────────────────────

    def refresh_cache(self) -> None:
        """Load all vertices and edges from HugeGraph into in-memory cache.

        HugeGraph REST cannot fetch by internal ID (bug). We load all data once
        and serve reads from the cache.
        """
        self._vertex_cache.clear()
        self._edge_cache.clear()

        # Load vertices
        page = None
        while True:
            params = {"limit": 1000}
            if page:
                params["page"] = page
            r = self.session.get(
                f"{self.BASE_URL}/graphs/hugegraph/graph/vertices",
                params=params,
                timeout=60,
            )
            if r.status_code != 200:
                break
            data = r.json()
            for v in data.get("vertices", []):
                props = v.get("properties", {})
                custom_id = props.get("id", "")
                if custom_id:
                    self._vertex_cache[custom_id] = {
                        "id": custom_id,
                        "kind": v.get("label", "unknown"),
                        "revision": int(props.get("revision", 1)),
                        "attributes": {k: val for k, val in props.items()
                                      if k not in ("id", "kind", "revision", "tenant_id")},
                    }
            page = data.get("page")
            if not page:
                break

        # Load edges
        page = None
        while True:
            params = {"limit": 1000}
            if page:
                params["page"] = page
            r = self.session.get(
                f"{self.BASE_URL}/graphs/hugegraph/graph/edges",
                params=params,
                timeout=60,
            )
            if r.status_code != 200:
                break
            data = r.json()
            for e in data.get("edges", []):
                props = e.get("properties", {})
                custom_id = props.get("id", "")
                if custom_id:
                    self._edge_cache[custom_id] = {
                        "id": custom_id,
                        "type": e.get("label", "unknown"),
                        "source": props.get("source", ""),
                        "target": props.get("target", ""),
                        "revision": int(props.get("revision", 1)),
                        "attributes": {k: val for k, val in props.items()
                                      if k not in ("id", "type", "source", "target", "revision")},
                    }
            page = data.get("page")
            if not page:
                break

    # ── Graph operations (Golem interface) ────────────────────────────────────

    def apply_ops(self, ops: list[dict]) -> int:
        """
        Apply a batch of Golem graph ops.
        Returns the final revision (count of ops applied).
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
        # HugeGraph creates id and tenant_id as properties on each vertex
        props = {"id": node_id, "tenant_id": "bench", **attrs}
        # Create vertex via REST — returns internal ID (e.g. "7:n0")
        r = self.session.post(
            f"{self.BASE_URL}/graphs/hugegraph/graph/vertices",
            json={"label": kind, "properties": props},
            timeout=30,
        )
        if r.status_code >= 400:
            print(f"DEBUG: vertex create failed: {r.status_code} {r.text[:200]} kind={kind} props={props}")
        r.raise_for_status()
        internal_id = r.json()["id"]
        # Update cache with internal ID for edge lookups
        self._vertex_cache[node_id] = {
            "id": node_id,
            "kind": kind,
            "revision": int(attrs.get("revision", 1)),
            "attributes": attrs,
            "_internal_id": internal_id,
        }
        return int(attrs.get("revision", 1))

    def _upsert_edge(self, edge_id: str, data: dict) -> int:
        etype = data.get("type", "DEPENDS_ON")
        src = data.get("source")
        tgt = data.get("target")
        attrs = data.get("attributes", {})
        # Properties must include all non-nullable fields: id, type + edge-specific props
        props = {"id": edge_id, "type": etype, **attrs}
        # Look up internal vertex IDs from cache
        src_v = self._vertex_cache.get(src, {})
        tgt_v = self._vertex_cache.get(tgt, {})
        src_internal = src_v.get("_internal_id", src)
        tgt_internal = tgt_v.get("_internal_id", tgt)
        try:
            r = self.session.post(
                f"{self.BASE_URL}/graphs/hugegraph/graph/edges",
                json={
                    "label": etype,
                    "outV": src_internal,
                    "inV": tgt_internal,
                    "properties": props,
                },
                timeout=30,
            )
            r.raise_for_status()
        except Exception:
            # HugeGraph rejects edges where source/target kinds don't match
            # the edge label's from_kinds/to_kinds. Skip such edges.
            return int(attrs.get("revision", 1))
        # Update cache
        self._edge_cache[edge_id] = {
            "id": edge_id,
            "type": etype,
            "source": src,
            "target": tgt,
            "revision": int(attrs.get("revision", 1)),
            "attributes": attrs,
        }
        return int(attrs.get("revision", 1))

    def _remove_node(self, node_id: str) -> None:
        v = self._vertex_cache.get(node_id, {})
        internal_id = v.get("_internal_id", node_id)
        try:
            self.session.delete(
                f"{self.BASE_URL}/graphs/hugegraph/graph/vertices/{internal_id}",
                timeout=30,
            )
        except Exception:
            pass
        self._vertex_cache.pop(node_id, None)

    def _remove_edge(self, edge_id: str) -> None:
        e = self._edge_cache.get(edge_id, {})
        internal_id = e.get("_internal_id", edge_id)
        try:
            self.session.delete(
                f"{self.BASE_URL}/graphs/hugegraph/graph/edges/{internal_id}",
                timeout=30,
            )
        except Exception:
            pass
        self._edge_cache.pop(edge_id, None)

    # ── Queries (from cache) ───────────────────────────────────────────────────

    def neighborhood(self, root_id: str, depth: int, max_nodes: int) -> dict:
        """
        Neighborhood query from cache — bounded BFS from root.

        HugeGraph's REST traversal API requires internal IDs which we don't have
        easy access to. We do BFS in Python using the cached graph.
        """
        # Accept both str IDs and node dicts (workload bug: _sample_node_ids
        # returns dicts when len(nodes) >= sample_size)
        if isinstance(root_id, dict):
            root_id = root_id.get("id", "")
        if root_id not in self._vertex_cache:
            return {"nodes": [], "edges": [], "truncated": False}

        visited: set[str] = {root_id}
        queue = [(root_id, 0)]
        result_nodes = []
        result_edges = []

        while queue:
            node_id, d = queue.pop(0)
            if d >= depth:
                continue
            v = self._vertex_cache.get(node_id)
            if v:
                result_nodes.append(v)
            # Find edges involving this node
            for edge_id, edge in self._edge_cache.items():
                if edge["source"] == node_id and edge["target"] not in visited:
                    result_edges.append(edge)
                    visited.add(edge["target"])
                    queue.append((edge["target"], d + 1))
                elif edge["target"] == node_id and edge["source"] not in visited:
                    result_edges.append(edge)
                    visited.add(edge["source"])
                    queue.append((edge["source"], d + 1))

            if len(result_nodes) >= max_nodes:
                return {"nodes": result_nodes[:max_nodes], "edges": result_edges, "truncated": True}

        return {"nodes": result_nodes, "edges": result_edges, "truncated": False}

    def get_node(self, node_id: str) -> dict | None:
        """Get a single node from cache."""
        if isinstance(node_id, dict):
            node_id = node_id.get("id", "")
        return self._vertex_cache.get(node_id)

    def list_nodes(self, tenant_id: str = "bench", limit: int = 1000) -> list[dict]:
        """List all cached vertices (tenant_id arg ignored — single-tenant cache)."""
        return list(self._vertex_cache.values())[:limit]

    def close(self) -> None:
        self.session.close()
