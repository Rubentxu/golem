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

// LLMMessage is one turn in a multi-turn LLM conversation (ADR-061, C4).
type LLMMessage struct {
	Role    string `json:"role"`    // "system" | "user" | "assistant"
	Content string `json:"content"` // already redacted by caller (Redactor.Redact)
}

// LLMRequest is the input to LLMProvider.Complete (ADR-061).
type LLMRequest struct {
	TenantID string       `json:"tenant_id"` // mandatory
	Messages []LLMMessage `json:"messages"`  // ordered, oldest first; replaces Prompt field
	Model    string       `json:"model,omitempty"`
}

// LLMResponse is the output from LLMProvider.Complete (ADR-061).
type LLMResponse struct {
	TenantID string   `json:"tenant_id"`
	Content  string   `json:"content"` // completion text
	Model    string   `json:"model"`
	Provider string   `json:"provider"` // adapter name (e.g., "openai-compatible", "memstore")
	Usage    LLMUsage `json:"usage"`    // token usage from provider
}

// LLMUsage holds token counts and cost for an LLM response (ADR-061, AC-6).
// Fields are provider-reported; sanity-bounded by the adapter.
// NOTE: W4 spec type divergence — LLMUsage.CostUSD tracks observed cost per
// LLM call, while the Meter subsystem (W4) aggregates CostUSD into hourly
// rollups per tenant per capability. LLMUsage is the per-call observed cost;
// MeteringRollup is the aggregated billable cost. The metering hook translates
// LLMUsage.CostUSD into MeteringEvent.CostUSD for the rollup pipeline.
type LLMUsage struct {
	InputTokens  int     `json:"input_tokens"`  // prompt tokens
	OutputTokens int     `json:"output_tokens"` // completion tokens
	CostUSD      float64 `json:"cost_usd"`      // observed cost in USD
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
