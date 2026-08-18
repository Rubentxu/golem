// Package bbolt provides a bbolt-backed JournalStore adapter.
// It persists the Graph Journal to a local file using bbolt's
// append-only bucket structure.
//
// ADR-046, ADR-052, ADR-057. go.etcd.io/bbolt v1.5.0 (MIT, pure-Go).
package bbolt

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/Rubentxu/golem/internal/ports"
	bolt "go.etcd.io/bbolt"
)

// ErrVersionConflict is returned by AppendIf when the stream version
// does not match the expected version.
var ErrVersionConflict = ports.ErrVersionConflict

// Store is a bbolt-backed JournalStore. All writes are serialized by mu.
type Store struct {
	db *bolt.DB
	mu sync.Mutex // serializes writers; bbolt already has an exclusive write lock
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
func (s *Store) Append(ctx context.Context, events []ports.RawEvent) ([]ports.AppendResult, error) {
	if len(events) == 0 {
		return nil, nil
	}
	_ = ctx

	s.mu.Lock()
	defer s.mu.Unlock()

	var results []ports.AppendResult
	err := s.db.Update(func(tx *bolt.Tx) error {
		results = make([]ports.AppendResult, 0, len(events))

		for _, e := range events {
			if err := validateEvent(e); err != nil {
				return err
			}

			// Check for duplicate by event ID.
			idIndex := tx.Bucket([]byte(bucketIDIndex))
			if posBytes := idIndex.Get([]byte(e.EventID)); posBytes != nil {
				pos := decodeUint64BE(posBytes)
				results = append(results, ports.AppendResult{
					EventID:   e.EventID,
					Position:  ports.StreamPosition(pos),
					Duplicate: true,
				})
				continue
			}

			// Read current head.
			head, err := readHead(tx)
			if err != nil {
				return err
			}
			newPos := head + 1

			// Store in id_index: eventID → position.
			if err := idIndex.Put([]byte(e.EventID), encodeUint64BE(newPos)); err != nil {
				return fmt.Errorf("store id index: %w", err)
			}

			// Store event: position → JSON.
			eventsBucket := tx.Bucket([]byte(bucketEvents))
			data, err := json.Marshal(e)
			if err != nil {
				return fmt.Errorf("marshal event: %w", err)
			}
			if err := eventsBucket.Put(encodeUint64BE(newPos), data); err != nil {
				return fmt.Errorf("store event: %w", err)
			}

			// Update head.
			if err := writeHead(tx, newPos); err != nil {
				return err
			}

			// Update stream: increment stream version and store position by version.
			streamsBucket := tx.Bucket([]byte(bucketStreams))
			streamKey := streamKey(string(e.TenantID), e.StreamID)
			streamVersionBytes := streamsBucket.Get(streamKey)
			var streamVersion uint64
			if streamVersionBytes != nil {
				streamVersion = decodeUint64BE(streamVersionBytes)
			}
			newStreamVersion := streamVersion + 1

			// Store new stream version → position.
			if err := streamsBucket.Put(streamKey, encodeUint64BE(newStreamVersion)); err != nil {
				return fmt.Errorf("update stream version: %w", err)
			}
			// Store version key → position.
			versionKey := streamVersionKey(string(e.TenantID), e.StreamID, newStreamVersion)
			if err := streamsBucket.Put(versionKey, encodeUint64BE(newPos)); err != nil {
				return fmt.Errorf("store stream version key: %w", err)
			}

			// Increment event count.
			if err := incrementCounter(tx, counterEventCount); err != nil {
				return err
			}

			results = append(results, ports.AppendResult{
				EventID:  e.EventID,
				Position: ports.StreamPosition(newPos),
			})
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
func (s *Store) AppendIf(ctx context.Context, expected ports.StreamVersion, events []ports.RawEvent) ([]ports.AppendResult, error) {
	if len(events) == 0 {
		return nil, nil
	}
	_ = ctx

	s.mu.Lock()
	defer s.mu.Unlock()

	var results []ports.AppendResult
	err := s.db.Update(func(tx *bolt.Tx) error {
		// Read-precondition: verify stream version matches expected.
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
		for _, e := range events {
			if err := validateEvent(e); err != nil {
				return err
			}

			// Check for duplicate within this batch.
			idIndex := tx.Bucket([]byte(bucketIDIndex))
			if posBytes := idIndex.Get([]byte(e.EventID)); posBytes != nil {
				pos := decodeUint64BE(posBytes)
				results = append(results, ports.AppendResult{
					EventID:   e.EventID,
					Position:  ports.StreamPosition(pos),
					Duplicate: true,
				})
				continue
			}

			// Read current head.
			head, err := readHead(tx)
			if err != nil {
				return err
			}
			newPos := head + 1

			// Store in id_index.
			if err := idIndex.Put([]byte(e.EventID), encodeUint64BE(newPos)); err != nil {
				return fmt.Errorf("store id index: %w", err)
			}

			// Store event.
			eventsBucket := tx.Bucket([]byte(bucketEvents))
			data, err := json.Marshal(e)
			if err != nil {
				return fmt.Errorf("marshal event: %w", err)
			}
			if err := eventsBucket.Put(encodeUint64BE(newPos), data); err != nil {
				return fmt.Errorf("store event: %w", err)
			}

			// Update head.
			if err := writeHead(tx, newPos); err != nil {
				return err
			}

			// Update stream: advance version and store position.
			newStreamVersion := expected.Version + 1
			if err := streamsBucket.Put(streamKey, encodeUint64BE(newStreamVersion)); err != nil {
				return fmt.Errorf("update stream version: %w", err)
			}
			versionKey := streamVersionKey(string(e.TenantID), e.StreamID, newStreamVersion)
			if err := streamsBucket.Put(versionKey, encodeUint64BE(newPos)); err != nil {
				return fmt.Errorf("store stream version key: %w", err)
			}

			// Increment event count.
			if err := incrementCounter(tx, counterEventCount); err != nil {
				return err
			}

			results = append(results, ports.AppendResult{
				EventID:  e.EventID,
				Position: ports.StreamPosition(newPos),
			})
		}
		return nil
	})

	if err != nil {
		return nil, err
	}
	return results, nil
}

// ReadStream returns events for one tenant/stream with version > fromVersion.
func (s *Store) ReadStream(ctx context.Context, tenant ports.TenantID, streamID string, fromVersion uint64) ([]ports.RawEvent, error) {
	_ = ctx
	if tenant == "" {
		return nil, ports.ErrEmptyTenant
	}

	s.mu.Lock()
	defer s.mu.Unlock()

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
func (s *Store) Replay(ctx context.Context, from ports.StreamPosition, limit int) ([]ports.RawEvent, ports.StreamPosition, error) {
	_ = ctx

	s.mu.Lock()
	defer s.mu.Unlock()

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
func (s *Store) Head(ctx context.Context) (ports.StreamPosition, error) {
	_ = ctx

	s.mu.Lock()
	defer s.mu.Unlock()

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
