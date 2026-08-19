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

	"github.com/Rubentxu/golem/internal/obs"
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
	Frame         *ports.Frame // replaces FrameID string (ADR-064)
	Payload       any
}

// FrameID returns the frame ID for backwards compatibility.
// Returns empty string if Frame is nil.
func (c Command) FrameID() string {
	if c.Frame == nil {
		return ""
	}
	return c.Frame.ID
}

// EventDraft is a domain-authored event awaiting envelope assignment. The
// bus assigns identity, time, tenant, actor and correlation so handlers
// stay pure domain logic. ExpectedStreamVersion (optional) makes the
// append conditional — optimistic concurrency (ADR-021).
type EventDraft struct {
	EventType             string
	StreamID              string
	SchemaVersion         int
	Payload               any
	EvidenceRefs          []string
	ExpectedStreamVersion *uint64
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
	obs      ports.Observability

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
		obs:      obs.Fill(ports.Observability{}),
		handlers: map[string]Handler{},
	}
}

// WithObservability sets the instrumentation bundle (chaining).
func (b *Bus) WithObservability(o ports.Observability) *Bus {
	b.obs = obs.Fill(o)
	return b
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

	// Correlation + trace path start here (OBSERVABILITY.md:
	// Command → Journal → outbox → projection).
	ctx = ports.WithCorrelation(ctx, ports.Correlation{
		CorrelationID: correlation,
		TenantID:      string(cmd.TenantID),
		ActorType:     cmd.Actor.Type,
		ActorID:       cmd.Actor.ID,
		CommandID:     commandID,
	})
	ctx, span := b.obs.Tracer.Start(ctx, "golem.command.Submit", ports.A("command", cmd.Name))

	var receipt ports.CommandReceipt
	var err error
	defer func() { span.End(err) }()

	receipt, err = b.submit(ctx, cmd, commandID, correlation)
	if err != nil {
		b.obs.Logger.Error(ctx, "command rejected", ports.A("command", cmd.Name), ports.A("error", err.Error()))
		b.obs.Meter.Counter("golem.commands").Add(ctx, 1, ports.A("result", "rejected"))
		return ports.CommandReceipt{}, err
	}
	if receipt.Duplicate {
		b.obs.Meter.Counter("golem.commands").Add(ctx, 1, ports.A("result", "duplicate"))
	} else {
		b.obs.Logger.Info(ctx, "command accepted", ports.A("command", cmd.Name), ports.A("position", int64(receipt.Position)))
		b.obs.Meter.Counter("golem.commands").Add(ctx, 1, ports.A("result", "accepted"))
	}
	return receipt, nil
}

func (b *Bus) submit(ctx context.Context, cmd Command, commandID, correlation string) (ports.CommandReceipt, error) {

	// Idempotent replay: a known command returns its stored receipt.
	if receipt, found, err := b.registry.Find(ctx, commandID); err != nil {
		return ports.CommandReceipt{}, fmt.Errorf("command %s: registry find: %w", cmd.Name, err)
	} else if found {
		receipt.Duplicate = true
		return receipt, nil
	}

	// Check if the command requires conditional appending (ExpectedStreamVersion).
	// CommandJournal doesn't support conditional appending, so we must use the legacy path.
	// We need to call the handler to know if any draft has ExpectedStreamVersion.
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

	// Check if any draft requires conditional appending.
	hasConditional := false
	for i := range drafts {
		if drafts[i].ExpectedStreamVersion != nil {
			hasConditional = true
			break
		}
	}

	// If conditional appending is required, use legacy path.
	// Also use legacy path if journal doesn't implement CommandJournal.
	if hasConditional {
		ls := &LegacyCommandSubmitter{
			journal:  b.journal,
			registry: b.registry,
			ids:      b.ids,
			clock:    b.clock,
		}
		return ls.Submit(ctx, cmd, commandID, correlation, drafts)
	}

	// For unconditional appends, prefer CommandJournal if available.
	if cj, ok := b.journal.(ports.CommandJournal); ok {
		return b.submitWithJournal(ctx, cmd, commandID, correlation, cj, drafts)
	}

	// Legacy path for unconditional appends when CommandJournal is not available.
	ls := &LegacyCommandSubmitter{
		journal:  b.journal,
		registry: b.registry,
		ids:      b.ids,
		clock:    b.clock,
	}
	return ls.Submit(ctx, cmd, commandID, correlation, drafts)
}

// submitWithJournal uses the CommandJournal interface for atomic command processing.
// It assumes drafts have already been validated and none have ExpectedStreamVersion set.
// The idempotent replay check (registry.Find) is done by the caller (submit).
func (b *Bus) submitWithJournal(ctx context.Context, cmd Command, commandID, correlation string, cj ports.CommandJournal, drafts []EventDraft) (ports.CommandReceipt, error) {
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
			FrameID:       cmd.FrameID(),
			Frame:         cmd.Frame,
			Payload:       payload,
			EvidenceRefs:  d.EvidenceRefs,
		})
	}

	// Build fingerprint from command record + events for collision detection.
	// The fingerprint is used to detect if the same command_id is reused
	// with a different payload (programming error).
	fingerprint := fmt.Sprintf("%s:%d", commandID, len(envelopes))

	cmdRecord := ports.CommandRecord{
		CommandID:     commandID,
		CommandKind:   cmd.Name,
		TenantID:      cmd.TenantID,
		Actor:         cmd.Actor,
		CorrelationID: correlation,
		Fingerprint:   fingerprint,
	}

	jr, err := cj.AppendCommand(ctx, cmdRecord, envelopes)
	if err != nil {
		return ports.CommandReceipt{}, fmt.Errorf("command %s: journal append: %w", cmd.Name, err)
	}

	// Convert CommandJournalReceipt to CommandReceipt.
	receipt := ports.CommandReceipt{
		CommandID: jr.CommandID,
		TenantID:  jr.Tenant,
		EventIDs:  jr.EventIDs,
		Position:  jr.Position,
		Duplicate: jr.Duplicate,
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

// submitLegacy uses the legacy Append + registry.Save path for journals
// that do not implement CommandJournal.
// The idempotent replay check (registry.Find) is done by the caller (submit).
func (b *Bus) submitLegacy(ctx context.Context, cmd Command, commandID, correlation string, drafts []EventDraft) (ports.CommandReceipt, error) {
	ls := &LegacyCommandSubmitter{
		journal:  b.journal,
		registry: b.registry,
		ids:      b.ids,
		clock:    b.clock,
	}
	return ls.Submit(ctx, cmd, commandID, correlation, drafts)
}
