package ports

// Permission is one element of the closed permission catalog v1
// (ADR-058, ADR-062). Unknown permissions reject Activation; tools may
// only invoke with permissions they declare.
type Permission string

// Closed permission catalog v1 (ADR-058).
const (
	PermissionGraphRead     Permission = "graph.read"
	PermissionGraphReadLens Permission = "graph.read:lens"
	PermissionProposalWrite Permission = "proposal.write"
	PermissionProposalApply Permission = "proposal.apply"
	PermissionEvidenceWrite Permission = "evidence.write"
)
