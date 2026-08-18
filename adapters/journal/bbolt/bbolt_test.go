package bbolt

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/Rubentxu/golem/internal/ports"
	"github.com/Rubentxu/golem/tck"
)

// TestJournalConformance runs the full JournalStore TCK against the bbolt adapter.
func TestJournalConformance(t *testing.T) {
	newStore := func() ports.JournalStore {
		tmp, err := os.CreateTemp("", "golem-bbolt-*.db")
		if err != nil {
			t.Fatalf("CreateTemp: %v", err)
		}
		tmp.Close()
		t.Cleanup(func() { os.Remove(tmp.Name()) })

		store, err := NewJournal(tmp.Name(), Options{FileMode: 0600})
		if err != nil {
			t.Fatalf("NewJournal: %v", err)
		}
		return store
	}

	tck.RunJournalStoreTCK(t, newStore)
}

// TestJournalBboltReopen verifies that a bbolt journal survives file reopen.
func TestJournalBboltReopen(t *testing.T) {
	tmp, err := os.CreateTemp("", "golem-bbolt-reopen-*.db")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	tmp.Close()
	path := tmp.Name()
	t.Cleanup(func() { os.Remove(path) })

	ctx := context.Background()
	now := time.Now().UTC()
	mk := func(n int) ports.RawEvent {
		return ports.RawEvent{
			EventID:       validID(n),
			TenantID:      "t1",
			StreamID:      "s1",
			EventType:     "work.item.created.v1",
			SchemaVersion: 1,
			OccurredAt:    now,
			Actor:         ports.Actor{Type: "user", ID: "u_1"},
			Payload:       []byte(`{"n":` + itoa(n) + `}`),
		}
	}

	// Open, append, close.
	s1, err := NewJournal(path, Options{FileMode: 0600})
	if err != nil {
		t.Fatalf("NewJournal: %v", err)
	}
	if _, err := s1.Append(ctx, []ports.RawEvent{mk(1), mk(2)}); err != nil {
		t.Fatalf("append: %v", err)
	}
	if err := s1.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Re-open and replay.
	s2, err := NewJournal(path, Options{FileMode: 0600})
	if err != nil {
		t.Fatalf("NewJournal reopen: %v", err)
	}
	defer s2.Close()

	evs, last, err := s2.Replay(ctx, 0, 0)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if len(evs) != 2 {
		t.Fatalf("replay: got %d events, want 2", len(evs))
	}
	if last != 2 {
		t.Fatalf("last position: got %d, want 2", last)
	}

	h, err := s2.Head(ctx)
	if err != nil {
		t.Fatalf("head: %v", err)
	}
	if h != 2 {
		t.Fatalf("head: got %d, want 2", h)
	}
}

// validID returns a deterministic event ID for testing.
func validID(n int) string {
	const base = "01JTCK000000000000000000"
	return base + itoa(1000 + n)[1:]
}

// itoa converts n to a decimal string.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
