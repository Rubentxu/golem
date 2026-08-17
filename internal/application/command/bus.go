// Package command implements the kernel write path: a command bus that
// validates commands through domain handlers and journals their events
// atomically, returning idempotent receipts
// (IMPLEMENTATION_SEQUENCE weeks 3–4: "command receipt/idempotency").
//
// Write path (ARCHITECTURE): Command → Domain validation → Journal append
// → Accepted Event → (outbox/projection reactions arrive later).
//
// Known limitation while the transactional outbox lands (weeks 5–6): the
// registry save happens after the journal append. A crash in between can
// re-execute a retried command and journal a second event set; the outbox
// will close this dual-write window by persisting the receipt in the same
// transaction as the events.
package command

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/Rubentxu/golem/internal/ports"
)

var (
	// ErrEmptyName enforces a registered command name.
	ErrEmptyName = errors.New("command: name is mandatory")
	// ErrUnknownCommand reports dispatch of an unregistered command.
	ErrUnknownCommand = errors.New("command: no handler registered")
)

// Command is the write-path input. CommandID may be empty: the bus
// generates one. Clients that want retry-safety supply a stable ID.
type Command struct {
	Name          string
	TenantID      ports.TenantID
	Actor         ports.Actor
	CommandID     string
	CorrelationID string
	CausationID   string
	FrameID       string
	Payload       any
}

// EventDraft is a domain-authored event awaiting envelope assignment. The
// bus assigns identity, time, tenant, actor and correlation so handlers
// stay pure domain logic.
type EventDraft struct {
	EventType     string
	StreamID      string
	SchemaVersion int
	Payload       any
	EvidenceRefs  []string
}

// Handler validates a command and produces its events. Errors are domain
// rejections: nothing is journaled.
type Handler func(ctx context.Context, cmd Command) ([]EventDraft, error)

// Bus dispatches commands to handlers and owns the journal transaction.
type Bus struct {
	journal  ports.JournalStore
	registry ports.CommandRegistry
	ids      ports.IDGenerator
	clock    ports.Clock

	mu       sync.RWMutex
	handlers map[string]Handler
}

// NewBus wires the bus. Handlers are registered before serving traffic.
func NewBus(journal ports.JournalStore, registry ports.CommandRegistry, ids ports.IDGenerator, clock ports.Clock) *Bus {
	return &Bus{
		journal:  journal,
		registry: registry,
		ids:      ids,
		clock:    clock,
		handlers: map[string]Handler{},
	}
}

// Register binds a handler to a command name. Registering twice replaces
// the handler (composition-root setup phase).
func (b *Bus) Register(name string, h Handler) {
	if name == "" {
		panic(ErrEmptyName)
	}
	b.mu.Lock()
	b.handlers[name] = h
	b.mu.Unlock()
}

// Submit processes a command idempotently by CommandID.
func (b *Bus) Submit(ctx context.Context, cmd Command) (ports.CommandReceipt, error) {
	switch {
	case cmd.Name == "":
		return ports.CommandReceipt{}, ErrEmptyName
	case cmd.TenantID == "":
		return ports.CommandReceipt{}, ports.ErrEmptyTenant
	case cmd.Actor.Type == "" || cmd.Actor.ID == "":
		return ports.CommandReceipt{}, ports.ErrEmptyActor
	}

	commandID := cmd.CommandID
	if commandID == "" {
		commandID = b.ids.NewID()
	}
	correlation := cmd.CorrelationID
	if correlation == "" {
		correlation = b.ids.NewID()
	}

	// Idempotent replay: a known command returns its stored receipt.
	if receipt, found, err := b.registry.Find(ctx, commandID); err != nil {
		return ports.CommandReceipt{}, fmt.Errorf("command %s: registry find: %w", cmd.Name, err)
	} else if found {
		receipt.Duplicate = true
		return receipt, nil
	}

	b.mu.RLock()
	handler, ok := b.handlers[cmd.Name]
	b.mu.RUnlock()
	if !ok {
		return ports.CommandReceipt{}, fmt.Errorf("%w: %s", ErrUnknownCommand, cmd.Name)
	}

	drafts, err := handler(ctx, cmd)
	if err != nil {
		return ports.CommandReceipt{}, fmt.Errorf("command %s rejected: %w", cmd.Name, err)
	}
	if len(drafts) == 0 {
		return ports.CommandReceipt{}, fmt.Errorf("command %s: handler produced no events", cmd.Name)
	}

	envelopes := make([]ports.RawEvent, 0, len(drafts))
	for _, d := range drafts {
		payload, err := json.Marshal(d.Payload)
		if err != nil {
			return ports.CommandReceipt{}, fmt.Errorf("command %s: encode payload of %s: %w", cmd.Name, d.EventType, err)
		}
		envelopes = append(envelopes, ports.RawEvent{
			EventID:       b.ids.NewID(),
			TenantID:      string(cmd.TenantID),
			StreamID:      d.StreamID,
			EventType:     d.EventType,
			SchemaVersion: d.SchemaVersion,
			OccurredAt:    b.clock.Now().UTC(),
			Actor:         cmd.Actor,
			CorrelationID: correlation,
			CausationID:   cmd.CausationID,
			CommandID:     commandID,
			FrameID:       cmd.FrameID,
			Payload:       payload,
			EvidenceRefs:  d.EvidenceRefs,
		})
	}

	results, err := b.journal.Append(ctx, envelopes)
	if err != nil {
		return ports.CommandReceipt{}, fmt.Errorf("command %s: journal append: %w", cmd.Name, err)
	}

	receipt := ports.CommandReceipt{
		CommandID: commandID,
		TenantID:  cmd.TenantID,
		EventIDs:  make([]string, 0, len(results)),
	}
	for _, r := range results {
		receipt.EventIDs = append(receipt.EventIDs, r.EventID)
		if r.Position > receipt.Position {
			receipt.Position = r.Position
		}
	}

	// A concurrent duplicate submission may win the Save race: its events
	// are equivalent (same command), so return the stored receipt.
	if err := b.registry.Save(ctx, receipt); err != nil {
		if errors.Is(err, ports.ErrDuplicateCommand) {
			if stored, found, ferr := b.registry.Find(ctx, commandID); ferr == nil && found {
				stored.Duplicate = true
				return stored, nil
			}
		}
		return ports.CommandReceipt{}, fmt.Errorf("command %s: registry save: %w", cmd.Name, err)
	}
	return receipt, nil
}
