package tck

import (
	"context"
	"testing"

	"github.com/Rubentxu/golem/internal/ports"
)

// RunSignatureVerifierTCK is a light TCK for SignatureVerifier.
// It asserts: matching identity+signature → verified/ok; wrong signature → invalid/malformed;
// wrong identity → invalid/identity_mismatch; unknown identity → unverified/unknown_identity.
func RunSignatureVerifierTCK(t *testing.T, newVerifier func() ports.SignatureVerifier) {
	v := newVerifier()

	t.Run("matching identity and signature is verified", func(t *testing.T) {
		r, err := v.Verify(context.Background(), "sha256:abc123", "sig-valid", "trusted-identity")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if r.Result != ports.SignatureVerified {
			t.Errorf("Result = %v, want verified", r.Result)
		}
		if r.Reason != ports.SignatureReasonOK {
			t.Errorf("Reason = %v, want ok", r.Reason)
		}
	})

	t.Run("wrong identity is invalid identity_mismatch", func(t *testing.T) {
		r, err := v.Verify(context.Background(), "sha256:abc123", "sig-valid", "some-other-identity")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if r.Result != ports.SignatureInvalid {
			t.Errorf("Result = %v, want invalid", r.Result)
		}
		if r.Reason != ports.SignatureReasonIdentityMismatch {
			t.Errorf("Reason = %v, want identity_mismatch", r.Reason)
		}
	})

	t.Run("unknown identity is unverified unknown_identity", func(t *testing.T) {
		r, err := v.Verify(context.Background(), "sha256:abc123", "sig-valid", "unknown-identity")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if r.Result != ports.SignatureUnverified {
			t.Errorf("Result = %v, want unverified", r.Result)
		}
		if r.Reason != ports.SignatureReasonUnknownIdentity {
			t.Errorf("Reason = %v, want unknown_identity", r.Reason)
		}
	})

	t.Run("malformed input (empty signature) is invalid malformed", func(t *testing.T) {
		r, err := v.Verify(context.Background(), "sha256:abc123", "", "trusted-identity")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if r.Result != ports.SignatureInvalid {
			t.Errorf("Result = %v, want invalid", r.Result)
		}
		if r.Reason != ports.SignatureReasonMalformed {
			t.Errorf("Reason = %v, want malformed", r.Reason)
		}
	})

	t.Run("determinism: identical inputs produce identical results", func(t *testing.T) {
		r1, _ := v.Verify(context.Background(), "sha256:xyz", "sig-a", "trusted-identity")
		r2, _ := v.Verify(context.Background(), "sha256:xyz", "sig-a", "trusted-identity")
		if r1 != r2 {
			t.Errorf("non-deterministic: got %+v then %+v", r1, r2)
		}
	})
}
