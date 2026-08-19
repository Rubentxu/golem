#!/usr/bin/env python3
"""
Schema definition for Golem Engineering Graph — ADR-004.

Maps Golem node kinds and edge types to each graph DB's native schema primitives.
Used by schema.py to create/drop the schema per benchmark candidate.

Node kinds (ADR-004):
  WorkItem, Project, PackageComponent, Artifact,
  Requirement, Task, Cell, Agent, PolicyRule, SupplyChainNode

Edge types (ADR-004):
  DEPENDS_ON, IMPLEMENTS, CONTAINS, CHAIN, LINKS,
  TRACKS, AUTHORED_BY, ASSIGNED_TO, VERIFIES

Each client module translates these to its native schema:
  - HugeGraph: property keys + vertex labels + edge labels (Gremlin)
  - NebulaGraph: tags + edge types (nGQL)
  - Dgraph: GraphQL++ type definitions
"""

from dataclasses import dataclass

# ── Domain model ─────────────────────────────────────────────────────────────

NODE_KINDS = [
    "WorkItem",
    "Project",
    "PackageComponent",
    "Artifact",
    "Requirement",
    "Task",
    "Cell",
    "Agent",
    "PolicyRule",
    "SupplyChainNode",
]

EDGE_TYPES = [
    "DEPENDS_ON",
    "IMPLEMENTS",
    "CONTAINS",
    "CHAIN",
    "LINKS",
    "TRACKS",
    "AUTHORED_BY",
    "ASSIGNED_TO",
    "VERIFIES",
]

# Schema generation strategy per DB
SCHEMA_STRATEGY = {
    "hugegraph": "gremlin",   # property keys + vertex labels + edge labels
    "nebula":    "ngql",      # CREATE TAG / CREATE EDGE
    "dgraph":    "graphql",   # GraphQL++ type definitions
}


@dataclass
class NodeKind:
    name: str
    properties: list[str]   # attribute keys

@dataclass
class EdgeType:
    name: str
    properties: list[str]
    from_kinds: list[str]   # valid source node kinds
    to_kinds: list[str]     # valid target node kinds


# Node kind definitions with their attributes
# These are representative of what appears in the Golem domain (ADR-004)
NODE_KIND_DEFS: dict[str, NodeKind] = {
    "WorkItem": NodeKind(
        name="WorkItem",
        properties=["title", "status", "priority", "created_at"],
    ),
    "Project": NodeKind(
        name="Project",
        properties=["name", "description", "status"],
    ),
    "PackageComponent": NodeKind(
        name="PackageComponent",
        properties=["name", "version", "ecosystem"],
    ),
    "Artifact": NodeKind(
        name="Artifact",
        properties=["name", "digest", "size_bytes"],
    ),
    "Requirement": NodeKind(
        name="Requirement",
        properties=["title", "statement", "risk_level"],
    ),
    "Task": NodeKind(
        name="Task",
        properties=["title", "status", "estimate_h"],
    ),
    "Cell": NodeKind(
        name="Cell",
        properties=["name", "cell_type"],
    ),
    "Agent": NodeKind(
        name="Agent",
        properties=["name", "agent_class"],
    ),
    "PolicyRule": NodeKind(
        name="PolicyRule",
        properties=["name", "effect", "resource"],
    ),
    "SupplyChainNode": NodeKind(
        name="SupplyChainNode",
        properties=["name", "node_type", "url"],
    ),
}

# Edge type definitions
EDGE_TYPE_DEFS: dict[str, EdgeType] = {
    "DEPENDS_ON": EdgeType(
        name="DEPENDS_ON",
        properties=["weight"],
        from_kinds=["WorkItem", "PackageComponent", "Artifact", "Task"],
        to_kinds=["WorkItem", "PackageComponent", "Artifact", "Task", "Requirement"],
    ),
    "IMPLEMENTS": EdgeType(
        name="IMPLEMENTS",
        properties=[],
        from_kinds=["WorkItem", "Task"],
        to_kinds=["Requirement", "Artifact"],
    ),
    "CONTAINS": EdgeType(
        name="CONTAINS",
        properties=["role"],
        from_kinds=["Project", "Cell", "WorkItem"],
        to_kinds=["WorkItem", "Task", "PackageComponent", "Artifact"],
    ),
    "CHAIN": EdgeType(
        name="CHAIN",
        properties=["position"],
        from_kinds=["Task", "WorkItem"],
        to_kinds=["Task", "WorkItem"],
    ),
    "LINKS": EdgeType(
        name="LINKS",
        properties=["context"],
        from_kinds=["WorkItem", "Artifact"],
        to_kinds=["WorkItem", "Artifact", "SupplyChainNode"],
    ),
    "TRACKS": EdgeType(
        name="TRACKS",
        properties=[],
        from_kinds=["Agent"],
        to_kinds=["WorkItem", "Task"],
    ),
    "AUTHORED_BY": EdgeType(
        name="AUTHORED_BY",
        properties=["role"],
        from_kinds=["WorkItem", "Requirement", "PolicyRule"],
        to_kinds=["Agent"],
    ),
    "ASSIGNED_TO": EdgeType(
        name="ASSIGNED_TO",
        properties=["assigned_at"],
        from_kinds=["WorkItem", "Task", "Requirement"],
        to_kinds=["Agent"],
    ),
    "VERIFIES": EdgeType(
        name="VERIFIES",
        properties=["method"],
        from_kinds=["Task", "WorkItem"],
        to_kinds=["Requirement", "Artifact"],
    ),
}
