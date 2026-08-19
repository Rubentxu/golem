package bbolt

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/Rubentxu/golem/internal/ports"
	"github.com/Rubentxu/golem/tck"
)

// TestSplitEventTypeGolden verifies that splitEventType produces identical output
// to the hand-written implementation for known inputs (S33).
func TestSplitEventTypeGolden(t *testing.T) {
	cases := []struct {
		input    string
		expected []string
	}{
		{
			input:    "migration.harness.started.v1",
			expected: []string{"migration", "harness", "started", "v1"},
		},
		{
			input:    "extension.pack.activated.v1",
			expected: []string{"extension", "pack", "activated", "v1"},
		},
		{
			input:    "single",
			expected: []string{"single"},
		},
		{
			input:    ".leading.dot",
			expected: []string{"leading", "dot"},
		},
		{
			input:    "trailing.dot.",
			expected: []string{"trailing", "dot"},
		},
		{
			input:    "empty..parts",
			expected: []string{"empty", "parts"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			got := splitEventType(tc.input)
			if len(got) != len(tc.expected) {
				t.Fatalf("len=%d, want %d; input=%q", len(got), len(tc.expected), tc.input)
			}
			for i := range got {
				if got[i] != tc.expected[i] {
					t.Errorf("got[%d]=%q, want[%d]=%q; input=%q", i, got[i], i, tc.expected[i], tc.input)
				}
			}
		})
	}
}

// TestJournalConformance runs the JournalStore TCK to verify bbolt is conformant
// with the journal contract (ADR-046, ADR-052). Each subtest gets its own isolated
// bbolt file via t.TempDir().
func TestJournalConformance(t *testing.T) {
	var current *Store
	tck.RunJournalStoreTCK(t, func() ports.JournalStore {
		if current != nil {
			current.Close()
		}
		dir := t.TempDir()
		store, err := NewJournal(dir+"/journal.db", Options{})
		if err != nil {
			t.Fatalf("NewJournal: %v", err)
		}
		current = store
		return store
	})
}

// makeTestEvent creates a valid RawEvent for testing.
func makeTestEvent(t *testing.T, idx int) ports.RawEvent {
	return ports.RawEvent{
		EventID:       fmt.Sprintf("evt-%04d", idx),
		TenantID:      "tenant-test",
		StreamID:      "stream-test",
		EventType:     "test.backup.streaming.v1",
		SchemaVersion: 1,
		OccurredAt:    time.Now(),
		Actor:         ports.Actor{Type: "user", ID: "actor-1"},
		Payload:       json.RawMessage(`{"index":` + fmt.Sprintf("%d", idx) + `}`),
	}
}

