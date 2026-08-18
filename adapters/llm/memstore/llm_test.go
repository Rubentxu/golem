package memstore

import (
	"context"
	"testing"

	"github.com/Rubentxu/golem/internal/ports"
)

// TestMemStore_DeterministicReplay verifies that identical requests produce
// byte-identical responses (deterministic replay).
func TestMemStore_DeterministicReplay(t *testing.T) {
	responses := map[string]string{
		"What is 2+2?": "4",
	}
	store := NewFromMap(responses)

	req := ports.LLMRequest{
		TenantID: "t-test",
		Prompt:   "What is 2+2?",
	}

	// First call
	resp1, err := store.Complete(context.Background(), req)
	if err != nil {
		t.Fatalf("first call failed: %v", err)
	}
	if resp1.Content != "4" {
		t.Errorf("expected content '4', got %q", resp1.Content)
	}
	if resp1.Provider != "memstore" {
		t.Errorf("expected provider 'memstore', got %q", resp1.Provider)
	}

	// Second call with same request should produce byte-identical response
	resp2, err := store.Complete(context.Background(), req)
	if err != nil {
		t.Fatalf("second call failed: %v", err)
	}

	if resp1.TenantID != resp2.TenantID {
		t.Errorf("TenantID mismatch: first=%s, second=%s", resp1.TenantID, resp2.TenantID)
	}
	if resp1.Content != resp2.Content {
		t.Errorf("Content mismatch: first=%s, second=%s", resp1.Content, resp2.Content)
	}
	if resp1.Provider != resp2.Provider {
		t.Errorf("Provider mismatch: first=%s, second=%s", resp1.Provider, resp2.Provider)
	}
	if resp1.Model != resp2.Model {
		t.Errorf("Model mismatch: first=%s, second=%s", resp1.Model, resp2.Model)
	}
}

// TestMemStore_Capabilities verifies the memstore capabilities.
func TestMemStore_Capabilities(t *testing.T) {
	store := New(nil)
	cap := store.Capabilities()

	if !cap.NoRetention {
		t.Error("expected NoRetention to be true")
	}
	if cap.Region != "local" {
		t.Errorf("expected Region 'local', got %q", cap.Region)
	}
	if !cap.Audit {
		t.Error("expected Audit to be true")
	}
}

// TestMemStore_InvalidRequest verifies that empty prompt returns error.
func TestMemStore_InvalidRequest(t *testing.T) {
	store := New(nil)
	req := ports.LLMRequest{
		TenantID: "t-test",
		Prompt:   "",
	}
	_, err := store.Complete(context.Background(), req)
	if err != ports.ErrInvalidLLMRequest {
		t.Errorf("expected ErrInvalidLLMRequest, got %v", err)
	}
}

// TestMemStore_MissingResponse verifies that unknown prompt returns error.
func TestMemStore_MissingResponse(t *testing.T) {
	store := New(nil)
	req := ports.LLMRequest{
		TenantID: "t-test",
		Prompt:   "What is 3+3?",
	}
	_, err := store.Complete(context.Background(), req)
	if err != ports.ErrProviderUnavailable {
		t.Errorf("expected ErrProviderUnavailable, got %v", err)
	}
}
