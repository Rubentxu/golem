package staticrouter

import (
	"testing"
	"time"

	"github.com/Rubentxu/golem/internal/ports"
)

// TestStaticRouter_CacheInvalidate verifies cache invalidation (REQ-CELL-005).
func TestStaticRouter_CacheInvalidate(t *testing.T) {
	t.Parallel()
	cache := NewCache(5*time.Minute, 100)

	tenantID := "tenant-123"
	cellID := ports.CellID("cell-a")

	// Set a cache entry.
	cache.Set(tenantID, cellID)

	// Verify it exists.
	if got, ok := cache.Get(tenantID); !ok || got != cellID {
		t.Errorf("Get after Set: got (%s, %v), want (cell-a, true)", got, ok)
	}

	// Invalidate.
	cache.Invalidate(tenantID)

	// Verify it's gone.
	if _, ok := cache.Get(tenantID); ok {
		t.Error("Get after Invalidate: expected not found")
	}
}
