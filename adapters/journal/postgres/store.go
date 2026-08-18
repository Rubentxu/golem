// Package postgres provides a PostgreSQL-backed JournalStore adapter.
// It persists the Graph Journal to PostgreSQL using a pool of connections.
//
// ADR-046, ADR-052, ADR-057. lib/pq (MIT).
package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/Rubentxu/golem/internal/ports"
)

// ErrVersionConflict is returned by AppendIf when the stream version
// does not match the expected version.
var ErrVersionConflict = ports.ErrVersionConflict

// Store is a PostgreSQL-backed JournalStore.
type Store struct {
	pool *Pool
	mu   sync.Mutex // serializes writers
}

// NewStore creates a Store from a connection string.
func NewStore(ctx context.Context, connString string) (*Store, error) {
	pool, err := NewPool(ctx, connString)
	if err != nil {
		return nil, fmt.Errorf("postgres.NewStore: %w", err)
	}
	s := &Store{pool: pool}
	if err := s.migrate(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("postgres.NewStore migrate: %w", err)
	}
	return s, nil
}

// Options configures the Store.
type Options struct {
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime int // seconds
}

// Close closes the connection pool.
func (s *Store) Close() error {
	return s.pool.Close()
}

// Append persists the batch atomically. Events with duplicate event_id
// are reported as duplicates (idempotent).
func (s *Store) Append(ctx context.Context, events []ports.RawEvent) ([]ports.AppendResult, error) {
	if len(events) == 0 {
		return nil, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.pool.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	results := make([]ports.AppendResult, 0, len(events))
	for _, e := range events {
		if err := validateEvent(e); err != nil {
			return nil, err
		}

		// Check duplicate by event ID.
		var existingPos int64
		err := tx.QueryRowContext(ctx, "SELECT position FROM id_index WHERE event_id = $1", e.EventID).Scan(&existingPos)
		if err == nil {
			// Duplicate found.
			results = append(results, ports.AppendResult{
				EventID:   e.EventID,
				Position:  ports.StreamPosition(existingPos),
				Duplicate: true,
			})
			continue
		}

		// Advance head atomically.
		var newPos int64
		err = tx.QueryRowContext(ctx, `
			UPDATE meta SET value = value + 1
			WHERE name = 'head'
			RETURNING value`).Scan(&newPos)
		if err != nil {
			return nil, fmt.Errorf("advance head: %w", err)
		}

		// Insert event.
		data, _ := json.Marshal(e)
		_, err = tx.ExecContext(ctx, `
			INSERT INTO events (tenant_id, stream_id, position, data)
			VALUES ($1, $2, $3, $4)`,
			e.TenantID, e.StreamID, newPos, data)
		if err != nil {
			return nil, fmt.Errorf("insert event: %w", err)
		}

		// Update id_index.
		_, err = tx.ExecContext(ctx, `
			INSERT INTO id_index (event_id, position) VALUES ($1, $2)
			ON CONFLICT DO NOTHING`, e.EventID, newPos)
		if err != nil {
			return nil, fmt.Errorf("update id_index: %w", err)
		}

		// Advance stream version.
		var currentVersion int64
		err = tx.QueryRowContext(ctx, `
			SELECT version FROM streams
			WHERE tenant_id = $1 AND stream_id = $2`,
			e.TenantID, e.StreamID).Scan(&currentVersion)
		if err != nil {
			currentVersion = 0
		}
		newVersion := currentVersion + 1

		_, err = tx.ExecContext(ctx, `
			INSERT INTO streams (tenant_id, stream_id, version, position)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (tenant_id, stream_id) DO UPDATE SET version = $3, position = $4`,
			e.TenantID, e.StreamID, newVersion, newPos)
		if err != nil {
			return nil, fmt.Errorf("update stream: %w", err)
		}

		// Increment event count.
		_, _ = tx.ExecContext(ctx, `
			UPDATE meta SET value = value + 1 WHERE name = 'event_count'`)

		results = append(results, ports.AppendResult{
			EventID:  e.EventID,
			Position: ports.StreamPosition(newPos),
		})
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	return results, nil
}

// AppendIf persists conditionally: it succeeds only when the stream currently
// holds exactly expected.Version events; otherwise returns ErrVersionConflict
// without persisting.
func (s *Store) AppendIf(ctx context.Context, expected ports.StreamVersion, events []ports.RawEvent) ([]ports.AppendResult, error) {
	if len(events) == 0 {
		return nil, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.pool.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	// Read-precondition: verify stream version matches expected.
	var currentVersion int64
	err = tx.QueryRowContext(ctx, `
		SELECT COALESCE(version, 0) FROM streams
		WHERE tenant_id = $1 AND stream_id = $2`,
		expected.TenantID, expected.StreamID).Scan(&currentVersion)
	if err != nil {
		currentVersion = 0
	}
	if currentVersion != int64(expected.Version) {
		return nil, ErrVersionConflict
	}

	results := make([]ports.AppendResult, 0, len(events))
	for _, e := range events {
		if err := validateEvent(e); err != nil {
			return nil, err
		}

		// Check duplicate within this batch.
		var existingPos int64
		err := tx.QueryRowContext(ctx, "SELECT position FROM id_index WHERE event_id = $1", e.EventID).Scan(&existingPos)
		if err == nil {
			results = append(results, ports.AppendResult{
				EventID:   e.EventID,
				Position:  ports.StreamPosition(existingPos),
				Duplicate: true,
			})
			continue
		}

		// Advance head.
		var newPos int64
		err = tx.QueryRowContext(ctx, `
			UPDATE meta SET value = value + 1
			WHERE name = 'head'
			RETURNING value`).Scan(&newPos)
		if err != nil {
			return nil, fmt.Errorf("advance head: %w", err)
		}

		// Insert event.
		data, _ := json.Marshal(e)
		_, err = tx.ExecContext(ctx, `
			INSERT INTO events (tenant_id, stream_id, position, data)
			VALUES ($1, $2, $3, $4)`,
			e.TenantID, e.StreamID, newPos, data)
		if err != nil {
			return nil, fmt.Errorf("insert event: %w", err)
		}

		// Update id_index.
		_, err = tx.ExecContext(ctx, `
			INSERT INTO id_index (event_id, position) VALUES ($1, $2)
			ON CONFLICT DO NOTHING`, e.EventID, newPos)
		if err != nil {
			return nil, fmt.Errorf("update id_index: %w", err)
		}

		// Advance stream version.
		newVersion := int64(expected.Version) + 1
		_, err = tx.ExecContext(ctx, `
			INSERT INTO streams (tenant_id, stream_id, version, position)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (tenant_id, stream_id) DO UPDATE SET version = $3, position = $4`,
			e.TenantID, e.StreamID, newVersion, newPos)
		if err != nil {
			return nil, fmt.Errorf("update stream: %w", err)
		}

		_, _ = tx.ExecContext(ctx, `UPDATE meta SET value = value + 1 WHERE name = 'event_count'`)

		results = append(results, ports.AppendResult{
			EventID:  e.EventID,
			Position: ports.StreamPosition(newPos),
		})
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	return results, nil
}

// ReadStream returns events for one tenant/stream with version > fromVersion.
func (s *Store) ReadStream(ctx context.Context, tenant ports.TenantID, streamID string, fromVersion uint64) ([]ports.RawEvent, error) {
	if tenant == "" {
		return nil, ports.ErrEmptyTenant
	}

	rows, err := s.pool.QueryContext(ctx, `
		SELECT data FROM events
		WHERE tenant_id = $1 AND stream_id = $2
		  AND position IN (
			  SELECT position FROM streams
			  WHERE tenant_id = $1 AND stream_id = $2
				AND version > $3
			  ORDER BY version ASC
		  )
		ORDER BY position ASC`, tenant, streamID, fromVersion)
	if err != nil {
		return nil, fmt.Errorf("read stream: %w", err)
	}
	defer rows.Close()

	var events []ports.RawEvent
	for rows.Next() {
		var data []byte
		if err := rows.Scan(&data); err != nil {
			return nil, fmt.Errorf("scan event: %w", err)
		}
		var e ports.RawEvent
		if err := json.Unmarshal(data, &e); err != nil {
			return nil, fmt.Errorf("unmarshal event: %w", err)
		}
		events = append(events, e)
	}
	return events, rows.Err()
}

// Replay returns events with position > from, up to limit (0 = all),
// and the position of the last returned event.
func (s *Store) Replay(ctx context.Context, from ports.StreamPosition, limit int) ([]ports.RawEvent, ports.StreamPosition, error) {
	var events []ports.RawEvent
	var lastPos ports.StreamPosition

	var head int64
	err := s.pool.QueryRowContext(ctx, `
		SELECT COALESCE(value, 0) FROM meta WHERE name = 'head'`).Scan(&head)
	if err != nil {
		return nil, 0, fmt.Errorf("read head: %w", err)
	}

	start := int(from)
	end := int(head)
	if limit > 0 && start+limit < end {
		end = start + limit
	}

	rows, err := s.pool.QueryContext(ctx, `
		SELECT position, data FROM events
		WHERE position > $1 AND position <= $2
		ORDER BY position ASC`, start, end)
	if err != nil {
		return nil, 0, fmt.Errorf("replay query: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var pos int64
		var data []byte
		if err := rows.Scan(&pos, &data); err != nil {
			return nil, 0, fmt.Errorf("scan event: %w", err)
		}
		var e ports.RawEvent
		if err := json.Unmarshal(data, &e); err != nil {
			return nil, 0, fmt.Errorf("unmarshal event: %w", err)
		}
		events = append(events, e)
		lastPos = ports.StreamPosition(pos)
	}

	return events, lastPos, rows.Err()
}

// Head returns the position of the newest event (0 when empty).
func (s *Store) Head(ctx context.Context) (ports.StreamPosition, error) {
	var head int64
	err := s.pool.QueryRowContext(ctx, `
		SELECT COALESCE(value, 0) FROM meta WHERE name = 'head'`).Scan(&head)
	if err != nil {
		return 0, fmt.Errorf("head: %w", err)
	}
	return ports.StreamPosition(head), nil
}

// Backup creates a consistent snapshot handle (REQ-DR-001).
// For postgres, we export all events as JSON and compute the SHA-256 digest.
func (s *Store) Backup(ctx context.Context) (ports.BackupHandle, error) {
	rows, err := s.pool.QueryContext(ctx, `
		SELECT data FROM events ORDER BY position ASC`)
	if err != nil {
		return ports.BackupHandle{}, fmt.Errorf("backup query: %w", err)
	}
	defer rows.Close()

	var allData []byte
	for rows.Next() {
		var data []byte
		if err := rows.Scan(&data); err != nil {
			return ports.BackupHandle{}, fmt.Errorf("scan: %w", err)
		}
		allData = append(allData, data...)
		allData = append(allData, '\n')
	}
	if err := rows.Err(); err != nil {
		return ports.BackupHandle{}, fmt.Errorf("backup rows: %w", err)
	}

	h := sha256.Sum256(allData)
	digest := fmt.Sprintf("sha256:%x", h)

	head, _ := s.Head(ctx)

	return ports.BackupHandle{
		ID:        fmt.Sprintf("pg-backup-%d", head),
		Path:      "postgres", // opaque identifier
		Digest:    digest,
		SizeBytes: int64(len(allData)),
	}, nil
}

// Restore is not implemented for postgres (use pg_dump/pg_restore).
func (s *Store) Restore(ctx context.Context, handle ports.BackupHandle) error {
	return fmt.Errorf("postgres: Restore not implemented; use pg_dump/pg_restore for DR")
}

// migrate creates the schema if it doesn't exist.
func (s *Store) migrate(ctx context.Context) error {
	schema := `
	CREATE TABLE IF NOT EXISTS meta (
		name  VARCHAR(64) PRIMARY KEY,
		value BIGINT NOT NULL DEFAULT 0
	);
	INSERT INTO meta (name, value) VALUES ('head', 0), ('event_count', 0)
	ON CONFLICT (name) DO NOTHING;

	CREATE TABLE IF NOT EXISTS events (
		id         BIGSERIAL PRIMARY KEY,
		tenant_id  VARCHAR(255) NOT NULL,
		stream_id  VARCHAR(255) NOT NULL,
		position   BIGINT NOT NULL,
		data       JSONB NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_events_tenant_stream_pos
		ON events (tenant_id, stream_id, position);
	CREATE INDEX IF NOT EXISTS idx_events_position
		ON events (position);

	CREATE TABLE IF NOT EXISTS id_index (
		event_id  VARCHAR(255) PRIMARY KEY,
		position  BIGINT NOT NULL
	);

	CREATE TABLE IF NOT EXISTS streams (
		tenant_id VARCHAR(255) NOT NULL,
		stream_id VARCHAR(255) NOT NULL,
		version   BIGINT NOT NULL,
		position  BIGINT NOT NULL,
		PRIMARY KEY (tenant_id, stream_id)
	);
	`
	_, err := s.pool.ExecContext(ctx, schema)
	return err
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
	}
	return nil
}
