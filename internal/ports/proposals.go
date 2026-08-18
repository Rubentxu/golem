package ports

import "context"

// ProposalStatus is the status of a proposal (ADR-065).
type ProposalStatus string

// Proposal status constants.
const (
	ProposalStatusDraft      ProposalStatus = "draft"
	ProposalStatusProposed   ProposalStatus = "proposed"
	ProposalStatusApproved   ProposalStatus = "approved"
	ProposalStatusRejected   ProposalStatus = "rejected"
	ProposalStatusApplied    ProposalStatus = "applied"
	ProposalStatusConflicted ProposalStatus = "conflicted"
)

// Proposal represents a change proposal (ADR-065, C4).
// Fields added per spec §7: Frame, EvidenceRefs, Risk, ObservedRevision.
type Proposal struct {
	ID               string      `json:"id"`
	TenantID         TenantID    `json:"tenant_id"`
	Frame            Frame       `json:"frame"`             // execution frame (C4)
	TargetSpec       TargetSpec  `json:"target_spec"`       // legacy field name preserved for app compat
	ObservedRevision Revision    `json:"observed_revision"` // revision at proposal time (C4)
	Operations       []Operation `json:"operations"`
	Rationale        string      `json:"rationale"`
	EvidenceRefs     []string    `json:"evidence_refs"` // evidence attached (C4)
	Risk             string      `json:"risk"`          // "low" | "medium" | "high" (C4)
	Status           string      `json:"status"`        // use string for UpdateStatus compat
	Revision         uint64      `json:"revision"`      // optimistic revision
	// Legacy fields preserved for application-layer compatibility
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
