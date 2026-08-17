package tck

import (
	"context"
	"testing"

	"github.com/Rubentxu/golem/internal/ports"
)

// RunSearchIndexTCK is the black-box conformance kit for the SearchIndex
// port (ADR-015, ADR-046). The in-memory baseline and the future
// OpenSearch adapter must both pass it: merge upserts, tenant scoping,
// kind filter, deterministic ordering and cursor pagination.
//
// The factory must return an empty, isolated index per call.
func RunSearchIndexTCK(t *testing.T, newIndex func() ports.SearchIndex) {
	doc := func(id, title string) ports.SearchDoc {
		return ports.SearchDoc{ID: id, Tenant: "t_tck", Kind: "WorkItem", Title: title, Text: title + " body"}
	}

	t.Run("index and query round trip", func(t *testing.T) {
		ix := newIndex()
		ctx := context.Background()
		if err := ix.Index(ctx, []ports.SearchDoc{doc("wi-1", "kernel slice"), doc("wi-2", "graph projection")}); err != nil {
			t.Fatal(err)
		}
		page, err := ix.Query(ctx, ports.SearchQuery{Tenant: "t_tck", Q: "slice", Limit: 10})
		if err != nil {
			t.Fatal(err)
		}
		if len(page.Hits) != 1 || page.Hits[0].Doc.ID != "wi-1" {
			t.Fatalf("hits = %+v", page.Hits)
		}
	})

	t.Run("index merges fields idempotently", func(t *testing.T) {
		ix := newIndex()
		ctx := context.Background()
		if err := ix.Index(ctx, []ports.SearchDoc{doc("wi-1", "first title")}); err != nil {
			t.Fatal(err)
		}
		// Update event carries only the changed field: merge, not replace.
		if err := ix.Index(ctx, []ports.SearchDoc{{ID: "wi-1", Tenant: "t_tck", Kind: "WorkItem", Title: "renamed"}}); err != nil {
			t.Fatal(err)
		}
		page, err := ix.Query(ctx, ports.SearchQuery{Tenant: "t_tck", Q: "body", Limit: 10})
		if err != nil {
			t.Fatal(err)
		}
		if len(page.Hits) != 1 {
			t.Fatalf("merge dropped Text: %+v", page.Hits)
		}
		if page.Hits[0].Doc.Title != "renamed" {
			t.Fatalf("title not merged: %+v", page.Hits[0].Doc)
		}
		// Replaying the same upsert twice is a no-op.
		if err := ix.Index(ctx, []ports.SearchDoc{{ID: "wi-1", Tenant: "t_tck", Title: "renamed"}}); err != nil {
			t.Fatal(err)
		}
		page, _ = ix.Query(ctx, ports.SearchQuery{Tenant: "t_tck", Q: "renamed", Limit: 10})
		if len(page.Hits) != 1 {
			t.Fatalf("replay changed result count: %+v", page.Hits)
		}
	})

	t.Run("tenants are isolated", func(t *testing.T) {
		ix := newIndex()
		ctx := context.Background()
		a := doc("wi-1", "shared title")
		b := doc("wi-1", "other tenant")
		b.Tenant = "t_other"
		if err := ix.Index(ctx, []ports.SearchDoc{a, b}); err != nil {
			t.Fatal(err)
		}
		page, err := ix.Query(ctx, ports.SearchQuery{Tenant: "t_tck", Q: "shared", Limit: 10})
		if err != nil {
			t.Fatal(err)
		}
		if len(page.Hits) != 1 {
			t.Fatalf("cross-tenant leak: %+v", page.Hits)
		}
	})

	t.Run("kind filter applies", func(t *testing.T) {
		ix := newIndex()
		ctx := context.Background()
		req := doc("req-1", "traceability spec")
		req.Kind = "Requirement"
		if err := ix.Index(ctx, []ports.SearchDoc{doc("wi-1", "traceability spec"), req}); err != nil {
			t.Fatal(err)
		}
		page, err := ix.Query(ctx, ports.SearchQuery{Tenant: "t_tck", Q: "traceability", Kind: "Requirement", Limit: 10})
		if err != nil {
			t.Fatal(err)
		}
		if len(page.Hits) != 1 || page.Hits[0].Doc.Kind != "Requirement" {
			t.Fatalf("kind filter: %+v", page.Hits)
		}
	})

	t.Run("cursor pagination is stable and exhaustive", func(t *testing.T) {
		ix := newIndex()
		ctx := context.Background()
		docs := make([]ports.SearchDoc, 0, 10)
		for i := 0; i < 10; i++ {
			docs = append(docs, doc("wi-"+string(rune('0'+i)), "common title"))
		}
		if err := ix.Index(ctx, docs); err != nil {
			t.Fatal(err)
		}

		var collected []string
		cursor := ""
		for {
			page, err := ix.Query(ctx, ports.SearchQuery{Tenant: "t_tck", Q: "common", Cursor: cursor, Limit: 3})
			if err != nil {
				t.Fatal(err)
			}
			for _, h := range page.Hits {
				collected = append(collected, h.Doc.ID)
			}
			if page.NextCursor == "" {
				break
			}
			cursor = page.NextCursor
		}
		if len(collected) != 10 {
			t.Fatalf("pagination collected %d docs, want 10: %v", len(collected), collected)
		}
		seen := map[string]bool{}
		for _, id := range collected {
			if seen[id] {
				t.Fatalf("duplicate id across pages: %s", id)
			}
			seen[id] = true
		}
	})

	t.Run("queries are budgeted", func(t *testing.T) {
		ix := newIndex()
		ctx := context.Background()
		if err := ix.Index(ctx, []ports.SearchDoc{doc("wi-1", "x")}); err != nil {
			t.Fatal(err)
		}
		if _, err := ix.Query(ctx, ports.SearchQuery{Tenant: "t_tck", Q: "x", Limit: 0}); err == nil {
			t.Fatal("limit 0 must be rejected")
		}
	})
}
