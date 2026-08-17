// Package ref provides a stdlib-only reference implementation of the ProvenanceVerifier
// port for SLSA/in-toto provenance statements. It validates the statement structure
// using only encoding/json from the standard library.
package ref

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Rubentxu/golem/internal/ports"
)

// Verifier implements ports.ProvenanceVerifier for SLSA provenance statements.
type Verifier struct{}

// NewVerifier builds a reference provenance verifier.
func NewVerifier() *Verifier { return &Verifier{} }

// intotoStatement is the JSON structure of an in-toto statement.
type intotoStatement struct {
	Type          string          `json:"_type"`
	Subject       []subjectEntry  `json:"subject"`
	PredicateType string          `json:"predicateType"`
	Predicate     json.RawMessage `json:"predicate"`
}

type subjectEntry struct {
	Name   string         `json:"name"`
	Digest map[string]any `json:"digest"`
}

type slsaPredicate struct {
	Builder   builderEntry    `json:"builder"`
	Materials []materialEntry `json:"materials"`
}

type builderEntry struct {
	ID string `json:"id"`
}

type materialEntry struct {
	URI    string         `json:"uri"`
	Digest map[string]any `json:"digest"`
}

// VerifySubject checks that at least one subject in the statement matches the expected digest.
func (v *Verifier) VerifySubject(ctx context.Context, statement []byte, digest string) (ports.ProvenanceVerification, error) {
	_ = ctx
	var st intotoStatement
	if err := json.Unmarshal(statement, &st); err != nil {
		return ports.ProvenanceVerification{
			OK: false,
			Checks: []ports.CheckResult{{
				Check:  ports.CheckSubjectDigest,
				OK:     false,
				Reason: "malformed JSON: " + err.Error(),
			}},
		}, nil
	}

	// Extract expected hash value (strip "sha256:" prefix if present).
	expectedHash := strings.TrimPrefix(digest, "sha256:")
	found := false
	for _, sub := range st.Subject {
		if sh, ok := sub.Digest["sha256"].(string); ok && sh == expectedHash {
			found = true
			break
		}
	}

	if found {
		return ports.ProvenanceVerification{
			OK: true,
			Checks: []ports.CheckResult{{
				Check:  ports.CheckSubjectDigest,
				OK:     true,
				Reason: "subject matches expected digest",
			}},
		}, nil
	}
	return ports.ProvenanceVerification{
		OK: false,
		Checks: []ports.CheckResult{{
			Check:  ports.CheckSubjectDigest,
			OK:     false,
			Reason: "subject_mismatch",
		}},
	}, nil
}

// VerifyBuilder checks that the predicate contains a non-empty builder.id.
func (v *Verifier) VerifyBuilder(ctx context.Context, statement []byte) (ports.ProvenanceVerification, error) {
	_ = ctx
	var st intotoStatement
	if err := json.Unmarshal(statement, &st); err != nil {
		return ports.ProvenanceVerification{
			OK: false,
			Checks: []ports.CheckResult{{
				Check:  ports.CheckBuilderIdentity,
				OK:     false,
				Reason: "malformed JSON: " + err.Error(),
			}},
		}, nil
	}

	var pred struct {
		Builder builderEntry `json:"builder"`
	}
	if err := json.Unmarshal(st.Predicate, &pred); err != nil {
		return ports.ProvenanceVerification{
			OK: false,
			Checks: []ports.CheckResult{{
				Check:  ports.CheckBuilderIdentity,
				OK:     false,
				Reason: "malformed predicate: " + err.Error(),
			}},
		}, nil
	}

	if pred.Builder.ID == "" {
		return ports.ProvenanceVerification{
			OK: false,
			Checks: []ports.CheckResult{{
				Check:  ports.CheckBuilderIdentity,
				OK:     false,
				Reason: "builder.id is empty",
			}},
		}, nil
	}
	return ports.ProvenanceVerification{
		OK: true,
		Checks: []ports.CheckResult{{
			Check:  ports.CheckBuilderIdentity,
			OK:     true,
			Reason: "builder.id present",
		}},
	}, nil
}

// VerifyMaterials checks that the predicate contains at least one material entry.
// Returns ok=true if materials are present, ok=false with reason if not.
func (v *Verifier) VerifyMaterials(ctx context.Context, statement []byte) (ports.ProvenanceVerification, error) {
	_ = ctx
	var st intotoStatement
	if err := json.Unmarshal(statement, &st); err != nil {
		return ports.ProvenanceVerification{
			OK: false,
			Checks: []ports.CheckResult{{
				Check:  ports.CheckMaterials,
				OK:     false,
				Reason: "malformed JSON",
			}},
		}, nil
	}

	var pred struct {
		Materials []materialEntry `json:"materials"`
	}
	if err := json.Unmarshal(st.Predicate, &pred); err != nil {
		return ports.ProvenanceVerification{
			OK: false,
			Checks: []ports.CheckResult{{
				Check:  ports.CheckMaterials,
				OK:     false,
				Reason: "malformed predicate",
			}},
		}, nil
	}

	if len(pred.Materials) == 0 {
		return ports.ProvenanceVerification{
			OK: false,
			Checks: []ports.CheckResult{{
				Check:  ports.CheckMaterials,
				OK:     false,
				Reason: "materials list is empty",
			}},
		}, nil
	}
	return ports.ProvenanceVerification{
		OK: true,
		Checks: []ports.CheckResult{{
			Check:  ports.CheckMaterials,
			OK:     true,
			Reason: fmt.Sprintf("%d material(s) present", len(pred.Materials)),
		}},
	}, nil
}