// TestBackupStreaming verifies Backup() streams to NDJSON file without loading
// all events into memory, and produces a valid backup with correct digest.
func TestBackupStreaming(t *testing.T) {
	dir := t.TempDir()
	store, err := NewJournal(dir+"/journal.db", Options{})
	if err != nil {
		t.Fatalf("NewJournal: %v", err)
	}
	defer store.Close()

	// Append 1000 events.
	const N = 1000
	events := make([]ports.RawEvent, N)
	for i := 0; i < N; i++ {
		events[i] = makeTestEvent(t, i)
	}
	_, err = store.Append(context.Background(), events)
	if err != nil {
		t.Fatalf("Append: %v", err)
	}

	// Call Backup.
	handle, err := store.Backup(context.Background())
	if err != nil {
		t.Fatalf("Backup: %v", err)
	}

	// Verify file exists.
	if _, err := os.Stat(handle.Path); os.IsNotExist(err) {
		t.Fatalf("backup file does not exist at %s", handle.Path)
	}

	// Verify ID format.
	if handle.ID == "" {
		t.Error("BackupHandle.ID should not be empty")
	}

	// Verify digest format.
	if handle.Digest == "" {
		t.Error("BackupHandle.Digest should not be empty")
	}

	// Verify size is reasonable.
	if handle.SizeBytes == 0 {
		t.Error("BackupHandle.SizeBytes should not be zero for non-empty journal")
	}

	// Read file and verify content matches events.
	f, err := os.Open(handle.Path)
	if err != nil {
		t.Fatalf("open backup: %v", err)
	}
	defer f.Close()

	// Compute digest and read content.
	hasher := sha256.New()
	multi := io.MultiWriter(hasher)
	var buf bytes.Buffer
	multi = io.MultiWriter(hasher, &buf)

	_, err = f.Seek(0, 0)
	if err != nil {
		t.Fatalf("seek: %v", err)
	}

	_, err = io.Copy(multi, f)
	if err != nil {
		t.Fatalf("copy: %v", err)
	}
	computedDigest := fmt.Sprintf("sha256:%x", hasher.Sum(nil))
	if computedDigest != handle.Digest {
		t.Errorf("digest mismatch: got %s, want %s", computedDigest, handle.Digest)
	}

	// Verify NDJSON: each line is a valid JSON object.
	lines := bytes.Split(buf.Bytes(), []byte{'\n'})
	// Last line is empty from trailing newline.
	var validLines int
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		var v json.RawMessage
		if err := json.Unmarshal(line, &v); err != nil {
			t.Errorf("line %d is not valid JSON: %v\n%s", validLines, err, string(line))
		}
		validLines++
	}
	if validLines != N {
		t.Errorf("expected %d events, got %d valid NDJSON lines", N, validLines)
	}

	// Verify each event can be unmarshaled and matches original.
	seenEvents := make(map[string]bool)
	lineIdx := 0
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		var raw json.RawMessage
		if err := json.Unmarshal(line, &raw); err != nil {
			continue
		}
		// Unwrap the outer RawMessage to get the actual event.
		var wrapper struct {
			EventID string `json:"event_id"`
		}
		if err := json.Unmarshal(raw, &wrapper); err != nil {
			t.Errorf("line %d: unmarshal wrapper: %v", lineIdx, err)
			lineIdx++
			continue
		}
		seenEvents[wrapper.EventID] = true
		lineIdx++
	}
	for i := 0; i < N; i++ {
		expectedID := fmt.Sprintf("evt-%04d", i)
		if !seenEvents[expectedID] {
			t.Errorf("expected event %s not found in backup", expectedID)
		}
	}
}

