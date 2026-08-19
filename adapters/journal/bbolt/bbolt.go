// Package bbolt provides a bbolt-backed JournalStore adapter.
// It persists the Graph Journal to a local file using bbolt's
// append-only bucket structure.
//
// ADR-046, ADR-052, ADR-057. go.etcd.io/bbolt v1.5.0 (MIT, pure-Go).
package bbolt

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/Rubentxu/golem/internal/ports"
	bolt "go.etcd.io/bbolt"
)

// ErrVersionConflict is returned by AppendIf when the stream version
// does not match the expected version.
var ErrVersionConflict = ports.ErrVersionConflict

// Store is a bbolt-backed JournalStore. mu serializes writers (Append, AppendIf)
// because bbolt's db.Update() only serializes the write transaction itself — it
// does NOT prevent the read-check-write race in AppendIf (two concurrent calls
// could both pass the version check before either writes). Read-only operations
// (ReadStream, Replay, Head, Backup) use db.View() which is MVCC-safe and does
// NOT require the mutex; bbolt allows concurrent readers regardless of writer
// lock status.
type Store struct {
	db *bolt.DB
	mu sync.Mutex // serializes Append/AppendIf only; see comment above
}

// NewJournal opens (or creates) a bbolt file at path and returns a Store.
// File mode is 0600 by default.
func NewJournal(path string, opts Options) (*Store, error) {
	if opts.FileMode == 0 {
		opts.FileMode = 0600
	}

	db, err := bolt.Open(path, opts.FileMode, bolt.DefaultOptions)
	if err != nil {
		return nil, fmt.Errorf("bbolt.NewJournal: %w", err)
	}

	// Initialize top-level buckets.
	if err := db.Update(func(tx *bolt.Tx) error {
		if _, err := tx.CreateBucketIfNotExists([]byte(bucketMeta)); err != nil {
			return fmt.Errorf("create bucket %s: %w", bucketMeta, err)
		}
		if _, err := tx.CreateBucketIfNotExists([]byte(bucketEvents)); err != nil {
			return fmt.Errorf("create bucket %s: %w", bucketEvents, err)
		}
		if _, err := tx.CreateBucketIfNotExists([]byte(bucketIDIndex)); err != nil {
			return fmt.Errorf("create bucket %s: %w", bucketIDIndex, err)
		}
		if _, err := tx.CreateBucketIfNotExists([]byte(bucketStreams)); err != nil {
			return fmt.Errorf("create bucket %s: %w", bucketStreams, err)
		}
		if _, err := tx.CreateBucketIfNotExists([]byte(bucketCommandIndex)); err != nil {
			return fmt.Errorf("create bucket %s: %w", bucketCommandIndex, err)
		}
		return nil
	}); err != nil {
		db.Close()
		return nil, fmt.Errorf("bbolt.NewJournal init: %w", err)
	}

	return &Store{db: db}, nil
}

// Options configures the bbolt Store.
type Options struct {
	FileMode os.FileMode // default 0600
}

// Close closes the underlying bbolt database.
func (s *Store) Close() error {
	return s.db.Close()
}

