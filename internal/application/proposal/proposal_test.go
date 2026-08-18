package proposal

import (
	"context"
	"testing"

	appcmd "github.com/Rubentxu/golem/internal/application/command"
	"github.com/Rubentxu/golem/internal/clock"
	"github.com/Rubentxu/golem/internal/ids"
	"github.com/Rubentxu/golem/internal/ports"
)

// mockProposalStore implements ports.ProposalStore for testing.
type mockProposalStore struct {
	proposals map[string]ports.Proposal
}

func newMockStore() *mockProposalStore {
	return &mockProposalStore{proposals: make(map[string]ports.Proposal)}
}

func (s *mockProposalStore) Append(ctx context.Context, p ports.Proposal) error {
	s.proposals[p.ID] = p
	return nil
}
func (s *mockProposalStore) Get(ctx context.Context, id string) (ports.Proposal, error) {
	if p, ok := s.proposals[id]; ok {
		return p, nil
	}
	return ports.Proposal{}, ErrProposalNotFound
}
func (s *mockProposalStore) List(ctx context.Context, tenantID string) ([]ports.Proposal, error) {
	var result []ports.Proposal
	for _, p := range s.proposals {
		if p.TenantID == ports.TenantID(tenantID) {
			result = append(result, p)
		}
	}
	return result, nil
}
func (s *mockProposalStore) Replay(ctx context.Context, tenantID string) ([]ports.Proposal, error) {
	return s.List(ctx, tenantID)
}
func (s *mockProposalStore) UpdateStatus(ctx context.Context, id string, expectedRevision uint64, newStatus string) error {
	if p, ok := s.proposals[id]; ok {
		if p.Revision != expectedRevision {
			return ports.ErrVersionConflict
		}
		p.Status = newStatus
		s.proposals[id] = p
		return nil
	}
	return ErrProposalNotFound
}

// mockPolicy implements ports.PolicyEvaluator for testing.
type mockPolicy struct {
	outcome ports.DecisionOutcome
}

func (p *mockPolicy) Evaluate(ctx context.Context, action ports.Action) (ports.Decision, error) {
	return ports.Decision{Outcome: p.outcome}, nil
}

func TestProposeHandler_CreatesProposal(t *testing.T) {
	store := newMockStore()
	gen := ids.NewGenerator(clock.SystemClock{})
	clk := clock.SystemClock{}
	h := ProposeHandler(store, gen, clk)

	cmd := appcmd.Command{
		Name:     CmdPropose,
		TenantID: "t-test",
		Actor:    ports.Actor{Type: "agent", ID: "agent-1"},
		Payload: ProposePayload{
			ProposalID: "p-001",
			TenantID:   "t-test",
			TargetSpec: ports.TargetSpec{Type: "node", ID: "n-001"},
			Operations: []ports.Operation{{Type: "CreateNode", Kind: "TestCase", ID: "tc-001"}},
			Rationale:  "add test case",
			Actor:      ports.Actor{Type: "agent", ID: "agent-1"},
		},
	}

	events, err := h(context.Background(), cmd)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].EventType != ports.EventProposalProposed {
		t.Errorf("expected event type %s, got %s", ports.EventProposalProposed, events[0].EventType)
	}
	if store.proposals["p-001"].Status != string(ports.ProposalStatusProposed) {
		t.Errorf("expected status proposed, got %s", store.proposals["p-001"].Status)
	}
}

func TestApproveHandler_TransitionsToApproved(t *testing.T) {
	store := newMockStore()
	store.proposals["p-001"] = ports.Proposal{
		ID:       "p-001",
		TenantID: "t-test",
		Status:   string(ports.ProposalStatusProposed),
		Revision: 1,
	}
	clk := clock.SystemClock{}
	pol := &mockPolicy{outcome: ports.DecisionOutcomeAllow}
	h := ApproveHandler(store, pol, clk)

	cmd := appcmd.Command{
		Name:     CmdApprove,
		TenantID: "t-test",
		Actor:    ports.Actor{Type: "user", ID: "admin-1"},
		Payload: ApprovePayload{
			ProposalID: "p-001",
			TenantID:   "t-test",
			Actor:      ports.Actor{Type: "user", ID: "admin-1"},
		},
	}

	events, err := h(context.Background(), cmd)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].EventType != ports.EventProposalApproved {
		t.Errorf("expected %s, got %s", ports.EventProposalApproved, events[0].EventType)
	}
	if store.proposals["p-001"].Status != string(ports.ProposalStatusApproved) {
		t.Errorf("expected approved, got %s", store.proposals["p-001"].Status)
	}
}