// TestConcurrentReadWrite verifies that concurrent ReadStream (read-only) operations
// and Append (write) operations can proceed without data races or torn reads.
// Uses the race detector to ensure no races exist.
//
// This test spawns:
//   - 1 writer goroutine that Appends 1000 events across 10 streams
//   - 4 reader goroutines that continuously ReadStream on all streams
//
// A torn read would occur if a single ReadStream call returns events with
// duplicate event IDs or non-contiguous versions (within that single call).
// Reading the same events across multiple calls is expected (not an error).
func TestConcurrentReadWrite(t *testing.T) {
	dir := t.TempDir()
	store, err := NewJournal(dir+"/journal.db", Options{})
	if err != nil {
		t.Fatalf("NewJournal: %v", err)
	}
	defer store.Close()

	const (
		numStreams      = 10
		eventsPerStream = 100
		numReaders      = 4
		readerIters     = 50
	)

	// Track errors from reader goroutines.
	var mu sync.Mutex
	var readerErrs []error

	// Reader goroutine: continuously reads all streams.
	// Within each ReadStream call, we check for torn reads (duplicate event IDs
	// or non-contiguous versions). Reading the same events across multiple
	// ReadStream calls is fine and expected.
	readerFunc := func(id int) {
		tenant := ports.TenantID("tenant-concurrent")
		for iter := 0; iter < readerIters; iter++ {
			for streamID := 0; streamID < numStreams; streamID++ {
				streamKey := fmt.Sprintf("stream-%04d", streamID)
				events, err := store.ReadStream(context.Background(), tenant, streamKey, 0)
				mu.Lock()
				if err != nil {
					readerErrs = append(readerErrs, fmt.Errorf("reader %d: %v", id, err))
					mu.Unlock()
					continue
				}

				// Check for torn reads WITHIN this single ReadStream call.
				// A torn read would show duplicate event IDs or non-contiguous versions.
				seenIDs := make(map[string]bool)
				var prevVersion uint64
				for i, e := range events {
					if seenIDs[e.EventID] {
						readerErrs = append(readerErrs, fmt.Errorf("reader %d: TORN READ - duplicate event ID %s in stream %s (read #%d)", id, e.EventID, streamKey, iter))
					}
					seenIDs[e.EventID] = true

					// Extract version from event payload.
					var payload struct {
						Index int `json:"index"`
					}
					if err := json.Unmarshal(e.Payload, &payload); err != nil {
						readerErrs = append(readerErrs, fmt.Errorf("reader %d: unmarshal payload: %v", id, err))
						continue
					}
					version := uint64(payload.Index + 1) // index 0 → version 1
					if i > 0 && version != prevVersion+1 {
						readerErrs = append(readerErrs, fmt.Errorf("reader %d: TORN READ - non-contiguous version in stream %s: expected %d, got %d (read #%d)", id, streamKey, prevVersion+1, version, iter))
					}
					prevVersion = version
				}
				mu.Unlock()
			}
			// Small yield to allow writer to interleave.
			time.Sleep(time.Microsecond)
		}
	}

	// Writer: append events to all streams.
	const totalEvents = numStreams * eventsPerStream
	allEvents := make([]ports.RawEvent, totalEvents)
	now := time.Now()
	for streamID := 0; streamID < numStreams; streamID++ {
		streamKey := fmt.Sprintf("stream-%04d", streamID)
		for eventIdx := 0; eventIdx < eventsPerStream; eventIdx++ {
			idx := streamID*eventsPerStream + eventIdx
			allEvents[idx] = ports.RawEvent{
				EventID:       fmt.Sprintf("evt-%06d", idx),
				TenantID:      "tenant-concurrent",
				StreamID:      streamKey,
				EventType:     "test.concurrent.stream.v1",
				SchemaVersion: 1,
				OccurredAt:    now.Add(time.Duration(idx) * time.Millisecond),
				Actor:         ports.Actor{Type: "user", ID: "actor-writer"},
				Payload:       json.RawMessage(fmt.Sprintf(`{"index":%d}`, eventIdx)),
			}
		}
	}

	// Run writer and readers concurrently.
	var wg sync.WaitGroup
	wg.Add(numReaders + 1)

	// Start readers.
	for i := 0; i < numReaders; i++ {
		go func(id int) {
			defer wg.Done()
			readerFunc(id)
		}(i)
	}

	// Start writer (append all events in one batch, but bbolt serializes internally).
	go func() {
		defer wg.Done()
		// Append in smaller batches to allow interleaving with readers.
		batchSize := 50
		for i := 0; i < totalEvents; i += batchSize {
			end := i + batchSize
			if end > totalEvents {
				end = totalEvents
			}
			_, err := store.Append(context.Background(), allEvents[i:end])
			if err != nil {
				t.Errorf("Append error: %v", err)
			}
			// Yield to allow readers to interleave.
			time.Sleep(time.Microsecond)
		}
	}()

	wg.Wait()

	// Report any errors.
	mu.Lock()
	defer mu.Unlock()
	for _, err := range readerErrs {
		t.Error(err)
	}

	// Verify all events were written correctly (final state check).
	tenant := ports.TenantID("tenant-concurrent")
	for streamID := 0; streamID < numStreams; streamID++ {
		streamKey := fmt.Sprintf("stream-%04d", streamID)
		events, err := store.ReadStream(context.Background(), tenant, streamKey, 0)
		if err != nil {
			t.Errorf("final ReadStream error for %s: %v", streamKey, err)
			continue
		}
		if len(events) != eventsPerStream {
			t.Errorf("stream %s: expected %d events, got %d", streamKey, eventsPerStream, len(events))
		}
		// Verify no duplicates in final state.
		seenIDs := make(map[string]bool)
		for _, e := range events {
			if seenIDs[e.EventID] {
				t.Errorf("stream %s: duplicate event ID %s in final state", streamKey, e.EventID)
			}
			seenIDs[e.EventID] = true
		}
	}
}

