#!/usr/bin/env python3
"""
Dgraph client for Golem benchmark (ADR-086).

Dgraph is a native GraphQL graph DB with low-latency queries.
API: HTTP endpoint at :8080 (admin + mutations), :9080 (gRPC, not used here).

Dgraph uses GraphQL± for mutations (upsert via upsertBlock) and
GraphQL for queries (traversal via has() and expand()).

Ref: https://dgraph.io/docs/
"""

import json
import uuid
from typing import Any

import requests

from .domain import NODE_KIND_DEFS, EDGE_TYPE_DEFS


class DgraphClient:
    """Dgraph GraphQL± client mapped to Golem graph operations."""

    def __init__(self, url: str):
        # url is http://host:8080
        self.url = url.rstrip("/")
        self.admin_url = f"{self.url}/admin"
        self.session = requests.Session()
        self.session.headers["Content-Type"] = "application/json"
        self.schema = self._build_schema()

    # ── Health check ──────────────────────────────────────────────────────────

    def health(self) -> bool:
        try:
            r = self.session.get(f"{self.url}/health", timeout=5)
            return r.status_code == 200
        except Exception:
            return False

    # ── Schema ────────────────────────────────────────────────────────────────

    def _build_schema(self) -> str:
        """Generate Dgraph GraphQL schema from Golem domain model."""
        types = []
        for name, kind_def in NODE_KIND_DEFS.items():
            props = ["id: string!", "dgraph.type: string!"]
            for p in kind_def.properties:
                props.append(f"{p}: string")
            props.append("revision: int")
            types.append(f"type {name} {{ {' '.join(props)} }}")

        for name, edge_def in EDGE_TYPE_DEFS.items():
            props = ["id: string!", "from: string!", "to: string!", "dgraph.type: string!"]
            for p in edge_def.properties:
                props.append(f"{p}: string")
            props.append("revision: int")
            # Dgraph models edges as nodes with from/to references
            types.append(f"type {name} {{ {' '.join(props)} }}")

        return "\n".join(types)

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

        # Build NQuad mutation
        nquads = [
            f'<{node_id}> <dgraph.type> "{kind}" .',
            f'<{node_id}> <id> "{node_id}" .',
        ]
        for p, v in attrs.items():
            if p not in ("kind", "id", "revision"):
                nquads.append(f'<{node_id}> <{p}> "{v}" .')
        nquads.append(f'<{node_id}> <revision> "{revision}" .')

        payload = {"mutations": [{"commit_now": True}], "query": ""}
        for nq in nquads:
            payload["mutations"][0].setdefault("set_nquads", [])
            payload["mutations"][0]["set_nquads"].append(nq)

        r = self.session.post(
            f"{self.url}/mutate?commitNow=true",
            json=payload,
            timeout=30,
        )
        r.raise_for_status()
        return revision

    def _upsert_edge(self, edge_id: str, data: dict) -> int:
        etype = data.get("type", "DEPENDS_ON")
        src = data.get("source")
        tgt = data.get("target")
        attrs = data.get("attributes", {})
        revision = int(attrs.get("revision", 1))

        # Dgraph models edges as separate nodes with from/to predicates
        nquads = [
            f'<{edge_id}> <dgraph.type> "{etype}" .',
            f'<{edge_id}> <id> "{edge_id}" .',
            f'<{edge_id}> <from> "{src}" .',
            f'<{edge_id}> <to> "{tgt}" .',
            f'<{edge_id}> <revision> "{revision}" .',
        ]
        for p, v in attrs.items():
            if p not in ("type", "id", "from", "to", "revision"):
                nquads.append(f'<{edge_id}> <{p}> "{v}" .')

        r = self.session.post(
            f"{self.url}/mutate?commitNow=true",
            json={"set_nquads": nquads},
            timeout=30,
        )
        r.raise_for_status()
        return revision

    def _remove_node(self, node_id: str) -> None:
        self.session.post(
            f"{self.url}/mutate?commitNow=true",
            json={"delete_nquads": f'<{node_id}> * .'},
            timeout=30,
        )

    def _remove_edge(self, edge_id: str) -> None:
        self.session.post(
            f"{self.url}/mutate?commitNow=true",
            json={"delete_nquads": f'<{edge_id}> * .'},
            timeout=30,
        )

    # ── Queries ───────────────────────────────────────────────────────────────

    def neighborhood(self, root_id: str, depth: int, max_nodes: int) -> dict:
        """
        Dgraph doesn't have native multi-hop traversal via GraphQL.
        Use expand() keyword with recursive query.
        """
        # Using GraphQL query (Dgraph v21+):
        query = {
            "query": f"""
            {{
                var(func: uid("{root_id}")) {{
                    expand(_all_) {{
                        expand(_all_) {{
                            uid
                            id: id
                            dgraph.type
                            revision
                        }}
                    }}
                }}
                result(func: uid("{root_id}")) {{
                    uid
                    id: id
                    dgraph.type
                    revision
                    expand(_all_) {{
                        uid
                        id: id
                        dgraph.type
                        revision
                    }}
                }}
            }}
            """
        }
        r = self.session.post(f"{self.url}/query", json=query, timeout=60)
        r.raise_for_status()
        data = r.json()
        # Parse result — simplified; real implementation needs full recursive parse
        result = data.get("result", []) or data.get("data", {}).get("result", [])
        nodes = []
        edges = []
        for item in (result if isinstance(result, list) else [result]):
            if isinstance(item, dict):
                dtype = item.get("dgraph.type", "")
                if dtype in NODE_KIND_DEFS:
                    nodes.append(self._dict_to_node(item))
                elif dtype in EDGE_TYPE_DEFS:
                    edges.append(self._dict_to_edge(item))
        return {"nodes": nodes[:max_nodes], "edges": edges, "truncated": len(nodes) > max_nodes}

    def traversal(
        self,
        root_id: str,
        edge_types: list[str],
        kinds: list[str],
        depth: int,
        max_nodes: int,
        max_edges: int,
    ) -> dict:
        """Typed traversal with edge-type and kind filters."""
        et_filter = " | ".join(edge_types) if edge_types else "_ALL_"
        kind_filter = " | ".join(kinds) if kinds else "_ALL_"

        query = {
            "query": f"""
            {{
                result(func: uid("{root_id}")) @recurse(depth: {depth}, loop: false) {{
                    uid
                    id: id
                    dgraph.type
                    revision
                    P as dgraph.type @filter(eq(dgraph.type, "{et_filter}"))
                }}
            }}
            """
        }
        r = self.session.post(f"{self.url}/query", json=query, timeout=60)
        r.raise_for_status()
        data = r.json()
        result = data.get("result", []) or data.get("data", {}).get("result", [])
        nodes = {}
        edges = {}
        self._collect_from_result(result, nodes, edges)
        truncated = len(nodes) >= max_nodes or len(edges) >= max_edges
        return {
            "nodes": list(nodes.values())[:max_nodes],
            "edges": list(edges.values())[:max_edges],
            "truncated": truncated,
        }

    def _collect_from_result(self, items, nodes: dict, edges: dict) -> None:
        for item in items if isinstance(items, list) else [items]:
            if not isinstance(item, dict):
                continue
            dtype = item.get("dgraph.type", "")
            if dtype in NODE_KIND_DEFS:
                nodes[item.get("id", "")] = self._dict_to_node(item)
            elif dtype in EDGE_TYPE_DEFS:
                edges[item.get("id", "")] = self._dict_to_edge(item)

    def get_node(self, node_id: str) -> dict | None:
        query = {
            "query": f'''
            query {{
                node(func: eq(id, "{node_id}")) {{
                    uid
                    id: id
                    dgraph.type
                    revision
                }}
            }}
            '''
        }
        r = self.session.post(f"{self.url}/query", json=query, timeout=10)
        r.raise_for_status()
        data = r.json()
        result = data.get("node", [])
        if not result:
            return None
        return self._dict_to_node(result[0])

    @staticmethod
    def _dict_to_node(d: dict) -> dict:
        return {
            "id": d.get("id", ""),
            "kind": d.get("dgraph.type", "WorkItem"),
            "revision": int(d.get("revision", 1)),
            "attributes": {k: v for k, v in d.items()
                          if k not in ("uid", "id", "dgraph.type", "revision")},
        }

    @staticmethod
    def _dict_to_edge(d: dict) -> dict:
        return {
            "id": d.get("id", ""),
            "type": d.get("dgraph.type", "DEPENDS_ON"),
            "source": d.get("from", ""),
            "target": d.get("to", ""),
            "revision": int(d.get("revision", 1)),
            "attributes": {k: v for k, v in d.items()
                          if k not in ("uid", "id", "dgraph.type", "from", "to", "revision")},
        }

    def close(self) -> None:
        self.session.close()
