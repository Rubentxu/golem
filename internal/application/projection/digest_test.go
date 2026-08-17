package projection

import (
	"testing"

	"github.com/Rubentxu/golem/internal/ports"
)

func TestDigestIsOrderInsensitiveAndContentSensitive(t *testing.T) {
	n1 := ports.Node{ID: "a", Kind: "WorkItem", Revision: 1, Attributes: map[string]any{"title": "x"}}
	n2 := ports.Node{ID: "b", Kind: "WorkItem", Revision: 1, Attributes: map[string]any{"title": "y"}}
	e1 := ports.Edge{ID: "e", Type: "DEPENDS_ON", SourceID: "a", TargetID: "b", Revision: 1}

	d1, err := Digest(ports.Subgraph{Nodes: []ports.Node{n1, n2}, Edges: []ports.Edge{e1}})
	if err != nil {
		t.Fatal(err)
	}
	d2, err := Digest(ports.Subgraph{Nodes: []ports.Node{n2, n1}, Edges: []ports.Edge{e1}})
	if err != nil {
		t.Fatal(err)
	}
	if d1 != d2 {
		t.Fatalf("digest differs under input reordering: %s vs %s", d1, d2)
	}

	n2Changed := n2
	n2Changed.Attributes["title"] = "changed"
	d3, err := Digest(ports.Subgraph{Nodes: []ports.Node{n1, n2Changed}, Edges: []ports.Edge{e1}})
	if err != nil {
		t.Fatal(err)
	}
	if d1 == d3 {
		t.Fatal("digest ignores attribute changes")
	}
}