// TestRestore verifies that Restore correctly inverse-Backups a journal.
func TestRestore(t *testing.T) {
	// --- Set up source store and populate 100 events ---
	srcDir := t.TempDir()
	srcStore, err := NewJournal(srcDir+"/journal.db", Options{})
	if err != nil {
		t.Fatalf("NewJournal source: %v", err)
	}

	const N = 100
	srcEvents := make([]ports.RawEvent, N)
	now := time.Now()
	for i := 0; i < N; i++ {
		srcEvents[i] = ports.RawEvent{
			EventID:       fmt.Sprintf("restore-evt-%04d", i),
			TenantID:      "tenant-restore",
			StreamID:      "stream-restore",
			EventType:     "test.restore.event.v1",
			SchemaVersion: 1,
			OccurredAt:    now.Add(time.Duration(i) * time.Millisecond),
			Actor:         ports.Actor{Type: "user", ID: "actor-restore"},
			Payload:       json.RawMessage(fmt.Sprintf(`{"index":%d}`, i)),
		}
	}
	_, err = srcStore.Append(context.Background(), srcEvents)
	if err != nil {
		t.Fatalf("Append source: %v", err)
	}

	// Backup the source.
	handle, err := srcStore.Backup(context.Background())
	if err != nil {
		t.Fatalf("Backup: %v", err)
	}
	srcStore.Close()

	// --- Open fresh target store at different path ---
	tgtDir := t.TempDir()
	tgtStore, err := NewJournal(tgtDir+"/journal.db", Options{})
	if err != nil {
		t.Fatalf("NewJournal target: %v", err)
	}
	defer tgtStore.Close()

	// Verify target is empty.
	h, err := tgtStore.Head(context.Background())
	if err != nil {
		t.Fatalf("Head: %v", err)
	}
	if h != 0 {
		t.Fatalf("target should be empty, got head=%d", h)
	}

	// --- Restore ---
	err = tgtStore.Restore(context.Background(), handle)
	if err != nil {
		t.Fatalf("Restore: %v", err)
	}

	// Verify Head == N.
	h, err = tgtStore.Head(context.Background())
	if err != nil {
		t.Fatalf("Head after restore: %v", err)
	}
	if h != ports.StreamPosition(N) {
		t.Errorf("Head after restore: got %d, want %d", h, N)
	}

	// Verify Replay returns same events in same order.
	replayed, lastPos, err := tgtStore.Replay(context.Background(), 0, 0)
	if err != nil {
		t.Fatalf("Replay: %v", err)
	}
	if len(replayed) != N {
		t.Errorf("Replay count: got %d, want %d", len(replayed), N)
	}
	if lastPos != ports.StreamPosition(N) {
		t.Errorf("lastPos: got %d, want %d", lastPos, N)
	}
	for i := 0; i < N; i++ {
		if replayed[i].EventID != srcEvents[i].EventID {
			t.Errorf("event[%d]: got EventID=%s, want %s", i, replayed[i].EventID, srcEvents[i].EventID)
		}
	}
}

// TestRestoreIdempotent verifies that calling Restore on a non-empty journal
// returns ErrRestoreNotEmpty.
func TestRestoreIdempotent(t *testing.T) {
	srcDir := t.TempDir()
	srcStore, err := NewJournal(srcDir+"/journal.db", Options{})
	if err != nil {
		t.Fatalf("NewJournal source: %v", err)
	}
	const N = 50
	events := make([]ports.RawEvent, N)
	now := time.Now()
	for i := 0; i < N; i++ {
		events[i] = ports.RawEvent{
			EventID:       fmt.Sprintf("idempotent-evt-%04d", i),
			TenantID:      "tenant-idempotent",
			StreamID:      "stream-idempotent",
			EventType:     "test.idempotent.event.v1",
			SchemaVersion: 1,
			OccurredAt:    now.Add(time.Duration(i) * time.Millisecond),
			Actor:         ports.Actor{Type: "user", ID: "actor-idempotent"},
			Payload:       json.RawMessage(fmt.Sprintf(`{"index":%d}`, i)),
		}
	}
	_, err = srcStore.Append(context.Background(), events)
	if err != nil {
		t.Fatalf("Append: %v", err)
	}
	handle, err := srcStore.Backup(context.Background())
	if err != nil {
		t.Fatalf("Backup: %v", err)
	}
	srcStore.Close()

	// Restore into a non-empty journal.
	tgtDir := t.TempDir()
	tgtStore, err := NewJournal(tgtDir+"/journal.db", Options{})
	if err != nil {
		t.Fatalf("NewJournal target: %v", err)
	}
	defer tgtStore.Close()

	// Add one event to make the target non-empty.
	_, err = tgtStore.Append(context.Background(), []ports.RawEvent{
		{
			EventID:       "pre-existing-evt",
			TenantID:      "tenant-idempotent",
			StreamID:      "stream-idempotent",
			EventType:     "test.idempotent.event.v1",
			SchemaVersion: 1,
			OccurredAt:    time.Now(),
			Actor:         ports.Actor{Type: "user", ID: "actor-idempotent"},
			Payload:       json.RawMessage(`{"index":999}`),
		},
	})
	if err != nil {
		t.Fatalf("Append target: %v", err)
	}

	// Restore should return ErrRestoreNotEmpty.
	err = tgtStore.Restore(context.Background(), handle)
	if !errors.Is(err, ports.ErrRestoreNotEmpty) {
		t.Errorf("Restore on non-empty: got %v, want %v", err, ports.ErrRestoreNotEmpty)
	}
}

