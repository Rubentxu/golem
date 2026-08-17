// Package memstore provides the in-memory reference adapter of the
// JournalStore port. It is the JournalPort reference implementation of the
// delivery sequence: authoritative append-only log with idempotent
// acceptance and checkpointed replay, without any external dependency.
//
// It passes tck.RunJournalStoreTCK and is the baseline every real adapter
// (SP-002 candidates) must be semantically equivalent to.
package memstore

import (
	"context"
	"strings"
	"sync"

	"github.com/Rubentxu/golem/internal/ports"
)

// Store is an in-memory JournalStore. Safe for concurrent use.
type Store struct {
	mu      sync.Mutex
	events  []ports.RawEvent
	byID    map[string]ports.StreamPosition
	streams map[string][]ports.StreamPosition
}

// NewJournal builds an empty journal.
func NewJournal() *Store {
	return &Store{
		byID:    map[string]ports.StreamPosition{},
		streams: map[string][]ports.StreamPosition{},
	}
}

// Append validates then persists the batch atomically. Events whose
// event_id already exists are reported as duplicates (no error, no rewrite).
func (s *Store) Append(ctx context.Context, events []ports.RawEvent) ([]ports.AppendResult, error) {
	_ = ctx
	if len(events) == 0 {
		return nil, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	// Validate the whole batch before mutating anything: all-or-nothing.
	for i := range events {
		if err := validate(events[i]); err != nil {
			return nil, err
		}
	}

	results := make([]ports.AppendResult, 0, len(events))
	for i := range events {
		e := events[i]
		if pos, dup := s.byID[e.EventID]; dup {
			results = append(results, ports.AppendResult{EventID: e.EventID, Position: pos, Duplicate: true})
			continue
		}
		pos := ports.StreamPosition(len(s.events) + 1)
		s.events = append(s.events, e)
		s.byID[e.EventID] = pos
		key := streamKey(e.TenantID, e.StreamID)
		s.streams[key] = append(s.streams[key], pos)
		results = append(results, ports.AppendResult{EventID: e.EventID, Position: pos})
	}
	return results, nil
}

// ReadStream returns the events of one tenant stream with version >
// fromVersion, in append order (version is the 1-based per-stream ordinal).
func (s *Store) ReadStream(ctx context.Context, tenant ports.TenantID, streamID string, fromVersion uint64) ([]ports.RawEvent, error) {
	_ = ctx
	if tenant == "" {
		return nil, ports.ErrEmptyTenant
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	positions := s.streams[streamKey(string(tenant), streamID)]
	out := []ports.RawEvent{}
	for i, pos := range positions {
		if uint64(i)+1 <= fromVersion {
			continue
		}
		out = append(out, s.events[pos-1])
	}
	return out, nil
}

// Replay returns events with position > from, at most limit (0 = all), and
// the position of the last event returned (a replay checkpoint).
func (s *Store) Replay(ctx context.Context, from ports.StreamPosition, limit int) ([]ports.RawEvent, ports.StreamPosition, error) {
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()

	start := int(from) // positions are 1-based: index = position-1.
	if start >= len(s.events) {
		return nil, ports.StreamPosition(len(s.events)), nil
	}
	end := len(s.events)
	if limit > 0 && start+limit < end {
		end = start + limit
	}
	out := make([]ports.RawEvent, 0, end-start)
	out = append(out, s.events[start:end]...)
	return out, ports.StreamPosition(end), nil
}

func validate(e ports.RawEvent) error {
	switch {
	case e.TenantID == "":
		return ports.ErrEmptyTenant
	case e.EventID == "":
		return ports.ErrEmptyEventID
	case e.Actor.Type == "" || e.Actor.ID == "":
		return ports.ErrEmptyActor
	case e.OccurredAt.IsZero():
		return ports.ErrZeroTimestamp
	case !validEventType(e.EventType):
		return ports.ErrInvalidEventType
	}
	return nil
}

// validEventType enforces <context>.<entity>.<verb>.v<major> with at least
// context.entity.verb and a v<digits> major suffix (EVENT_MODEL).
func validEventType(t string) bool {
	parts := strings.Split(t, ".")
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

func streamKey(tenant, stream string) string { return tenant + "\x00" + stream }
