// Package memstore provides the in-memory reference adapter of the
// SearchIndex port: a substring-matching index with merge upserts and
// deterministic (score, ID) ordering. It defines the semantic baseline
// the OpenSearch adapter (ADR-015 reference) must reproduce via
// tck.RunSearchIndexTCK.
package memstore

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/Rubentxu/golem/internal/ports"
)

// MaxLimit caps any query page (query budget).
const MaxLimit = 100

type key struct {
	tenant string
	id     string
}

// Index is an in-memory SearchIndex. Safe for concurrent use.
type Index struct {
	mu   sync.RWMutex
	docs map[key]ports.SearchDoc
}

// NewSearch builds an empty index.
func NewSearch() *Index { return &Index{docs: map[key]ports.SearchDoc{}} }

// Index upserts documents with merge semantics: non-zero fields of the
// incoming doc overwrite the stored ones; zero fields keep the stored
// value. Replaying the same document is a no-op.
func (ix *Index) Index(_ context.Context, docs []ports.SearchDoc) error {
	ix.mu.Lock()
	defer ix.mu.Unlock()
	for _, d := range docs {
		if d.Tenant == "" || d.ID == "" {
			return ports.ErrEmptyTenant
		}
		k := key{tenant: string(d.Tenant), id: d.ID}
		stored := ix.docs[k] // zero value when absent
		if d.Kind != "" {
			stored.Kind = d.Kind
		}
		if d.Title != "" {
			stored.Title = d.Title
		}
		if d.Text != "" {
			stored.Text = d.Text
		}
		stored.ID, stored.Tenant = d.ID, d.Tenant
		ix.docs[k] = stored
	}
	return nil
}

// Query matches the case-insensitive substring in Title or Text, applies
// the kind filter, orders by (score desc, ID asc) and pages by ID
// cursor. Score: 2.0 for a title match, 1.0 for body-only — stable and
// adapter-defined.
func (ix *Index) Query(_ context.Context, q ports.SearchQuery) (ports.SearchPage, error) {
	if q.Tenant == "" {
		return ports.SearchPage{}, ports.ErrEmptyTenant
	}
	if q.Limit <= 0 || q.Limit > MaxLimit {
		return ports.SearchPage{}, fmt.Errorf("search: limit must be in (0, %d], got %d", MaxLimit, q.Limit)
	}
	needle := strings.ToLower(strings.TrimSpace(q.Q))

	ix.mu.RLock()
	hits := make([]ports.SearchHit, 0)
	for _, d := range ix.docs {
		if string(d.Tenant) != string(q.Tenant) {
			continue
		}
		if q.Kind != "" && d.Kind != q.Kind {
			continue
		}
		if needle != "" {
			inTitle := strings.Contains(strings.ToLower(d.Title), needle)
			inText := strings.Contains(strings.ToLower(d.Text), needle)
			if !inTitle && !inText {
				continue
			}
			score := 1.0
			if inTitle {
				score = 2.0
			}
			hits = append(hits, ports.SearchHit{Doc: d, Score: score})
		} else {
			hits = append(hits, ports.SearchHit{Doc: d, Score: 0})
		}
	}
	ix.mu.RUnlock()

	sort.Slice(hits, func(i, j int) bool {
		if hits[i].Score != hits[j].Score {
			return hits[i].Score > hits[j].Score
		}
		return hits[i].Doc.ID < hits[j].Doc.ID
	})

	start := 0
	if q.Cursor != "" {
		start = sort.Search(len(hits), func(i int) bool { return hits[i].Doc.ID > q.Cursor })
	}
	end := start + q.Limit
	if end > len(hits) {
		end = len(hits)
	}
	page := ports.SearchPage{Hits: hits[start:end]}
	if end < len(hits) {
		page.NextCursor = hits[end-1].Doc.ID
	}
	return page, nil
}
