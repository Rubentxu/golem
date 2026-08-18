// Package proposal provides application-level handlers for the Proposal lifecycle
// (ADR-065). These handlers are registered on the command.Bus and enforce
// PolicyEvaluator gating and optimistic revision checking.
package proposal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	appcmd "github.com/Rubentxu/golem/internal/application/command"
	"github.com/Rubentxu/golem/internal/ports"
)

// Command names for the proposal lifecycle.
const (
	CmdPropose  = "proposal.propose"
	CmdApprove  = "proposal.approve"
	CmdReject   = "proposal.reject"
	CmdApply    = "proposal.apply"
	CmdConflict = "proposal.conflict"
)

// Domain errors.
var (
	ErrProposalNotFound        = errors.New("proposal: not found")
	ErrInvalidStatusTransition = errors.New("proposal: invalid status transition")
	ErrPolicyDenied            = errors.New("proposal: policy denied")
	ErrEmptyProposalID         = errors.New("proposal: id is mandatory")
	ErrEmptyTenant             = errors.New("proposal: tenant is mandatory")
	ErrEmptyActor              = errors.New("proposal: actor is mandatory")
)

// ProposePayload is the payload of CmdPropose.
type ProposePayload struct {
	ProposalID string
	TenantID   string
	TargetSpec ports.TargetSpec
	Operations []ports.Operation
	Rationale  string
	Actor      ports.Actor
}

// ApprovePayload is the payload of CmdApprove.
type ApprovePayload struct {
	ProposalID string
	TenantID   string
	Actor      ports.Actor
}

// RejectPayload is the payload of CmdReject.
type RejectPayload struct {
	ProposalID string
	TenantID   string
	Reason     string
	Actor      ports.Actor
}

// ApplyPayload is the payload of CmdApply.
type ApplyPayload struct {
	ProposalID      string
	TenantID        string
	ExpectedVersion uint64 // optimistic revision check
	Actor           ports.Actor
}

// ConflictPayload is the payload of CmdConflict.
type ConflictPayload struct {
	ProposalID string
	TenantID   string
	Reason     string
	Actor      ports.Actor
}

// ProposeHandler returns a handler for CmdPropose that creates a proposal
// from the given operation + actor + frame. The handler emits
// proposal.proposed.v1 on the proposal stream.
func ProposeHandler(store ports.ProposalStore, gen ports.IDGenerator, clock ports.Clock) appcmd.Handler {
	return func(ctx context.Context, cmd appcmd.Command) ([]appcmd.EventDraft, error) {
		p, ok := cmd.Payload.(ProposePayload)
		if !ok {
			return nil, errors.New("proposal: payload must be proposal.ProposePayload")
		}
		if p.TenantID == "" {
			return nil, ErrEmptyTenant
		}
		if p.Actor.Type == "" || p.Actor.ID == "" {
			return nil, ErrEmptyActor
		}

		id := p.ProposalID
		if id == "" {
			id = gen.NewID()
		}
		now := clock.Now().UTC()

		proposal := ports.Proposal{
			ID:         id,
			TenantID:   ports.TenantID(p.TenantID),
			Status:     string(ports.ProposalStatusProposed),
			TargetSpec: p.TargetSpec,
			Operations: p.Operations,
			Revision:   1,
			Rationale:  p.Rationale,
			ProposedBy: p.Actor,
			ProposedAt: now.Format("2006-01-02T15:04:05Z"),
		}

		if err := store.Append(ctx, proposal); err != nil {
			return nil, fmt.Errorf("proposal store append: %w", err)
		}

		payloadBytes, _ := json.Marshal(proposal)
		return []appcmd.EventDraft{{
			EventType:     ports.EventProposalProposed,
			StreamID:      "proposal:" + id,
			SchemaVersion: 1,
			Payload:       json.RawMessage(payloadBytes),
		}}, nil
	}
}

