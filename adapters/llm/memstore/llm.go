// Package memstore provides a deterministic LLM adapter for testing and replay.
package memstore

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"

	"github.com/Rubentxu/golem/internal/ports"
)

// LLMStore implements ports.LLMProvider with deterministic replay.
// Responses are indexed by sha256(request) for deterministic replay.
type LLMStore struct {
	responses map[string]ports.LLMResponse
}

// New creates a new LLMStore with the given responses.
func New(responses map[string]ports.LLMResponse) *LLMStore {
	return &LLMStore{responses: responses}
}

// NewFromMap creates a new LLMStore from a map of prompt to response.
func NewFromMap(responses map[string]string) *LLMStore {
	store := &LLMStore{responses: make(map[string]ports.LLMResponse)}
	for prompt, content := range responses {
		hash := hashRequest(prompt)
		store.responses[hash] = ports.LLMResponse{
			Content:  content,
			Provider: "memstore",
		}
	}
	return store
}

// Complete returns a deterministic response for the given request.
// The response is indexed by sha256(prompt) for deterministic replay.
func (s *LLMStore) Complete(ctx context.Context, req ports.LLMRequest) (ports.LLMResponse, error) {
	if req.Prompt == "" {
		return ports.LLMResponse{}, ports.ErrInvalidLLMRequest
	}
	hash := hashRequest(req.Prompt)
	resp, ok := s.responses[hash]
	if !ok {
		return ports.LLMResponse{}, ports.ErrProviderUnavailable
	}
	resp.TenantID = req.TenantID
	return resp, nil
}

// Capabilities returns the memstore capabilities.
func (s *LLMStore) Capabilities() ports.LLMProviderCapabilities {
	return ports.LLMProviderCapabilities{
		NoRetention: true,
		Region:      "local",
		Audit:       true,
	}
}

// hashRequest returns sha256 of the prompt for deterministic indexing.
func hashRequest(prompt string) string {
	h := sha256.Sum256([]byte(prompt))
	return fmt.Sprintf("%x", h)
}

// AddResponse adds a response for a given prompt.
func (s *LLMStore) AddResponse(prompt string, response ports.LLMResponse) {
	hash := hashRequest(prompt)
	s.responses[hash] = response
}

// ResponseFor returns the response for a given prompt, or an error if not found.
func (s *LLMStore) ResponseFor(prompt string) (ports.LLMResponse, error) {
	hash := hashRequest(prompt)
	resp, ok := s.responses[hash]
	if !ok {
		return ports.LLMResponse{}, fmt.Errorf("memstore: no response for prompt")
	}
	return resp, nil
}

// MarshalJSON implements json.Marshaler for deterministic serialization.
func (s *LLMStore) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.responses)
}
