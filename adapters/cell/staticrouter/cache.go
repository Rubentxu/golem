package staticrouter

import (
	"container/list"
	"sync"
	"time"

	"github.com/Rubentxu/golem/internal/ports"
)

// Cache implements an LRU cache with TTL for cell routing (REQ-CELL-005).
type Cache struct {
	mu      sync.RWMutex
	entries map[string]*list.Element
	lru     *list.List
	ttl     time.Duration
	maxSize int
}

// cacheEntry holds a cached cell assignment.
type cacheEntry struct {
	cell      ports.CellID
	expiresAt time.Time
}

// NewCache creates an LRU cache with the given TTL and max size.
func NewCache(ttl time.Duration, maxSize int) *Cache {
	return &Cache{
		entries: make(map[string]*list.Element),
		lru:     list.New(),
		ttl:     ttl,
		maxSize: maxSize,
	}
}

// Get returns the cached cell for a tenant, or false if not found/expired.
func (c *Cache) Get(tenantID string) (ports.CellID, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	elem, ok := c.entries[tenantID]
	if !ok {
		return "", false
	}
	entry := elem.Value.(*cacheEntry)
	if time.Now().After(entry.expiresAt) {
		c.lru.Remove(elem)
		delete(c.entries, tenantID)
		return "", false
	}
	// Move to front (most recently used).
	c.lru.MoveToFront(elem)
	return entry.cell, true
}

// Set caches a cell assignment for a tenant.
func (c *Cache) Set(tenantID string, cell ports.CellID) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if already exists.
	if elem, ok := c.entries[tenantID]; ok {
		c.lru.MoveToFront(elem)
		entry := elem.Value.(*cacheEntry)
		entry.cell = cell
		entry.expiresAt = time.Now().Add(c.ttl)
		return
	}

	// Evict oldest if at capacity.
	if c.lru.Len() >= c.maxSize {
		oldest := c.lru.Back()
		if oldest != nil {
			entry := oldest.Value.(*cacheEntry)
			c.lru.Remove(oldest)
			// Find and delete the entry by value - we need tenantID key.
			for k, v := range c.entries {
				if v == oldest {
					delete(c.entries, k)
					break
				}
			}
			_ = entry // suppress unused warning
		}
	}

	// Add new entry.
	entry := &cacheEntry{cell: cell, expiresAt: time.Now().Add(c.ttl)}
	elem := c.lru.PushFront(entry)
	c.entries[tenantID] = elem
}

// Invalidate removes a tenant from the cache.
func (c *Cache) Invalidate(tenantID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.entries[tenantID]; ok {
		c.lru.Remove(elem)
		delete(c.entries, tenantID)
	}
}

// Compile-time interface check.
var _ ports.CellRouter = (*Router)(nil)
