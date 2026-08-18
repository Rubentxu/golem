package ports

import "context"

// Proposal status constants.
const (
	ProposalStatusDraft      = "draft"
	ProposalStatusProposed   = "proposed"
	ProposalStatusApproved   = "approved"
	ProposalStatusRejected   = "rejected"
	ProposalStatusApplied    = "applied"
	ProposalStatusConflicted = "conflicted"
)

// Proposal represents a change proposal (ADR-065).
type Proposal struct {
	ID         string
	TenantID   string
	Status     string
	TargetSpec TargetSpec
	Operations []Operation
	Revision   uint64
	Rationale  string
	ProposedBy Actor
	ProposedAt string
	ApprovedBy Actor
	ApprovedAt string
	RejectedBy Actor
	RejectedAt string
	AppliedAt  string
}

// TargetSpec identifies the target of a proposal.
type TargetSpec struct {
	Type string // "node", "edge", "graph"
	ID   string
}

// Operation represents a graph mutation operation.
type Operation struct {
	Type string // "CreateNode", "UpdateNode", "DeleteNode", "AddEdge", "RemoveEdge"
	Kind string
	ID   string
	From string
	To   string
	// Payload is operation-specific data
	Payload map[string]any
}

// ProposalStore is the port for proposal persistence (ADR-065).
type ProposalStore interface {
	// Append adds a new proposal and returns the created proposal.
	Append(ctx context.Context, p Proposal) error
	// Get returns a proposal by ID.
	Get(ctx context.Context, id string) (Proposal, error)
	// List returns all proposals for a tenant.
	List(ctx context.Context, tenantID string) ([]Proposal, error)
	// Replay replays all proposals for a tenant.
	Replay(ctx context.Context, tenantID string) ([]Proposal, error)
	// UpdateStatus updates the status of a proposal with optimistic locking.
	UpdateStatus(ctx context.Context, id string, expectedRevision uint64, newStatus string) error
}

// ProposalOp is a discriminated union of proposal operations (ADR-065).
type ProposalOp struct {
	Op     string // "propose", "approve", "reject", "conflict", "apply"
	Node   *Operation
	Edge   *Operation
	Graph  *Operation
	Reason string
}

// ProposalQuery represents a query for proposals (ADR-065).
type ProposalQuery struct {
	TenantID string
	Status   string
	Limit    int
}
