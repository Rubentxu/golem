package ports

import (
	"context"
	"errors"
)

// LLM provider port errors (ADR-061).
var (
	// ErrInvalidLLMRequest is returned when the LLM request is malformed.
	ErrInvalidLLMRequest = errors.New("ports: invalid LLM request")
	// ErrProviderUnavailable is returned when the LLM provider is unreachable.
	ErrProviderUnavailable = errors.New("ports: LLM provider unavailable")
	// ErrProviderCapability is returned when a capability is not supported.
	ErrProviderCapability = errors.New("ports: LLM provider capability not supported")
)

// LLMProvider is the port for LLM completion services (ADR-061).
// It does NOT support streaming — completions are returned as a whole.
type LLMProvider interface {
	// Complete returns a deterministic completion for the given request.
	// The provider MUST be deterministic when deterministic replay is required.
	Complete(ctx context.Context, req LLMRequest) (LLMResponse, error)
	// Capabilities returns the provider capabilities.
	Capabilities() LLMProviderCapabilities
}

// LLMProviderCapabilities describes what an LLMProvider can do (ADR-061).
type LLMProviderCapabilities struct {
	// NoRetention indicates the provider does not store prompts/responses.
	NoRetention bool
	// Region is the provider deployment region (e.g., "us-east-1", "local").
	Region string
	// Audit indicates whether the provider emits audit events.
	Audit bool
}

// LLMRequest is the input to LLMProvider.Complete (ADR-061).
type LLMRequest struct {
	TenantID string `json:"tenant_id"` // mandatory
	Prompt   string `json:"prompt"`    // mandatory
	// Model is the target model (provider-specific identifier).
	Model string `json:"model,omitempty"`
}

// LLMResponse is the output from LLMProvider.Complete (ADR-061).
type LLMResponse struct {
	TenantID  string `json:"tenant_id"`
	Content   string `json:"content"` // completion text
	Model     string `json:"model"`
	Provider  string `json:"provider"` // adapter name (e.g., "openai-compatible", "memstore")
	TokenUsed int    `json:"token_used,omitempty"`
}

// EmbeddingProvider is the port for embedding services (ADR-061).
type EmbeddingProvider interface {
	// Embeddings returns vector embeddings for the given input.
	Embeddings(ctx context.Context, req EmbeddingRequest) (EmbeddingResponse, error)
}

// EmbeddingRequest is the input to EmbeddingProvider.Embeddings (ADR-061).
type EmbeddingRequest struct {
	TenantID string `json:"tenant_id"` // mandatory
	Text     string `json:"text"`      // mandatory
	Model    string `json:"model,omitempty"`
}

// EmbeddingResponse is the output from EmbeddingProvider.Embeddings (ADR-061).
type EmbeddingResponse struct {
	TenantID  string    `json:"tenant_id"`
	Embedding []float64 `json:"embedding"` // vector
	Model     string    `json:"model"`
	Provider  string    `json:"provider"` // adapter name
}
