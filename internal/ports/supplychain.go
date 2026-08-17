package ports

import "context"

// SBOMParsed represents a parsed SBOM document with normalized component data.
type SBOMParsed struct {
	DocID          string
	DocName        string
	Format         string // "spdx" or "cyclonedx"
	SpecVersion    string // e.g. "SPDX-2.3", "1.5"
	ArtifactDigest string // sha256:... of the described artifact (extracted per format)
	Components     []ParsedComponent
	Warnings       []string
}

// ParsedComponent is a normalized SBOM component extracted from a parsed document.
type ParsedComponent struct {
	Purl      string // normalized purl; empty if synthetic
	Name      string
	Version   string
	Synthetic bool // true when purl was derived from name+version+ecosystem
}

// SBOMParser parses an SBOM document in a supported format and returns a
// normalized SBOMParsed structure.
type SBOMParser interface {
	// Parse interprets raw bytes as the named format and returns a structured
	// representation. The format string is one of: "spdx-2.3", "spdx-3.0",
	// "cyclonedx-1.5", "cyclonedx-1.6".
	Parse(ctx context.Context, raw []byte) (SBOMParsed, error)
}

// SignatureResult enumerates the possible outcomes of signature verification.
type SignatureResult string

const (
	SignatureVerified   SignatureResult = "verified"
	SignatureUnverified SignatureResult = "unverified"
	SignatureInvalid    SignatureResult = "invalid"
)

// SignatureReason explains why a signature check returned a given result.
type SignatureReason string

const (
	SignatureReasonOK               SignatureReason = "ok"
	SignatureReasonMismatch         SignatureReason = "signature_mismatch"
	SignatureReasonIdentityMismatch SignatureReason = "identity_mismatch"
	SignatureReasonMalformed        SignatureReason = "malformed"
	SignatureReasonUnknownIdentity  SignatureReason = "unknown_identity"
)

// SignatureVerification is the result of a signature check.
type SignatureVerification struct {
	Result SignatureResult
	Reason SignatureReason
}

// SignatureVerifier checks a cryptographic signature against an expected digest
// and an identity claim. Implementations must be deterministic: identical inputs
// always produce identical results.
type SignatureVerifier interface {
	// Verify checks whether the signature was produced by identity over the given
	// digest. Returns SignatureVerification with a result and reason.
	Verify(ctx context.Context, digest, signature, identity string) (SignatureVerification, error)
}

// ProvenanceCheck identifies which aspect of a provenance statement was checked.
type ProvenanceCheck string

const (
	CheckSubjectDigest   ProvenanceCheck = "subject_digest"
	CheckBuilderIdentity ProvenanceCheck = "builder_identity"
	CheckMaterials       ProvenanceCheck = "materials_present"
)

// CheckResult records the outcome of a single provenance check.
type CheckResult struct {
	Check  ProvenanceCheck
	OK     bool
	Reason string
}

// ProvenanceVerification is the result of provenance validation.
type ProvenanceVerification struct {
	OK     bool
	Checks []CheckResult
}

// ProvenanceVerifier validates an in-toto/SLSA provenance statement against
// an expected subject digest. Implementations must be deterministic.
type ProvenanceVerifier interface {
	// VerifySubject checks that the statement's subject digest matches the expected
	// artifact digest.
	VerifySubject(ctx context.Context, statement []byte, digest string) (ProvenanceVerification, error)
	// VerifyBuilder checks that the statement contains a non-empty builder identity.
	VerifyBuilder(ctx context.Context, statement []byte) (ProvenanceVerification, error)
}
