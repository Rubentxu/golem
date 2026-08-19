package command

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Rubentxu/golem/internal/ports"
)

// LegacyCommandSubmitter handles the legacy Append + registry.Save path for
// journals that do not implement CommandJournal.
type LegacyCommandSubmitter struct {
	journal  ports.JournalStore
	registry ports.CommandRegistry
	ids      ports.IDGenerator
	clock    ports.Clock
}

// Submit executes the legacy append + registry.Save sequence.
// It is called when the journal does not implement CommandJournal.
func (ls *LegacyCommandSubmitter) Submit(ctx context.Context, cmd Command, commandID, correlation string, drafts []EventDraft) (ports.CommandReceipt, error) {
	envelopes := make([]ports.RawEvent, 0, len(drafts))
	for _, d := range drafts {
		payload, err := json.Marshal(d.Payload)
		if err != nil {
			return ports.CommandReceipt{}, fmt.Errorf("command %s: encode payload of %s: %w", cmd.Name, d.EventType, err)
		}
		envelopes = append(envelopes, ports.RawEvent{
			EventID:       ls.ids.NewID(),
			TenantID:      string(cmd.TenantID),
			StreamID:      d.StreamID,
			EventType:     d.EventType,
			SchemaVersion: d.SchemaVersion,
			OccurredAt:    ls.clock.Now().UTC(),
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

	// Conditional append when any draft carries an expected version.
	var expected *ports.StreamVersion
	for i := range drafts {
		if v := drafts[i].ExpectedStreamVersion; v != nil {
			if expected != nil && (expected.StreamID != drafts[i].StreamID || expected.Version != *v) {
				return ports.CommandReceipt{}, fmt.Errorf("command %s: conflicting stream expectations in one command", cmd.Name)
			}
			expected = &ports.StreamVersion{TenantID: cmd.TenantID, StreamID: drafts[i].StreamID, Version: *v}
		}
	}

	var results []ports.AppendResult
	var err error
	if expected != nil {
		results, err = ls.journal.AppendIf(ctx, *expected, envelopes)
	} else {
		results, err = ls.journal.Append(ctx, envelopes)
	}
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

	// A concurrent duplicate submission may win the Save race.
	if err := ls.registry.Save(ctx, receipt); err != nil {
		if errors.Is(err, ports.ErrDuplicateCommand) {
			if stored, found, ferr := ls.registry.Find(ctx, commandID); ferr == nil && found {
				stored.Duplicate = true
				return stored, nil
			}
		}
		return ports.CommandReceipt{}, fmt.Errorf("command %s: registry save: %w", cmd.Name, err)
	}
	return receipt, nil
}