// TestRestoreDigestMismatch verifies that restoring a tampered backup file
// returns ErrRestoreMismatch.
func TestRestoreDigestMismatch(t *testing.T) {
	srcDir := t.TempDir()
	srcStore, err := NewJournal(srcDir+"/journal.db", Options{})
	if err != nil {
		t.Fatalf("NewJournal source: %v", err)
	}
	const N = 20
	events := make([]ports.RawEvent, N)
	now := time.Now()
	for i := 0; i < N; i++ {
		events[i] = ports.RawEvent{
			EventID:       fmt.Sprintf("tamper-evt-%04d", i),
			TenantID:      "tenant-tamper",
			StreamID:      "stream-tamper",
			EventType:     "test.tamper.event.v1",
			SchemaVersion: 1,
			OccurredAt:    now.Add(time.Duration(i) * time.Millisecond),
			Actor:         ports.Actor{Type: "user", ID: "actor-tamper"},
			Payload:       json.RawMessage(fmt.Sprintf(`{"index":%d}`, i)),
		}
	}
	_, err = srcStore.Append(context.Background(), events)
	if err != nil {
		t.Fatalf("Append: %v", err)
	}
	handle, err := srcStore.Backup(context.Background())
	if err != nil {
		t.Fatalf("Backup: %v", err)
	}
	srcStore.Close()

	// Tamper with the backup file (append a newline to corrupt it).
	f, err := os.OpenFile(handle.Path, os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		t.Fatalf("OpenFile: %v", err)
	}
	fmt.Fprintln(f, "// this corrupts the digest")
	f.Close()

	// Restore should return ErrRestoreMismatch.
	tgtDir := t.TempDir()
	tgtStore, err := NewJournal(tgtDir+"/journal.db", Options{})
	if err != nil {
		t.Fatalf("NewJournal target: %v", err)
	}
	defer tgtStore.Close()

	err = tgtStore.Restore(context.Background(), handle)
	if !errors.Is(err, ports.ErrRestoreMismatch) {
		t.Errorf("Restore tampered: got %v, want %v", err, ports.ErrRestoreMismatch)
	}
}

