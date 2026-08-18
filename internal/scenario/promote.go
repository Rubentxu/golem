package scenario

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Rubentxu/golem/internal/ports"
)

// PromoteResult reports a successful promotion.
type PromoteResult struct {
	ScenarioID    string
	EventsApplied int
	PromotedAt    ports.StreamPosition
}

// Promote verifies lineage, requires approval, then appends the overlay
// batch to the real journal atomically (one Append call — the port's
// atomicity contract) and emits scenario.promoted.v1 on stream
// scenario.{id}.
//
// Lineage: base_position must be ≤ the current journal head. Forking from
// a future position (or a head that moved backwards) is a conflict.
// There is no semantic auto-merge: the overlay delta is applied as-is
// (SCENARIOS_FORK_DIFF_PROMOTE.md).
func Promote(ctx context.Context, journal ports.JournalStore, ids ports.IDGenerator, clock ports.Clock, s *ports.Scenario) (PromoteResult, error) {
	if !s.Approved {
		return PromoteResult{}, fmt.Errorf("%w: scenario %s not approved", ErrScenarioConflict, s.ID)
	}
	head, err := journal.Head(ctx)
	if err != nil {
		return PromoteResult{}, fmt.Errorf("scenario: journal head: %w", err)
	}
	if s.BasePosition > head {
		return PromoteResult{}, fmt.Errorf("%w: base_position %d is ahead of journal head %d",
			ErrScenarioConflict, s.BasePosition, head)
	}

	events := make([]ports.RawEvent, 0, len(s.Overlay)+1)
	for _, raw := range s.Overlay {
		var env ports.RawEvent
		if err := json.Unmarshal(raw, &env); err != nil {
			return PromoteResult{}, fmt.Errorf("scenario: overlay decode: %w", err)
		}
		// Lineage travels with every promoted event.
		events = append(events, env)
	}

	promotedPayload, _ := json.Marshal(map[string]any{
		"scenario_id":    s.ID,
		"base_position":  uint64(s.BasePosition),
		"events_applied": len(events),
		"promoted_by":    "scenario.promote",
	})
	promoted := ports.RawEvent{
		EventID:       ids.NewID(),
		TenantID:      string(s.TenantID),
		StreamID:      "scenario." + s.ID,
		EventType:     ports.EventScenarioPromoted,
		SchemaVersion: 1,
		OccurredAt:    clock.Now(),
		Actor:         ports.Actor{Type: "service", ID: "scenario-promoter"},
		Payload:       promotedPayload,
	}
	events = append(events, promoted)

	results, err := journal.Append(ctx, events)
	if err != nil {
		return PromoteResult{}, fmt.Errorf("scenario: promote batch: %w", err)
	}
	if len(results) == 0 {
		return PromoteResult{}, errors.New("scenario: promote batch returned no results")
	}
	return PromoteResult{
		ScenarioID:    s.ID,
		EventsApplied: len(s.Overlay),
		PromotedAt:    results[len(results)-1].Position,
	}, nil
}