// ApproveHandler returns a handler for CmdApprove that transitions a proposal
// to approved status after PolicyEvaluator ALLOW.
func ApproveHandler(store ports.ProposalStore, policy ports.PolicyEvaluator, clock ports.Clock) appcmd.Handler {
	return func(ctx context.Context, cmd appcmd.Command) ([]appcmd.EventDraft, error) {
		p, ok := cmd.Payload.(ApprovePayload)
		if !ok {
			return nil, errors.New("proposal: payload must be proposal.ApprovePayload")
		}
		if p.TenantID == "" {
			return nil, ErrEmptyTenant
		}
		if p.Actor.Type == "" || p.Actor.ID == "" {
			return nil, ErrEmptyActor
		}
		if p.ProposalID == "" {
			return nil, ErrEmptyProposalID
		}

		proposal, err := store.Get(ctx, p.ProposalID)
		if err != nil {
			if errors.Is(err, ErrProposalNotFound) {
				return nil, ErrProposalNotFound
			}
			return nil, fmt.Errorf("proposal store get: %w", err)
		}
		if proposal.TenantID != ports.TenantID(p.TenantID) {
			return nil, ErrProposalNotFound
		}
		if proposal.Status != string(ports.ProposalStatusProposed) {
			return nil, fmt.Errorf("%w: cannot approve proposal in status %s", ErrInvalidStatusTransition, proposal.Status)
		}

		// Policy gate: agent proposals require explicit ALLOW
		action := ports.Action{
			Type:       "proposal.approve",
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
			return nil, ErrPolicyDenied
		}

		proposal.Status = string(ports.ProposalStatusApproved)
		proposal.ApprovedBy = p.Actor
		proposal.ApprovedAt = clock.Now().UTC().Format("2006-01-02T15:04:05Z")
		proposal.Revision++

		if err := store.UpdateStatus(ctx, p.ProposalID, proposal.Revision-1, string(ports.ProposalStatusApproved)); err != nil {
			return nil, fmt.Errorf("proposal store update: %w", err)
		}

		payloadBytes, _ := json.Marshal(map[string]any{
			"proposal_id": proposal.ID,
			"tenant_id":   proposal.TenantID,
			"approved_by": proposal.ApprovedBy,
			"approved_at": proposal.ApprovedAt,
			"revision":    proposal.Revision,
		})
		return []appcmd.EventDraft{{
			EventType:     ports.EventProposalApproved,
			StreamID:      "proposal:" + p.ProposalID,
			SchemaVersion: 1,
			Payload:       json.RawMessage(payloadBytes),
		}}, nil
	}
}

// RejectHandler returns a handler for CmdReject that transitions a proposal
// to rejected status. This is the DENY path — no policy evaluation needed.
func RejectHandler(store ports.ProposalStore, clock ports.Clock) appcmd.Handler {
	return func(ctx context.Context, cmd appcmd.Command) ([]appcmd.EventDraft, error) {
		p, ok := cmd.Payload.(RejectPayload)
		if !ok {
			return nil, errors.New("proposal: payload must be proposal.RejectPayload")
		}
		if p.TenantID == "" {
			return nil, ErrEmptyTenant
		}
		if p.Actor.Type == "" || p.Actor.ID == "" {
			return nil, ErrEmptyActor
		}
		if p.ProposalID == "" {
			return nil, ErrEmptyProposalID
		}

		proposal, err := store.Get(ctx, p.ProposalID)
		if err != nil {
			return nil, ErrProposalNotFound
		}
		if proposal.TenantID != ports.TenantID(p.TenantID) {
			return nil, ErrProposalNotFound
		}
		if proposal.Status != string(ports.ProposalStatusProposed) {
			return nil, fmt.Errorf("%w: cannot reject proposal in status %s", ErrInvalidStatusTransition, proposal.Status)
		}

		proposal.Status = string(ports.ProposalStatusRejected)
		proposal.RejectedBy = p.Actor
		proposal.RejectedAt = clock.Now().UTC().Format("2006-01-02T15:04:05Z")
		proposal.Revision++

		if err := store.UpdateStatus(ctx, p.ProposalID, proposal.Revision-1, string(ports.ProposalStatusRejected)); err != nil {
			return nil, fmt.Errorf("proposal store update: %w", err)
		}

		payloadBytes, _ := json.Marshal(map[string]any{
			"proposal_id": proposal.ID,
			"tenant_id":   proposal.TenantID,
			"rejected_by": proposal.RejectedBy,
			"rejected_at": proposal.RejectedAt,
			"reason":      p.Reason,
			"revision":    proposal.Revision,
		})
		return []appcmd.EventDraft{{
			EventType:     ports.EventProposalRejected,
			StreamID:      "proposal:" + p.ProposalID,
			SchemaVersion: 1,
			Payload:       json.RawMessage(payloadBytes),
		}}, nil
	}
}

