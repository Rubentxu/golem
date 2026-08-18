package bbolt

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Rubentxu/golem/internal/ports"
	bolt "go.etcd.io/bbolt"
)

// BackupToPath creates a consistent snapshot of the journal and writes it to a
// backup file at backupPath (REQ-DR-001). This method handles the full
// backup-to-disk workflow atomically under a single lock.
func (s *Store) BackupToPath(ctx context.Context, backupPath string) (ports.BackupHandle, error) {
	_ = ctx

	s.mu.Lock()
	defer s.mu.Unlock()

	// Collect all events into a JSON array.
	var buf bytes.Buffer
	buf.WriteByte('[')
	first := true

	var digestStr string
	var sizeBytes int64

	err := s.db.View(func(tx *bolt.Tx) error {
		c := tx.Bucket([]byte(bucketEvents)).Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			if !first {
				buf.WriteByte(',')
			}
			first = false
			buf.Write(v)
		}
		buf.WriteByte(']')

		sum := sha256.Sum256(buf.Bytes())
		digestStr = hex.EncodeToString(sum[:])
		sizeBytes = int64(buf.Len())
		return nil
	})
	if err != nil {
		return ports.BackupHandle{}, fmt.Errorf("backup collect events: %w", err)
	}

	// Compute head.
	head, _ := s.readHeadLocked()

	// Write backup file as JSON: {digest, events}.
	f, err := os.OpenFile(backupPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return ports.BackupHandle{}, fmt.Errorf("backup open: %w", err)
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.Encode(struct {
		Digest string          `json:"digest"`
		Events json.RawMessage `json:"events"`
	}{
		Digest: digestStr,
		Events: buf.Bytes(),
	})

	return ports.BackupHandle{
		ID:        fmt.Sprintf("backup-%d", head),
		Path:      backupPath,
		Digest:    digestStr,
		SizeBytes: sizeBytes,
	}, nil
}

// Restore replays a backup file into an empty journal, verifying sha256 (REQ-DR-001).
// Restore requires the journal to be empty; if it is not, ErrRestoreNotEmpty is returned.
func (s *Store) Restore(ctx context.Context, handle ports.BackupHandle) error {
	_ = ctx

	s.mu.Lock()
	defer s.mu.Unlock()

	// Reject non-empty journal.
	head, err := s.readHeadLocked()
	if err != nil {
		return fmt.Errorf("restore check head: %w", err)
	}
	if head > 0 {
		return ports.ErrRestoreNotEmpty
	}

	if handle.Path == "" {
		return fmt.Errorf("restore: backup handle has no path")
	}

	f, err := os.Open(handle.Path)
	if err != nil {
		return fmt.Errorf("restore open: %w", err)
	}
	defer f.Close()

	var entry struct {
		Digest string          `json:"digest"`
		Events json.RawMessage `json:"events"`
	}
	if err := json.NewDecoder(f).Decode(&entry); err != nil {
		return fmt.Errorf("restore parse: %w", err)
	}

	// Verify digest.
	sum := sha256.Sum256(entry.Events)
	got := hex.EncodeToString(sum[:])
	if got != entry.Digest {
		return fmt.Errorf("%w: want %s, got %s", ports.ErrRestoreMismatch, entry.Digest, got)
	}

	// Replay events into journal.
	var events []ports.RawEvent
	if err := json.Unmarshal(entry.Events, &events); err != nil {
		return fmt.Errorf("restore unmarshal events: %w", err)
	}

	for _, e := range events {
		if err := s.appendEventLocked(e); err != nil {
			return fmt.Errorf("restore append event %s: %w", e.EventID, err)
		}
	}

	return nil
}

// readHeadLocked reads the journal head (caller must hold mu).
func (s *Store) readHeadLocked() (uint64, error) {
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

// appendEventLocked appends a single event (caller must hold mu).
func (s *Store) appendEventLocked(e ports.RawEvent) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		// Check duplicate.
		idIndex := tx.Bucket([]byte(bucketIDIndex))
		if idIndex.Get([]byte(e.EventID)) != nil {
			return nil // already restored, idempotent
		}

		head, err := readHead(tx)
		if err != nil {
			return err
		}
		newPos := head + 1

		// Index by ID.
		if err := idIndex.Put([]byte(e.EventID), encodeUint64BE(newPos)); err != nil {
			return fmt.Errorf("restore id index: %w", err)
		}

		// Store event.
		eventsBucket := tx.Bucket([]byte(bucketEvents))
		data, err := json.Marshal(e)
		if err != nil {
			return fmt.Errorf("restore marshal: %w", err)
		}
		if err := eventsBucket.Put(encodeUint64BE(newPos), data); err != nil {
			return fmt.Errorf("restore store event: %w", err)
		}

		// Update head.
		if err := writeHead(tx, newPos); err != nil {
			return err
		}

		// Update stream index.
		streamsBucket := tx.Bucket([]byte(bucketStreams))
		streamKey := streamKey(string(e.TenantID), e.StreamID)
		var streamVersion uint64
		if sv := streamsBucket.Get(streamKey); sv != nil {
			streamVersion = decodeUint64BE(sv)
		}
		newStreamVersion := streamVersion + 1
		if err := streamsBucket.Put(streamKey, encodeUint64BE(newStreamVersion)); err != nil {
			return fmt.Errorf("restore stream index: %w", err)
		}
		versionKey := streamVersionKey(string(e.TenantID), e.StreamID, newStreamVersion)
		if err := streamsBucket.Put(versionKey, encodeUint64BE(newPos)); err != nil {
			return fmt.Errorf("restore version key: %w", err)
		}

		return incrementCounter(tx, counterEventCount)
	})
}

// BackupFilePath returns the default backup file path for this journal.
func BackupFilePath(dbPath string) string {
	dir := filepath.Dir(dbPath)
	file := filepath.Base(dbPath)
	return filepath.Join(dir, file+".backup")
}