func TestApproveHandler_PolicyDenied(t *testing.T) {
	store := newMockStore()
	store.proposals["p-001"] = ports.Proposal{
		ID:       "p-001",
		TenantID: "t-test",
		Status:   string(ports.ProposalStatusProposed),
		Revision: 1,
	}
	clk := clock.SystemClock{}
	pol := &mockPolicy{outcome: ports.DecisionOutcomeDeny}
	h := ApproveHandler(store, pol, clk)

	cmd := appcmd.Command{
		Name:     CmdApprove,
		TenantID: "t-test",
		Actor:    ports.Actor{Type: "agent", ID: "agent-1"},
		Payload: ApprovePayload{
			ProposalID: "p-001",
			TenantID:   "t-test",
			Actor:      ports.Actor{Type: "agent", ID: "agent-1"},
		},
	}

	_, err := h(context.Background(), cmd)
	if err != ErrPolicyDenied {
		t.Errorf("expected ErrPolicyDenied, got %v", err)
	}
}

func TestRejectHandler_TransitionsToRejected(t *testing.T) {
	store := newMockStore()
	store.proposals["p-001"] = ports.Proposal{
		ID:       "p-001",
		TenantID: "t-test",
		Status:   string(ports.ProposalStatusProposed),
		Revision: 1,
	}
	clk := clock.SystemClock{}
	h := RejectHandler(store, clk)

	cmd := appcmd.Command{
		Name:     CmdReject,
		TenantID: "t-test",
		Actor:    ports.Actor{Type: "user", ID: "admin-1"},
		Payload: RejectPayload{
			ProposalID: "p-001",
			TenantID:   "t-test",
			Reason:     "not aligned with goals",
			Actor:      ports.Actor{Type: "user", ID: "admin-1"},
		},
	}

	events, err := h(context.Background(), cmd)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].EventType != ports.EventProposalRejected {
		t.Errorf("expected %s, got %s", ports.EventProposalRejected, events[0].EventType)
	}
	if store.proposals["p-001"].Status != string(ports.ProposalStatusRejected) {
		t.Errorf("expected rejected, got %s", store.proposals["p-001"].Status)
	}
}

func TestApplyHandler_AppliesApprovedProposal(t *testing.T) {
	store := newMockStore()
	store.proposals["p-001"] = ports.Proposal{
		ID:       "p-001",
		TenantID: "t-test",
		Status:   string(ports.ProposalStatusApproved),
		Revision: 2,
	}
	clk := clock.SystemClock{}
	policy := &mockPolicy{outcome: ports.DecisionOutcomeAllow}
	h := ApplyHandler(store, policy, clk)

	cmd := appcmd.Command{
		Name:     CmdApply,
		TenantID: "t-test",
		Actor:    ports.Actor{Type: "user", ID: "admin-1"},
		Payload: ApplyPayload{
			ProposalID:      "p-001",
			TenantID:        "t-test",
			ExpectedVersion: 2,
			Actor:           ports.Actor{Type: "user", ID: "admin-1"},
		},
	}

	events, err := h(context.Background(), cmd)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].EventType != ports.EventProposalApplied {
		t.Errorf("expected %s, got %s", ports.EventProposalApplied, events[0].EventType)
	}
	if store.proposals["p-001"].Status != string(ports.ProposalStatusApplied) {
		t.Errorf("expected applied, got %s", store.proposals["p-001"].Status)
	}
}

func TestApplyHandler_VersionConflict(t *testing.T) {
	store := newMockStore()
	store.proposals["p-001"] = ports.Proposal{
		ID:       "p-001",
		TenantID: "t-test",
		Status:   string(ports.ProposalStatusApproved),
		Revision: 3, // expected 2 but actual is 3
	}
	clk := clock.SystemClock{}
	policy := &mockPolicy{outcome: ports.DecisionOutcomeAllow}
	h := ApplyHandler(store, policy, clk)

	cmd := appcmd.Command{
		Name:     CmdApply,
		TenantID: "t-test",
		Actor:    ports.Actor{Type: "user", ID: "admin-1"},
		Payload: ApplyPayload{
			ProposalID:      "p-001",
			TenantID:        "t-test",
			ExpectedVersion: 2,
			Actor:           ports.Actor{Type: "user", ID: "admin-1"},
		},
	}

	_, err := h(context.Background(), cmd)
	if err != ports.ErrVersionConflict {
		t.Errorf("expected ErrVersionConflict, got %v", err)
	}
}

func TestConflictHandler_EmitsConflictedEvent(t *testing.T) {
	store := newMockStore()
	store.proposals["p-001"] = ports.Proposal{
		ID:       "p-001",
		TenantID: "t-test",
		Status:   string(ports.ProposalStatusApproved),
		Revision: 2,
	}
	h := ConflictHandler(store)

	cmd := appcmd.Command{
		Name:     CmdConflict,
		TenantID: "t-test",
		Actor:    ports.Actor{Type: "agent", ID: "agent-1"},
		Payload: ConflictPayload{
			ProposalID: "p-001",
			TenantID:   "t-test",
			Reason:     "revision drift detected",
			Actor:      ports.Actor{Type: "agent", ID: "agent-1"},
		},
	}

	events, err := h(context.Background(), cmd)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].EventType != ports.EventProposalConflicted {
		t.Errorf("expected %s, got %s", ports.EventProposalConflicted, events[0].EventType)
	}
}
