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
//
// Note: BackupToPath uses a JSON-array format {digest, events} and is NOT
// the inverse of Backup(). Use Restore() (which reads NDJSON) for restoring
// backups created by Backup().
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
	head, _ := s.readHeadRLocked()

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

// BackupFilePath returns the default backup file path for this journal.
func BackupFilePath(dbPath string) string {
	dir := filepath.Dir(dbPath)
	file := filepath.Base(dbPath)
	return filepath.Join(dir, file+".backup")
}
