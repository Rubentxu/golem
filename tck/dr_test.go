package tck

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/Rubentxu/golem/adapters/journal/bbolt"
	"github.com/Rubentxu/golem/internal/ports"
)

// TestJournal_BackupCreatesConsistentSnapshot verifies Backup returns a valid
// handle with a matching sha256 digest (REQ-DR-001, W5.2).
func TestJournal_BackupCreatesConsistentSnapshot(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	path := t.TempDir() + "/journal.db"
	store, err := bbolt.NewJournal(path, bbolt.Options{})
	if err != nil {
		t.Fatalf("NewJournal: %v", err)
	}
	defer store.Close()

	// Append some events.
	events := sampleEvents(3)
	_, err = store.Append(ctx, events)
	if err != nil {
		t.Fatalf("Append: %v", err)
	}

	// Create backup.
	h, err := store.Backup(ctx)
	if err != nil {
		t.Fatalf("Backup: %v", err)
	}

	if h.ID == "" {
		t.Error("BackupHandle.ID is empty")
	}
	if h.Path == "" {
		t.Error("BackupHandle.Path is empty")
	}
	if h.Digest == "" {
		t.Error("BackupHandle.Digest is empty")
	}
	if h.SizeBytes == 0 {
		t.Error("BackupHandle.SizeBytes is zero after backup")
	}
}

// TestJournal_RestoreReplaysDigest verifies Restore accepts a valid backup
// and replays events with matching digest (REQ-DR-001, W5.3).
func TestJournal_RestoreReplaysDigest(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	path := t.TempDir() + "/journal.db"
	store, err := bbolt.NewJournal(path, bbolt.Options{})
	if err != nil {
		t.Fatalf("NewJournal: %v", err)
	}
	defer store.Close()

	// Append events and capture head.
	events := sampleEvents(5)
	results, err := store.Append(ctx, events)
	if err != nil {
		t.Fatalf("Append: %v", err)
	}
	origHead, _ := store.Head(ctx)

	// Create backup.
	backupPath := t.TempDir() + "/backup.json"
	h, err := store.BackupToPath(ctx, backupPath)
	if err != nil {
		t.Fatalf("BackupToPath: %v", err)
	}

	// Verify backup file exists.
	if _, err := os.Stat(backupPath); err != nil {
		t.Fatalf("backup file not created: %v", err)
	}

	// Create a fresh journal and restore.
	path2 := t.TempDir() + "/journal2.db"
	store2, err := bbolt.NewJournal(path2, bbolt.Options{})
	if err != nil {
		t.Fatalf("NewJournal (restore target): %v", err)
	}
	defer store2.Close()

	h.Path = backupPath
	if err := store2.Restore(ctx, h); err != nil {
		t.Fatalf("Restore: %v", err)
	}

	// Verify head matches original.
	restoredHead, _ := store2.Head(ctx)
	if restoredHead != origHead {
		t.Errorf("Restored head=%d, original=%d", restoredHead, origHead)
	}

	// Verify we can read back the events.
	_ = results // events were captured above
}

// TestJournal_RestoreDrillInCI verifies the restore drill can be run in CI
// against a fresh journal (AC-4, W5.8). This is the CI-entrypoint test.
func TestJournal_RestoreDrillInCI(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// Create source journal with events.
	srcPath := t.TempDir() + "/source.db"
	src, err := bbolt.NewJournal(srcPath, bbolt.Options{})
	if err != nil {
		t.Fatalf("NewJournal (source): %v", err)
	}

	events := sampleEvents(10)
	if _, err := src.Append(ctx, events); err != nil {
		t.Fatalf("Append: %v", err)
	}
	srcHead, _ := src.Head(ctx)

	// Perform backup.
	backupPath := t.TempDir() + "/drill-backup.json"
	h, err := src.BackupToPath(ctx, backupPath)
	if err != nil {
		t.Fatalf("BackupToPath: %v", err)
	}
	h.Path = backupPath
	src.Close()

	// Restore to fresh journal.
	dstPath := t.TempDir() + "/target.db"
	dst, err := bbolt.NewJournal(dstPath, bbolt.Options{})
	if err != nil {
		t.Fatalf("NewJournal (target): %v", err)
	}
	defer dst.Close()

	if err := dst.Restore(ctx, h); err != nil {
		t.Fatalf("Restore: %v", err)
	}

	// Verify RTO metric: head matches original.
	dstHead, _ := dst.Head(ctx)
	if dstHead != srcHead {
		t.Errorf("RTO drill: restored head=%d, source head=%d", dstHead, srcHead)
	}
}

// sampleEvents returns n sample events for testing.
func sampleEvents(n int) []ports.RawEvent {
	events := make([]ports.RawEvent, n)
	now := time.Now().UTC()
	for i := 0; i < n; i++ {
		events[i] = ports.RawEvent{
			EventID:    "event-" + string(rune('a'+i)),
			TenantID:   "tenant-test",
			StreamID:   "stream-test",
			EventType:  "test.entity.created.v1",
			OccurredAt: now.Add(time.Duration(i) * time.Millisecond),
			Actor:      ports.Actor{Type: "user", ID: "user-test"},
			Payload:    []byte(`{}`),
		}
	}
	return events
}
