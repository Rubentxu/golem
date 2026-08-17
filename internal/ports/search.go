package ports

import "context"

// SearchDoc is a document unit of the search projection. Documents are
// keyed by (TenantID, ID) and upserted with MERGE semantics: indexing a
// doc again merges its non-zero fields into the stored one, keeping the
// projection stateless and deterministic under replay (ADR-015/049).
type SearchDoc struct {
	ID     string
	Tenant TenantID
	Kind   string // canonical node kind (WorkItem, Requirement, ...)
	Title  string
	Text   string // searchable body (title + attributes concatenation)
}

// SearchQuery is a tenant-scoped, budgeted text query (API_GUIDELINES:
// queries are paginated, tenant-scoped and explicitly filtered).
type SearchQuery struct {
	Tenant TenantID
	Q      string // case-insensitive substring/terms
	Kind   string // optional kind filter
	Cursor string // exclusive start: docs with ID > cursor (empty = first page)
	Limit  int    // page size, must be > 0 and capped by the adapter
}

// SearchHit is one result row.
type SearchHit struct {
	Doc   SearchDoc
	Score float64 // adapter-specific relevance; deterministic per adapter
}

// SearchPage is one results page. NextCursor is empty on the last page.
type SearchPage struct {
	Hits       []SearchHit
	NextCursor string
}

// SearchIndex is the search projection port (ADR-015: OpenSearch is the
// reference adapter; search never owns authoritative data — the whole
// index must be rebuildable from the Graph Journal).
type SearchIndex interface {
	// Index upserts documents with merge semantics; it is idempotent
	// (replaying the same event twice yields the same stored doc).
	Index(ctx context.Context, docs []SearchDoc) error
	// Query runs a bounded, tenant-scoped search. Results are ordered
	// deterministically (score desc, then ID asc) so replay/rebuild
	// yields identical pages.
	Query(ctx context.Context, q SearchQuery) (SearchPage, error)
}