// Append persists the batch atomically. Events with duplicate event_id
// are reported as duplicates (idempotent).
//
// Performance note: events are sorted by (TenantID, StreamID) before writing.
// This groups events for the same stream together so that bbolt's cursor
// amortizes seeks across the B-tree. For workloads with many events per
// stream, this reduces the number of distinct stream-version writes per batch.
// Trade-off: sorting adds O(n log n) overhead; for single-event batches or
// uniformly distributed streams the benefit may not outweigh the cost.
// Benchmark to validate for your specific workload.
func (s *Store) Append(ctx context.Context, events []ports.RawEvent) ([]ports.AppendResult, error) {
	if len(events) == 0 {
		return nil, nil
	}
	_ = ctx

	// Sort by (TenantID, StreamID) to amortize bbolt cursor seeks within each stream.
	// Duplicates are still detected via idIndex so sort stability doesn't matter.
	sorted := make([]ports.RawEvent, len(events))
	copy(sorted, events)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].TenantID != sorted[j].TenantID {
			return sorted[i].TenantID < sorted[j].TenantID
		}
		return sorted[i].StreamID < sorted[j].StreamID
	})

	s.mu.Lock()
	defer s.mu.Unlock()

	var results []ports.AppendResult
	err := s.db.Update(func(tx *bolt.Tx) error {
		results = make([]ports.AppendResult, 0, len(sorted))
		for _, e := range sorted {
			res, err := s.appendOneEvent(tx, e, nil)
			if err != nil {
				return err
			}
			results = append(results, res)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return results, nil
}

// AppendIf persists conditionally: it succeeds only when the stream currently
// holds exactly expected.Version events; otherwise returns ErrVersionConflict
// without persisting.
//
// Concurrency note: the mutex is REQUIRED here. Without it, two concurrent
// AppendIf calls with the same expected.Version could both read the current
// version, both pass the check, and both write — causing a version conflict
// that the check was supposed to prevent. bbolt's db.Update() only serializes
// the write transaction; it does NOT prevent the read-modify-write race.
// The mutex ensures the read-check-write sequence is atomic.
func (s *Store) AppendIf(ctx context.Context, expected ports.StreamVersion, events []ports.RawEvent) ([]ports.AppendResult, error) {
	if len(events) == 0 {
		return nil, nil
	}
	_ = ctx

	s.mu.Lock()
	defer s.mu.Unlock()

	var results []ports.AppendResult
	err := s.db.Update(func(tx *bolt.Tx) error {
		// Precondition: stream version must match expected.
		streamsBucket := tx.Bucket([]byte(bucketStreams))
		streamKey := streamKey(string(expected.TenantID), expected.StreamID)
		streamVersionBytes := streamsBucket.Get(streamKey)
		var currentVersion uint64
		if streamVersionBytes != nil {
			currentVersion = decodeUint64BE(streamVersionBytes)
		}
		if currentVersion != expected.Version {
			return ErrVersionConflict
		}

		results = make([]ports.AppendResult, 0, len(events))
		baseVersion := expected.Version
		for _, e := range events {
			res, err := s.appendOneEvent(tx, e, &baseVersion)
			if err != nil {
				return err
			}
			results = append(results, res)
			baseVersion++
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return results, nil
}

// appendOneEvent writes a single event inside a bolt tx.
// If baseVersion is nil, auto-increments stream version from current state.
// If baseVersion is set, uses it as the base and increments for each call.
func (s *Store) appendOneEvent(tx *bolt.Tx, e ports.RawEvent, baseVersion *uint64) (ports.AppendResult, error) {
	if err := validateEvent(e); err != nil {
		return ports.AppendResult{}, err
	}

	idIndex := tx.Bucket([]byte(bucketIDIndex))
	if posBytes := idIndex.Get([]byte(e.EventID)); posBytes != nil {
		pos := decodeUint64BE(posBytes)
		return ports.AppendResult{EventID: e.EventID, Position: ports.StreamPosition(pos), Duplicate: true}, nil
	}

	head, err := readHead(tx)
	if err != nil {
		return ports.AppendResult{}, err
	}
	newPos := head + 1

	if err := idIndex.Put([]byte(e.EventID), encodeUint64BE(newPos)); err != nil {
		return ports.AppendResult{}, fmt.Errorf("store id index: %w", err)
	}

	eventsBucket := tx.Bucket([]byte(bucketEvents))
	data, err := json.Marshal(e)
	if err != nil {
		return ports.AppendResult{}, fmt.Errorf("marshal event: %w", err)
	}
	if err := eventsBucket.Put(encodeUint64BE(newPos), data); err != nil {
		return ports.AppendResult{}, fmt.Errorf("store event: %w", err)
	}

	if err := writeHead(tx, newPos); err != nil {
		return ports.AppendResult{}, err
	}

	streamsBucket := tx.Bucket([]byte(bucketStreams))
	streamKey := streamKey(string(e.TenantID), e.StreamID)
	var streamVersion uint64
	if baseVersion != nil {
		streamVersion = *baseVersion
	} else {
		if v := streamsBucket.Get(streamKey); v != nil {
			streamVersion = decodeUint64BE(v)
		}
	}
	newStreamVersion := streamVersion + 1

	if err := streamsBucket.Put(streamKey, encodeUint64BE(newStreamVersion)); err != nil {
		return ports.AppendResult{}, fmt.Errorf("update stream version: %w", err)
	}
	versionKey := streamVersionKey(string(e.TenantID), e.StreamID, newStreamVersion)
	if err := streamsBucket.Put(versionKey, encodeUint64BE(newPos)); err != nil {
		return ports.AppendResult{}, fmt.Errorf("store stream version key: %w", err)
	}

	if err := incrementCounter(tx, counterEventCount); err != nil {
		return ports.AppendResult{}, err
	}

	return ports.AppendResult{EventID: e.EventID, Position: ports.StreamPosition(newPos)}, nil
}

// ReadStream returns events for one tenant/stream with version > fromVersion.
// No mutex needed: bbolt.View() uses MVCC — readers see a consistent snapshot
// regardless of concurrent writers; bbolt's write lock does not block readers.
func (s *Store) ReadStream(ctx context.Context, tenant ports.TenantID, streamID string, fromVersion uint64) ([]ports.RawEvent, error) {
	_ = ctx
	if tenant == "" {
		return nil, ports.ErrEmptyTenant
	}

	var events []ports.RawEvent
	err := s.db.View(func(tx *bolt.Tx) error {
		streamsBucket := tx.Bucket([]byte(bucketStreams))
		eventsBucket := tx.Bucket([]byte(bucketEvents))

		streamKey := streamKey(string(tenant), streamID)
		streamVersionBytes := streamsBucket.Get(streamKey)
		if streamVersionBytes == nil {
			return nil // stream doesn't exist
		}
		currentVersion := decodeUint64BE(streamVersionBytes)

		if fromVersion >= currentVersion {
			return nil // no events after fromVersion
		}

		// Collect positions for versions (fromVersion+1) .. currentVersion.
		positions := make([]uint64, 0, currentVersion-fromVersion)
		for v := fromVersion + 1; v <= currentVersion; v++ {
			versionKey := streamVersionKey(string(tenant), streamID, v)
			posBytes := streamsBucket.Get(versionKey)
			if posBytes != nil {
				positions = append(positions, decodeUint64BE(posBytes))
			}
		}

		// Load events in position order.
		for _, pos := range positions {
			data := eventsBucket.Get(encodeUint64BE(pos))
			if data == nil {
				continue
			}
			var e ports.RawEvent
			if err := json.Unmarshal(data, &e); err != nil {
				return fmt.Errorf("unmarshal event at pos %d: %w", pos, err)
			}
			events = append(events, e)
		}
		return nil
	})

	if err != nil {
		return nil, err
	}
	return events, nil
}

// Replay returns events with position > from, up to limit (0 = all),
// and the position of the last returned event.
// No mutex needed: bbolt.View() uses MVCC — readers see a consistent snapshot
// regardless of concurrent writers.
func (s *Store) Replay(ctx context.Context, from ports.StreamPosition, limit int) ([]ports.RawEvent, ports.StreamPosition, error) {
	_ = ctx

	var events []ports.RawEvent
	var lastPos ports.StreamPosition

	err := s.db.View(func(tx *bolt.Tx) error {
		eventsBucket := tx.Bucket([]byte(bucketEvents))
		head, err := readHead(tx)
		if err != nil {
			return err
		}

		start := int(from)
		if start >= int(head) {
			lastPos = from
			return nil
		}

		end := int(head)
		if limit > 0 && start+limit < end {
			end = start + limit
		}

		for pos := start + 1; pos <= end; pos++ {
			data := eventsBucket.Get(encodeUint64BE(uint64(pos)))
			if data == nil {
				continue
			}
			var e ports.RawEvent
			if err := json.Unmarshal(data, &e); err != nil {
				return fmt.Errorf("unmarshal event at pos %d: %w", pos, err)
			}
			events = append(events, e)
			lastPos = ports.StreamPosition(pos)
		}
		return nil
	})

	if err != nil {
		return nil, 0, err
	}
	return events, lastPos, nil
}

// Head returns the position of the newest event (0 when empty).
// No mutex needed: bbolt.View() uses MVCC — readers see a consistent snapshot
// regardless of concurrent writers.
func (s *Store) Head(ctx context.Context) (ports.StreamPosition, error) {
	_ = ctx

	var head uint64
	err := s.db.View(func(tx *bolt.Tx) error {
		h, err := readHead(tx)
		if err != nil {
			return err
		}
		head = h
		return nil
	})

	if err != nil {
		return 0, err
	}
	return ports.StreamPosition(head), nil
}

// Backup creates a consistent snapshot of the journal (REQ-DR-001).
// Events are streamed directly to a NDJSON file to avoid loading all events
// into memory (O(1) memory vs O(n) for large journals).
//
// No mutex needed: bbolt.View() uses MVCC — each call gets a consistent snapshot
// of the database at that point in time. Concurrent writers do not block readers.
// Note: the head value is read once and used only for the output filename;
// if head changes between the read and file operations, the filename may not
// reflect the latest head, but the event data is always consistent with the
// snapshot from which it was read.
func (s *Store) Backup(ctx context.Context) (ports.BackupHandle, error) {
	_ = ctx

	// Get head first (used only for naming the output file).
	var head uint64
	err := s.db.View(func(tx *bolt.Tx) error {
		h, err := readHead(tx)
		if err != nil {
			return err
		}
		head = h
		return nil
	})
	if err != nil {
		return ports.BackupHandle{}, fmt.Errorf("backup read head: %w", err)
	}

	// Determine backup path in same directory as bbolt file.
	dir := filepath.Dir(s.db.Path())

	// Create temp file in same directory.
	f, err := os.CreateTemp(dir, "backup-*.jsonl")
	if err != nil {
		return ports.BackupHandle{}, fmt.Errorf("backup create temp: %w", err)
	}
	tempPath := f.Name()

	// Set up streaming hash: write to file + hasher simultaneously.
	hasher := sha256.New()
	writer := io.MultiWriter(f, hasher)

	// Stream events directly from cursor to NDJSON file.
	err = s.db.View(func(tx *bolt.Tx) error {
		c := tx.Bucket([]byte(bucketEvents)).Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			// v is raw JSON from bbolt — write as-is as one NDJSON line.
			// json.Encoder.Encode would base64-encode []byte, so write directly.
			if _, err := writer.Write(v); err != nil {
				return fmt.Errorf("backup write event: %w", err)
			}
			if _, err := writer.Write([]byte{'\n'}); err != nil {
				return fmt.Errorf("backup write newline: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		f.Close()
		os.Remove(tempPath)
		return ports.BackupHandle{}, fmt.Errorf("backup stream: %w", err)
	}

	// Close file before computing final stats.
	if err := f.Close(); err != nil {
		os.Remove(tempPath)
		return ports.BackupHandle{}, fmt.Errorf("backup close: %w", err)
	}

	// Compute digest from hasher.
	digestStr := fmt.Sprintf("sha256:%x", hasher.Sum(nil))

	// Get file stats for size.
	stat, err := os.Stat(tempPath)
	if err != nil {
		os.Remove(tempPath)
		return ports.BackupHandle{}, fmt.Errorf("backup stat: %w", err)
	}

	// Rename to deterministic name.
	finalPath := filepath.Join(dir, fmt.Sprintf("backup-%d.jsonl", head))
	if err := os.Rename(tempPath, finalPath); err != nil {
		os.Remove(tempPath)
		return ports.BackupHandle{}, fmt.Errorf("backup rename: %w", err)
	}

	return ports.BackupHandle{
		ID:        fmt.Sprintf("backup-%d", head),
		Path:      finalPath,
		Digest:    digestStr,
		SizeBytes: stat.Size(),
	}, nil
}

// Restore restores the journal from a backup handle (REQ-DR-001).
// It reads the NDJSON backup file, verifies the sha256 digest, and replays
// all events into the journal atomically under the write mutex.
//
// Restore is the inverse of Backup: the backup format is NDJSON (one JSON-encoded
// event per line, as written by Backup). The digest is computed over the raw
// file bytes and compared against handle.Digest (with "sha256:" prefix stripped
// if present).
//
// Restore rejects non-empty target journals (ErrRestoreNotEmpty) and returns
// ErrRestoreMismatch when the digest verification fails. On parse error the
// journal is left empty (all writes happen in a single db.Update tx).
func (s *Store) Restore(ctx context.Context, handle ports.BackupHandle) error {
	_ = ctx

	s.mu.Lock()
	defer s.mu.Unlock()

	// Reject non-empty journal.
	head, err := s.readHeadRLocked()
	if err != nil {
		return fmt.Errorf("restore check head: %w", err)
	}
	if head > 0 {
		return ports.ErrRestoreNotEmpty
	}

	if handle.Path == "" {
		return fmt.Errorf("restore: backup handle has no path")
	}

	// Open backup file and compute digest incrementally.
	f, err := os.Open(handle.Path)
	if err != nil {
		return fmt.Errorf("restore open: %w", err)
	}
	defer f.Close()

	hasher := sha256.New()
	_, err = io.Copy(hasher, f)
	if err != nil {
		return fmt.Errorf("restore read: %w", err)
	}

	computed := hasher.Sum(nil)
	// handle.Digest has "sha256:" prefix; strip it for comparison.
	wantHex := handle.Digest
	if strings.HasPrefix(wantHex, "sha256:") {
		wantHex = wantHex[len("sha256:"):]
	}
	gotHex := fmt.Sprintf("%x", computed)
	if gotHex != wantHex {
		return fmt.Errorf("%w: want %s, got %s", ports.ErrRestoreMismatch, wantHex, gotHex)
	}

	// Digest verified. Re-open and replay into journal under single tx.
	f2, err := os.Open(handle.Path)
	if err != nil {
		return fmt.Errorf("restore reopen: %w", err)
	}
	defer f2.Close()

	return s.db.Update(func(tx *bolt.Tx) error {
		scanner := bufio.NewScanner(f2)
		var lineNum int
		for scanner.Scan() {
			line := scanner.Bytes()
			if len(line) == 0 {
				continue
			}
			var e ports.RawEvent
			if err := json.Unmarshal(line, &e); err != nil {
				return fmt.Errorf("restore parse line %d: %w", lineNum, err)
			}
			// Use appendOneEvent directly inside the tx — it writes event
			// data directly into the tx's buckets without needing a separate
			// db.Update wrapper.
			_, err := s.appendOneEvent(tx, e, nil)
			if err != nil {
				return fmt.Errorf("restore append event %s: %w", e.EventID, err)
			}
			lineNum++
		}
		if err := scanner.Err(); err != nil {
			return fmt.Errorf("restore scan: %w", err)
		}
		return nil
	})
}

// commandIndexRecord is stored in the command_index bucket.
type commandIndexRecord struct {
	CommandID   string   `json:"command_id"`
	EventIDs    []string `json:"event_ids"`
	Position    uint64   `json:"position"`
	Tenant      string   `json:"tenant"`
	ActorType   string   `json:"actor_type"`
	ActorID     string   `json:"actor_id"`
	Correlation string   `json:"correlation"`
	Fingerprint string   `json:"fingerprint,omitempty"`
}

// AppendCommand implements ports.CommandJournal.
// It atomically appends events and indexes the command under command_id.
// If command_id already exists with matching fingerprint, returns cached receipt (Duplicate=true).
// If command_id exists with different fingerprint, returns ErrCommandMismatch.
func (s *Store) AppendCommand(ctx context.Context, cmd ports.CommandRecord, events []ports.RawEvent) (ports.CommandJournalReceipt, error) {
	if cmd.CommandID == "" {
		return ports.CommandJournalReceipt{}, errors.New("command_id is mandatory")
	}
	if len(events) == 0 {
		return ports.CommandJournalReceipt{}, errors.New("events is mandatory")
	}
	_ = ctx

	s.mu.Lock()
	defer s.mu.Unlock()

	var receipt ports.CommandJournalReceipt
	err := s.db.Update(func(tx *bolt.Tx) error {
		cmdIdx := tx.Bucket([]byte(bucketCommandIndex))
		key := []byte(cmd.CommandID)

		// Check for existing command_id
		if existing := cmdIdx.Get(key); existing != nil {
			var rec commandIndexRecord
			if err := json.Unmarshal(existing, &rec); err != nil {
				return fmt.Errorf("unmarshal command index: %w", err)
			}
			// Compare fingerprint to detect payload mismatch
			if rec.Fingerprint != "" && cmd.Fingerprint != "" && rec.Fingerprint != cmd.Fingerprint {
				return ports.ErrCommandMismatch
			}
			// Return cached receipt (idempotent retry)
			receipt = ports.CommandJournalReceipt{
				CommandID:   rec.CommandID,
				EventIDs:    rec.EventIDs,
				Position:    ports.StreamPosition(rec.Position),
				Tenant:      ports.TenantID(rec.Tenant),
				Actor:       ports.Actor{Type: rec.ActorType, ID: rec.ActorID},
				Correlation: rec.Correlation,
				Duplicate:   true,
			}
			return nil
		}

		// Build eventIDs and track positions
		eventIDs := make([]string, 0, len(events))
		var maxPos uint64

		// Write events using appendOneEvent logic but without the id_index duplicate check
		// (we're already checking command-level idempotency)
		eventsBucket := tx.Bucket([]byte(bucketEvents))
		idIdx := tx.Bucket([]byte(bucketIDIndex))

		for _, e := range events {
			// Validate event
			if err := validateEvent(e); err != nil {
				return err
			}

			// Check id_index for duplicate event (separate from command-level idempotency)
			if posBytes := idIdx.Get([]byte(e.EventID)); posBytes != nil {
				pos := decodeUint64BE(posBytes)
				eventIDs = append(eventIDs, e.EventID)
				if pos > maxPos {
					maxPos = pos
				}
				continue
			}

			// Read current head
			head, err := readHead(tx)
			if err != nil {
				return err
			}
			newPos := head + 1

			// Write to id_index
			if err := idIdx.Put([]byte(e.EventID), encodeUint64BE(newPos)); err != nil {
				return fmt.Errorf("store id index: %w", err)
			}

			// Write event
			data, merr := json.Marshal(e)
			if merr != nil {
				return fmt.Errorf("marshal event: %w", merr)
			}
			if err := eventsBucket.Put(encodeUint64BE(newPos), data); err != nil {
				return fmt.Errorf("store event: %w", err)
			}

			// Update head
			if err := writeHead(tx, newPos); err != nil {
				return err
			}

			eventIDs = append(eventIDs, e.EventID)
			maxPos = newPos

			// Update streams bucket
			streamsBucket := tx.Bucket([]byte(bucketStreams))
			streamKey := streamKey(e.TenantID, e.StreamID)
			var streamVersion uint64
			if v := streamsBucket.Get(streamKey); v != nil {
				streamVersion = decodeUint64BE(v)
			}
			newStreamVersion := streamVersion + 1
			if err := streamsBucket.Put(streamKey, encodeUint64BE(newStreamVersion)); err != nil {
				return fmt.Errorf("update stream version: %w", err)
			}
			versionKey := streamVersionKey(e.TenantID, e.StreamID, newStreamVersion)
			if err := streamsBucket.Put(versionKey, encodeUint64BE(newPos)); err != nil {
				return fmt.Errorf("store stream version key: %w", err)
			}

			if err := incrementCounter(tx, counterEventCount); err != nil {
				return err
			}
		}

		// Write command index record
		rec := commandIndexRecord{
			CommandID:   cmd.CommandID,
			EventIDs:    eventIDs,
			Position:    maxPos,
			Tenant:      string(cmd.TenantID),
			ActorType:   cmd.Actor.Type,
			ActorID:     cmd.Actor.ID,
			Correlation: cmd.CorrelationID,
			Fingerprint: cmd.Fingerprint,
		}
		recData, err := json.Marshal(rec)
		if err != nil {
			return fmt.Errorf("marshal command index record: %w", err)
		}
		if err := cmdIdx.Put(key, recData); err != nil {
			return fmt.Errorf("store command index: %w", err)
		}

		receipt = ports.CommandJournalReceipt{
			CommandID:   cmd.CommandID,
			EventIDs:    eventIDs,
			Position:    ports.StreamPosition(maxPos),
			Tenant:      cmd.TenantID,
			Actor:       cmd.Actor,
			Correlation: cmd.CorrelationID,
			Duplicate:   false,
		}
		return nil
	})
	if err != nil {
		return ports.CommandJournalReceipt{}, err
	}
	return receipt, nil
}

// readHeadRLocked reads head; caller must hold s.mu.
func (s *Store) readHeadRLocked() (uint64, error) {
	var head uint64
	err := s.db.View(func(tx *bolt.Tx) error {
		h, err := readHead(tx)
		if err != nil {
			return err
		}
		head = h
		return nil
	})
	return head, err
}

// validateEvent checks envelope invariants before persisting.
func validateEvent(e ports.RawEvent) error {
	switch {
	case e.TenantID == "":
		return ports.ErrEmptyTenant
	case e.EventID == "":
		return ports.ErrEmptyEventID
	case e.Actor.Type == "" || e.Actor.ID == "":
		return ports.ErrEmptyActor
	case e.OccurredAt.IsZero():
		return ports.ErrZeroTimestamp
	case !isValidEventType(e.EventType):
		return ports.ErrInvalidEventType
	}
	return nil
}

// isValidEventType enforces <context>.<entity>.<verb>.v<major> with at least
// context.entity.verb and a v<digits> major suffix.
func isValidEventType(t string) bool {
	parts := splitEventType(t)
	if len(parts) < 4 {
		return false
	}
	last := parts[len(parts)-1]
	if len(last) < 2 || last[0] != 'v' {
		return false
	}
	for _, c := range last[1:] {
		if c < '0' || c > '9' {
			return false
		}
	}
	for _, p := range parts[:len(parts)-1] {
		if p == "" {
			return false
		}
	}
	return true
}

func splitEventType(t string) []string {
	return strings.FieldsFunc(t, func(r rune) bool { return r == '.' })
}
