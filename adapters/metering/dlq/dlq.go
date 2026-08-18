// Package dlq provides a dead-letter queue for failed metering events.
package dlq

import (
	"context"
	"sync"

	"github.com/Rubentxu/golem/internal/ports"
)

// Queue stores failed metering events for later replay.
type Queue struct {
	events []ports.MeteringEvent
	mu     sync.RWMutex
}

// NewQueue creates a new DLQ.
func NewQueue() *Queue {
	return &Queue{
		events: make([]ports.MeteringEvent, 0, 1000),
	}
}

// Add adds a failed metering event to the DLQ.
func (q *Queue) Add(ctx context.Context, event ports.MeteringEvent) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.events = append(q.events, event)
	return nil
}

// Replay returns all DLQ events and clears the queue.
func (q *Queue) Replay(ctx context.Context) ([]ports.MeteringEvent, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	events := make([]ports.MeteringEvent, len(q.events))
	copy(events, q.events)

	// Clear the queue.
	q.events = q.events[:0]

	return events, nil
}

// Size returns the number of events in the DLQ.
func (q *Queue) Size(ctx context.Context) (int, error) {
	q.mu.RLock()
	defer q.mu.RUnlock()

	return len(q.events), nil
}

// Ensure Queue implements DLQ interface.
var _ = (*MeteringDLQ)(nil)

// MeteringDLQ is implemented by Queue.
type MeteringDLQ interface {
	Add(ctx context.Context, event ports.MeteringEvent) error
	Replay(ctx context.Context) ([]ports.MeteringEvent, error)
	Size(ctx context.Context) (int, error)
}