// ApplyHandler returns a handler for CmdApply that applies an approved proposal
// over the journal with optimistic revision checking. Only approved proposals
// can be applied. Policy evaluation gates the apply: if policy denies, the
// proposal is NOT applied (REQ-006 / ADR-021).
func ApplyHandler(store ports.ProposalStore, policy ports.PolicyEvaluator, clock ports.Clock) appcmd.Handler {
	return func(ctx context.Context, cmd appcmd.Command) ([]appcmd.EventDraft, error) {
		p, ok := cmd.Payload.(ApplyPayload)
		if !ok {
			return nil, errors.New("proposal: payload must be proposal.ApplyPayload")
		}
		if p.TenantID == "" {
			return nil, ErrEmptyTenant
		}
		if p.Actor.Type == "" || p.Actor.ID == "" {
			return nil, ErrEmptyActor
		}
		if p.ProposalID == "" {
			return nil, ErrEmptyProposalID
		}

		proposal, err := store.Get(ctx, p.ProposalID)
		if err != nil {
			return nil, ErrProposalNotFound
		}
		if proposal.TenantID != ports.TenantID(p.TenantID) {
			return nil, ErrProposalNotFound
		}
		if proposal.Status != string(ports.ProposalStatusApproved) {
			return nil, fmt.Errorf("%w: cannot apply proposal in status %s", ErrInvalidStatusTransition, proposal.Status)
		}
		if p.ExpectedVersion > 0 && proposal.Revision != p.ExpectedVersion {
			return nil, ports.ErrVersionConflict
		}

		// Policy gate: evaluate "proposal.apply" before mutating.
		// If policy denies, do NOT apply — return error without UpdateStatus.
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
			return nil, ErrPolicyDenied
		}

		proposal.Status = string(ports.ProposalStatusApplied)
		proposal.AppliedAt = clock.Now().UTC().Format("2006-01-02T15:04:05Z")
		proposal.Revision++

		if err := store.UpdateStatus(ctx, p.ProposalID, proposal.Revision-1, string(ports.ProposalStatusApplied)); err != nil {
			if errors.Is(err, ports.ErrVersionConflict) {
				return nil, ports.ErrVersionConflict
			}
			return nil, fmt.Errorf("proposal store update: %w", err)
		}

		payloadBytes, _ := json.Marshal(map[string]any{
			"proposal_id": proposal.ID,
			"tenant_id":   proposal.TenantID,
			"operations":  proposal.Operations,
			"applied_at":  proposal.AppliedAt,
			"revision":    proposal.Revision,
		})
		return []appcmd.EventDraft{{
			EventType:     ports.EventProposalApplied,
			StreamID:      "proposal:" + p.ProposalID,
			SchemaVersion: 1,
			Payload:       json.RawMessage(payloadBytes),
		}}, nil
	}
}

// ConflictHandler returns a handler for CmdConflict that emits proposal.conflicted.v1
// when a revision drift is detected during apply.
func ConflictHandler(store ports.ProposalStore) appcmd.Handler {
	return func(ctx context.Context, cmd appcmd.Command) ([]appcmd.EventDraft, error) {
		p, ok := cmd.Payload.(ConflictPayload)
		if !ok {
			return nil, errors.New("proposal: payload must be proposal.ConflictPayload")
		}
		if p.TenantID == "" {
			return nil, ErrEmptyTenant
		}
		if p.Actor.Type == "" || p.Actor.ID == "" {
			return nil, ErrEmptyActor
		}
		if p.ProposalID == "" {
			return nil, ErrEmptyProposalID
		}

		proposal, err := store.Get(ctx, p.ProposalID)
		if err != nil {
			return nil, ErrProposalNotFound
		}
		if proposal.TenantID != ports.TenantID(p.TenantID) {
			return nil, ErrProposalNotFound
		}

		proposal.Status = string(ports.ProposalStatusConflicted)
		proposal.Revision++

		if err := store.UpdateStatus(ctx, p.ProposalID, proposal.Revision-1, string(ports.ProposalStatusConflicted)); err != nil {
			// Best effort — conflict is written even if update fails
		}

		payloadBytes, _ := json.Marshal(map[string]any{
			"proposal_id":   proposal.ID,
			"tenant_id":     proposal.TenantID,
			"reason":        p.Reason,
			"conflicted_by": p.Actor,
			"revision":      proposal.Revision,
		})
		return []appcmd.EventDraft{{
			EventType:     ports.EventProposalConflicted,
			StreamID:      "proposal:" + p.ProposalID,
			SchemaVersion: 1,
			Payload:       json.RawMessage(payloadBytes),
		}}, nil
	}
}
