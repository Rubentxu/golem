package memstore

import (
	"context"
	"testing"
)

// TestQuotaStore_Consume verifies quota consumption.
func TestQuotaStore_Consume(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	store := NewQuotaStore()
	store.SetQuota("tenant-1", "events", 100)

	decision, err := store.Consume(ctx, "tenant-1", "events", 10)
	if err != nil {
		t.Fatalf("Consume error: %v", err)
	}

	if decision.Outcome != "allowed" {
		t.Errorf("Expected 'allowed', got '%s'", decision.Outcome)
	}
}

// TestQuotaStore_Denied verifies quota exceeded.
func TestQuotaStore_Denied(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	store := NewQuotaStore()
	store.SetQuota("tenant-2", "events", 10)

	// Exceed quota.
	decision, err := store.Consume(ctx, "tenant-2", "events", 15)
	if err != nil {
		t.Fatalf("Consume error: %v", err)
	}

	if decision.Outcome != "denied" {
		t.Errorf("Expected 'denied', got '%s'", decision.Outcome)
	}
}

// TestQuotaStore_Refund verifies refund.
func TestQuotaStore_Refund(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	store := NewQuotaStore()
	store.SetQuota("tenant-3", "events", 100)

	// Consume 50 units.
	_, _ = store.Consume(ctx, "tenant-3", "events", 50)

	// Refund 20 units.
	err := store.Refund(ctx, "tenant-3", "events", 20)
	if err != nil {
		t.Fatalf("Refund error: %v", err)
	}

	// Now we should be able to consume 80 more (50 - 20 = 30 consumed, so 70 remaining).
	decision, _ := store.Consume(ctx, "tenant-3", "events", 70)
	if decision.Outcome != "allowed" {
		t.Errorf("Expected 'allowed' after refund, got '%s'", decision.Outcome)
	}
}

// TestQuotaStore_Limits verifies limits retrieval.
func TestQuotaStore_Limits(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	store := NewQuotaStore()
	store.SetQuota("tenant-4", "events", 500)
	store.SetQuota("tenant-4", "storage", 1000)

	limits, err := store.Limits(ctx, "tenant-4")
	if err != nil {
		t.Fatalf("Limits error: %v", err)
	}

	if limits["events"] != 500 {
		t.Errorf("Expected events limit 500, got %d", limits["events"])
	}
	if limits["storage"] != 1000 {
		t.Errorf("Expected storage limit 1000, got %d", limits["storage"])
	}
}
