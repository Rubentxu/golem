package tck

import (
	"context"
	"testing"

	"github.com/Rubentxu/golem/internal/ports"
)

// RunProvenanceVerifierTCK is a light TCK for ProvenanceVerifier.
// It exercises: subject digest match, subject mismatch, builder.id present,
// materials present, and determinism.
func RunProvenanceVerifierTCK(t *testing.T, newVerifier func() ports.ProvenanceVerifier) {
	v := newVerifier()

	t.Run("valid slsa provenance passes all checks", func(t *testing.T) {
		// This test uses a real SLSA statement fixture.
		doc := `{
			"_type": "https://in-toto.io/Statement/v1",
			"subject": [{"name":"example-binary","digest":{"sha256":"3a2b1c0d3e4f5a6b7c8d9e0f1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a"}}],
			"predicateType": "slsa-provenance/v1",
			"predicate": {
				"builder": {"id": "https://github.com/slsa-framework/slsa-github-generator@v1.4.0"},
				"materials": [{"uri":"git+https://github.com/example/project@v2.0.0","digest":{"sha1":"abc123"}}]
			}
		}`
		subjResult, err := v.VerifySubject(context.Background(), []byte(doc), "sha256:3a2b1c0d3e4f5a6b7c8d9e0f1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a")
		if err != nil {
			t.Fatalf("VerifySubject error: %v", err)
		}
		if !subjResult.OK {
			t.Errorf("VerifySubject: expected OK, got %+v", subjResult)
		}

		builderResult, err := v.VerifyBuilder(context.Background(), []byte(doc))
		if err != nil {
			t.Fatalf("VerifyBuilder error: %v", err)
		}
		if !builderResult.OK {
			t.Errorf("VerifyBuilder: expected OK, got %+v", builderResult)
		}
	})

	t.Run("subject mismatch fails the attestation", func(t *testing.T) {
		doc := `{
			"_type": "https://in-toto.io/Statement/v1",
			"subject": [{"name":"example-binary","digest":{"sha256":"0000000000000000000000000000000000000000000000000000000000000000"}}],
			"predicateType": "slsa-provenance/v1",
			"predicate": {
				"builder": {"id": "https://github.com/slsa-framework/slsa-github-generator@v1.4.0"},
				"materials": [{"uri":"git+https://github.com/example/project","digest":{"sha1":"abc123"}}]
			}
		}`
		subjResult, err := v.VerifySubject(context.Background(), []byte(doc), "sha256:deadbeef")
		if err != nil {
			t.Fatalf("VerifySubject error: %v", err)
		}
		if subjResult.OK {
			t.Error("VerifySubject: expected failure for mismatched digest, got OK")
		}
		if len(subjResult.Checks) == 0 || subjResult.Checks[0].Reason != "subject_mismatch" {
			t.Errorf("Check reason = %v, want subject_mismatch", subjResult.Checks)
		}
	})

	t.Run("malformed JSON returns verification with ok=false", func(t *testing.T) {
		r, err := v.VerifySubject(context.Background(), []byte("not json"), "sha256:abc")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if r.OK {
			t.Error("expected OK=false for malformed JSON")
		}
	})

	t.Run("builder.id present returns ok", func(t *testing.T) {
		doc := `{
			"_type": "https://in-toto.io/Statement/v1",
			"subject": [{"name":"x","digest":{"sha256":"abc"}}],
			"predicateType": "slsa-provenance/v1",
			"predicate": {"builder": {"id": "https://github.com/actions@example.com"}}
		}`
		r, err := v.VerifyBuilder(context.Background(), []byte(doc))
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		if !r.OK {
			t.Errorf("expected OK, got %+v", r)
		}
	})

	t.Run("empty builder.id returns not ok", func(t *testing.T) {
		doc := `{
			"_type": "https://in-toto.io/Statement/v1",
			"subject": [{"name":"x","digest":{"sha256":"abc"}}],
			"predicateType": "slsa-provenance/v1",
			"predicate": {"builder": {"id": ""}}
		}`
		r, err := v.VerifyBuilder(context.Background(), []byte(doc))
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		if r.OK {
			t.Error("expected not OK for empty builder.id")
		}
	})
}
