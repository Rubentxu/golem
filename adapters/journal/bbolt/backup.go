package bbolt

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/Rubentxu/golem/internal/ports"
	bolt "go.etcd.io/bbolt"
)

// BackupToPath creates a consistent snapshot of the journal and writes it to a
// backup file at backupPath (REQ-DR-001). Events are streamed in NDJSON format
// (one JSON object per line) so the output is directly consumable by Restore().
//
// No mutex needed on bbolt operations: bbolt.View() uses MVCC — each call gets
// a consistent snapshot. The internal s.mu is held only to serialize concurrent
// BackupToPath calls, which is required because we open a user-specified path.
func (s *Store) BackupToPath(ctx context.Context, backupPath string) (ports.BackupHandle, error) {
	_ = ctx

	s.mu.Lock()
	defer s.mu.Unlock()

	// Open backup file, creating/truncating atomically.
	f, err := os.OpenFile(backupPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return ports.BackupHandle{}, fmt.Errorf("backup open: %w", err)
	}

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
		return ports.BackupHandle{}, fmt.Errorf("backup stream: %w", err)
	}

	// Close file before computing final stats.
	if err := f.Close(); err != nil {
		return ports.BackupHandle{}, fmt.Errorf("backup close: %w", err)
	}

	// Compute digest from hasher (sha256 of raw file bytes).
	digestStr := fmt.Sprintf("sha256:%x", hasher.Sum(nil))

	// Get file stats for size.
	stat, err := os.Stat(backupPath)
	if err != nil {
		return ports.BackupHandle{}, fmt.Errorf("backup stat: %w", err)
	}

	// Compute head for ID.
	head, _ := s.readHeadRLocked()

	return ports.BackupHandle{
		ID:        fmt.Sprintf("backup-%d", head),
		Path:      backupPath,
		Digest:    digestStr,
		SizeBytes: stat.Size(),
	}, nil
}

// BackupFilePath returns the default backup file path for this journal.
func BackupFilePath(dbPath string) string {
	dir := filepath.Dir(dbPath)
	file := filepath.Base(dbPath)
	return filepath.Join(dir, file+".backup")
}