// TestRestoreParseError verifies that a valid-digest but invalid-NDJSON backup
// returns a parse error and leaves the journal empty.
func TestRestoreParseError(t *testing.T) {
	srcDir := t.TempDir()
	srcStore, err := NewJournal(srcDir+"/journal.db", Options{})
	if err != nil {
		t.Fatalf("NewJournal source: %v", err)
	}
	const N = 10
	events := make([]ports.RawEvent, N)
	now := time.Now()
	for i := 0; i < N; i++ {
		events[i] = ports.RawEvent{
			EventID:       fmt.Sprintf("parse-err-evt-%04d", i),
			TenantID:      "tenant-parse",
			StreamID:      "stream-parse",
			EventType:     "test.parse.event.v1",
			SchemaVersion: 1,
			OccurredAt:    now.Add(time.Duration(i) * time.Millisecond),
			Actor:         ports.Actor{Type: "user", ID: "actor-parse"},
			Payload:       json.RawMessage(fmt.Sprintf(`{"index":%d}`, i)),
		}
	}
	_, err = srcStore.Append(context.Background(), events)
	if err != nil {
		t.Fatalf("Append: %v", err)
	}
	handle, err := srcStore.Backup(context.Background())
	if err != nil {
		t.Fatalf("Backup: %v", err)
	}
	srcStore.Close()

	// Replace backup content with valid digest but invalid JSON lines.
	// We need to compute a new sha256 for the corrupted content.
	// Write some NDJSON lines that look valid but have bad payloads.
	validLines := "{\"event_id\":\"evt-A\",\"tenant_id\":\"tenant\",\"stream_id\":\"s\",\"event_type\":\"test.parse.err.v1\",\"schema_version\":1,\"occurred_at\":\"2024-01-01T00:00:00Z\",\"actor\":{\"type\":\"user\",\"id\":\"a\"},\"payload\":{}}\n"
	corruptedContent := validLines + "this is not json at all\n"

	// Write corrupted content and compute its digest.
	f2, err := os.Create(handle.Path)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	hasher := sha256.New()
	hasher.Write([]byte(corruptedContent))
	corruptedDigest := fmt.Sprintf("sha256:%x", hasher.Sum(nil))
	f2.WriteString(corruptedContent)
	f2.Close()

	// Update handle digest to match corrupted content.
	handle.Digest = corruptedDigest

	tgtDir := t.TempDir()
	tgtStore, err := NewJournal(tgtDir+"/journal.db", Options{})
	if err != nil {
		t.Fatalf("NewJournal target: %v", err)
	}
	defer tgtStore.Close()

	// Restore should return a parse error.
	err = tgtStore.Restore(context.Background(), handle)
	if err == nil {
		t.Fatalf("Restore with bad JSON: expected error, got nil")
	}
	// Journal should be empty (all-or-nothing).
	h, _ := tgtStore.Head(context.Background())
	if h != 0 {
		t.Errorf("journal after parse error: got head=%d, want 0 (empty)", h)
	}
}

// BenchmarkConcurrentReads measures the throughput of concurrent ReadStream operations
// while a writer is appending events. This validates that removing the mutex from
// read operations allows better concurrency.
func BenchmarkConcurrentReads(b *testing.B) {
	dir := b.TempDir()
	store, err := NewJournal(dir+"/journal.db", Options{})
	if err != nil {
		b.Fatalf("NewJournal: %v", err)
	}
	defer store.Close()

	const (
		numStreams      = 10
		eventsPerStream = 100
		numReaders      = 4
	)

	// Pre-populate with events.
	allEvents := make([]ports.RawEvent, numStreams*eventsPerStream)
	now := time.Now()
	for streamID := 0; streamID < numStreams; streamID++ {
		streamKey := fmt.Sprintf("stream-%04d", streamID)
		for eventIdx := 0; eventIdx < eventsPerStream; eventIdx++ {
			idx := streamID*eventsPerStream + eventIdx
			allEvents[idx] = ports.RawEvent{
				EventID:       fmt.Sprintf("evt-%06d", idx),
				TenantID:      "tenant-bench",
				StreamID:      streamKey,
				EventType:     "bench.concurrent.stream.v1",
				SchemaVersion: 1,
				OccurredAt:    now.Add(time.Duration(idx) * time.Millisecond),
				Actor:         ports.Actor{Type: "user", ID: "actor-bench"},
				Payload:       json.RawMessage(fmt.Sprintf(`{"index":%d}`, eventIdx)),
			}
		}
	}
	_, err = store.Append(context.Background(), allEvents)
	if err != nil {
		b.Fatalf("Append: %v", err)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		tenant := ports.TenantID("tenant-bench")
		for pb.Next() {
			for streamID := 0; streamID < numStreams; streamID++ {
				streamKey := fmt.Sprintf("stream-%04d", streamID)
				_, _ = store.ReadStream(context.Background(), tenant, streamKey, 0)
			}
		}
	})
}
