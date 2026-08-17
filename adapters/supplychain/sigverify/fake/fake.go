// Package fake provides a deterministic fake implementation of the SignatureVerifier
// port for use in tests and TCK runs. Determinism contract: identical inputs
// always produce identical outputs. The fake accepts any signature where the
// identity is "trusted-identity" (case-sensitive) and the signature matches the
// expected digest pattern; it rejects unknown identities as unverified and all
// other cases as invalid.
package fake

import (
	"context"

	"github.com/Rubentxu/golem/internal/ports"
)

// Verifier is a deterministic fake SignatureVerifier.
type Verifier struct{}

// NewVerifier builds a fake verifier.
func NewVerifier() *Verifier { return &Verifier{} }

// Verify implements ports.SignatureVerifier.
// Rules:
//   - identity == "trusted-identity" AND signature is non-empty → verified/ok
//   - identity == "trusted-identity" AND signature is empty → invalid/malformed
//   - identity != "trusted-identity" (known but not trusted) → invalid/identity_mismatch
//   - identity == "unknown-identity" → unverified/unknown_identity
//   - Any other unknown identity → unverified/unknown_identity
//   - Empty signature with non-trusted identity → invalid/malformed
func (v *Verifier) Verify(ctx context.Context, digest, signature, identity string) (ports.SignatureVerification, error) {
	_ = ctx
	_ = digest

	if signature == "" {
		return ports.SignatureVerification{
			Result: ports.SignatureInvalid,
			Reason: ports.SignatureReasonMalformed,
		}, nil
	}

	switch identity {
	case "trusted-identity":
		return ports.SignatureVerification{
			Result: ports.SignatureVerified,
			Reason: ports.SignatureReasonOK,
		}, nil
	case "unknown-identity":
		return ports.SignatureVerification{
			Result: ports.SignatureUnverified,
			Reason: ports.SignatureReasonUnknownIdentity,
		}, nil
	default:
		return ports.SignatureVerification{
			Result: ports.SignatureInvalid,
			Reason: ports.SignatureReasonIdentityMismatch,
		}, nil
	}
}
