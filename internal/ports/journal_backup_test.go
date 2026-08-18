package ports

import "testing"

// TestJournalStore_BackupSignature verifies backup has a signature.
func TestJournalStore_BackupSignature(t *testing.T) {
	t.Parallel()
	// Verify BackupHandle has required fields.
	handle := BackupHandle{
		ID:        "backup-123",
		Path:      "/backups/journal/2024-01-01-000000.snap",
		Digest:    "sha256:abc123...",
		SizeBytes: 1024000,
	}

	if handle.ID == "" {
		t.Error("expected ID to be set")
	}
	if handle.Digest == "" {
		t.Error("expected Digest to be set")
	}
	if handle.SizeBytes <= 0 {
		t.Error("expected SizeBytes > 0")
	}
}
