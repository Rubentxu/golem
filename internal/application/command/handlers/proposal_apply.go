// Package handlers provides command-level handlers registered on the command Bus.
// ApplyProposal is the command handler that enforces PolicyEvaluator before
// journal.AppendIf (optimistic revision check).
package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	appcmd "github.com/Rubentxu/golem/internal/application/command"
	"github.com/Rubentxu/golem/internal/application/proposal"
	"github.com/Rubentxu/golem/internal/ports"
)

// ErrApplyPolicyDenied is returned when PolicyEvaluator denies the apply action.
var ErrApplyPolicyDenied = errors.New("handlers: apply denied by policy")

// ApplyProposalHandler returns a command handler that evaluates PolicyEvaluator
// before allowing a proposal apply. It uses journal.AppendIf for optimistic
// revision checking — if the stream version has moved, the append fails with
// ErrVersionConflict and the caller should emit proposal.conflicted.v1.
func ApplyProposalHandler(
	store ports.ProposalStore,
	policy ports.PolicyEvaluator,
	journal ports.JournalStore,
	clock ports.Clock,
) appcmd.Handler {
	return func(ctx context.Context, cmd appcmd.Command) ([]appcmd.EventDraft, error) {
		p, ok := cmd.Payload.(proposal.ApplyPayload)
		if !ok {
			return nil, errors.New("handlers: payload must be proposal.ApplyPayload")
		}
		if p.TenantID == "" {
			return nil, proposal.ErrEmptyTenant
		}
		if p.Actor.Type == "" || p.Actor.ID == "" {
			return nil, proposal.ErrEmptyActor
		}
		if p.ProposalID == "" {
			return nil, proposal.ErrEmptyProposalID
		}

		// Load current proposal state
		current, err := store.Get(ctx, p.ProposalID)
		if err != nil {
			return nil, proposal.ErrProposalNotFound
		}
		if current.TenantID != ports.TenantID(p.TenantID) {
			return nil, proposal.ErrProposalNotFound
		}

		// Only approved proposals can be applied
		if current.Status != string(ports.ProposalStatusApproved) {
			return nil, fmt.Errorf("%w: cannot apply proposal in status %s",
				proposal.ErrInvalidStatusTransition, current.Status)
		}

		// Policy gate before any mutation
		action := ports.Action{
			Type:       "proposal.apply",
			Actor:      p.Actor,
			TenantID:   ports.TenantID(p.TenantID),
			Target:     "proposal:" + p.ProposalID,
			Permission: ports.PermissionProposalApply,
		}
		decision, err := policy.Evaluate(ctx, action)
		if err != nil {
			return nil, fmt.Errorf("policy evaluate: %w", err)
		}
		if decision.Outcome != ports.DecisionOutcomeAllow {
			return nil, ErrApplyPolicyDenied
		}

		// Optimistic revision check: read the proposal stream head
		streamID := "proposal:" + p.ProposalID
		streamEvents, err := journal.ReadStream(ctx, ports.TenantID(p.TenantID), streamID, 0)
		if err != nil {
			return nil, fmt.Errorf("journal read stream: %w", err)
		}
		expectedVersion := uint64(len(streamEvents))

		// If caller provided an expected version, use it; otherwise use observed head
		if p.ExpectedVersion > 0 && p.ExpectedVersion < expectedVersion {
			// Version drift detected — return conflict error so caller can emit conflicted.v1
			return nil, ports.ErrVersionConflict
		}

		appliedAt := clock.Now().UTC().Format("2006-01-02T15:04:05Z")

		payload := map[string]any{
			"proposal_id": current.ID,
			"tenant_id":   current.TenantID,
			"operations":  current.Operations,
			"applied_at":  appliedAt,
			"revision":    current.Revision + 1,
		}
		payloadBytes, _ := json.Marshal(payload)

		return []appcmd.EventDraft{{
			EventType:             ports.EventProposalApplied,
			StreamID:              streamID,
			SchemaVersion:         1,
			Payload:               payloadBytes,
			ExpectedStreamVersion: &expectedVersion,
		}}, nil
	}
}
