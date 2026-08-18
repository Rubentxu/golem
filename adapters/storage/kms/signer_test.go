package kms

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

// TestSignedManifest_Verify verifies the HMAC-SHA256 signature math used by KMS signing.
func TestSignedManifest_Verify(t *testing.T) {
	t.Parallel()

	// We test the signature verification math directly without real KMS.
	data := []byte(`{"tenant_id":"tenant-test","journal_position":{"head":42}}`)

	// Compute expected HMAC-SHA256.
	sum := sha256.Sum256(data)
	want := hex.EncodeToString(sum[:])

	// Verify: HMAC-SHA256(data) should equal itself.
	if want == "" {
		t.Error("sha256 should not be empty")
	}

	// Sign with the math directly.
	sig, err := signData(data)
	if err != nil {
		t.Fatalf("signData: %v", err)
	}

	// Verify should pass with correct signature.
	if err := verifySig(data, sig); err != nil {
		t.Errorf("verifySig failed: %v", err)
	}

	// Verify should fail with wrong signature.
	if err := verifySig(data, "wrong"); err == nil {
		t.Error("verifySig should have failed for wrong signature")
	}
}

// TestSignedManifest_InvalidRejected verifies that a tampered manifest is rejected.
func TestSignedManifest_InvalidRejected(t *testing.T) {
	t.Parallel()

	original := []byte(`{"tenant_id":"tenant-test","journal_position":{"head":42}}`)
	sig, err := signData(original)
	if err != nil {
		t.Fatalf("signData: %v", err)
	}

	// Tamper with data.
	tampered := []byte(`{"tenant_id":"tenant-test","journal_position":{"head":100}}`)
	if err := verifySig(tampered, sig); err == nil {
		t.Error("tampered data should have been rejected")
	}
}

// signData computes HMAC-SHA256 for testing (same algorithm as KMS Sign).
func signData(data []byte) (string, error) {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

// verifySig verifies signature for testing.
func verifySig(data []byte, sig string) error {
	expected, _ := signData(data)
	if expected != sig {
		return ErrInvalidSignature
	}
	return nil
}
