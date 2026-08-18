// Package openai provides an OpenAI-compatible LLM adapter using net/http stdlib.
package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/Rubentxu/golem/internal/ports"
)

// Adapter implements ports.LLMProvider using OpenAI-compatible HTTP API.
type Adapter struct {
	baseURL    string
	apiKey     string
	model      string
	httpClient *http.Client
}

// AdapterOptions configures the OpenAI-compatible adapter.
type AdapterOptions struct {
	BaseURL string // e.g., "https://api.openai.com/v1"
	APIKey  string
	Model   string
	Timeout time.Duration
}

// New creates a new OpenAI-compatible adapter.
func New(opts AdapterOptions) *Adapter {
	if opts.Timeout == 0 {
		opts.Timeout = 30 * time.Second
	}
	return &Adapter{
		baseURL: opts.BaseURL,
		apiKey:  opts.APIKey,
		model:   opts.Model,
		httpClient: &http.Client{
			Timeout: opts.Timeout,
		},
	}
}

// Complete sends a completion request to the OpenAI-compatible API.
// It retries up to 2 times with exponential backoff (200ms → 1.5s).
// NOTE: This adapter does NOT support streaming. Completions are returned as a whole.
// Code review must enforce: no SSE, no streaming response handling.
func (a *Adapter) Complete(ctx context.Context, req ports.LLMRequest) (ports.LLMResponse, error) {
	url := fmt.Sprintf("%s/chat/completions", a.baseURL)

	payload := map[string]any{
		"model": req.Model,
		"messages": []map[string]string{
			{"role": "user", "content": req.Prompt},
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return ports.LLMResponse{}, ports.ErrInvalidLLMRequest
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return ports.LLMResponse{}, ports.ErrInvalidLLMRequest
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if a.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+a.apiKey)
	}

	var lastErr error
	backoff := 200 * time.Millisecond

	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ports.LLMResponse{}, ctx.Err()
			case <-time.After(backoff):
			}
			backoff = time.Duration(float64(backoff) * 1.5)
			if backoff > 1500*time.Millisecond {
				backoff = 1500 * time.Millisecond
			}
		}

		resp, err := a.httpClient.Do(httpReq)
		if err != nil {
			lastErr = err
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			var result openaiResponse
			if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
				return ports.LLMResponse{}, ports.ErrProviderUnavailable
			}
			if len(result.Choices) == 0 {
				return ports.LLMResponse{}, ports.ErrProviderUnavailable
			}
			return ports.LLMResponse{
				TenantID:  req.TenantID,
				Content:   result.Choices[0].Message.Content,
				Model:     result.Model,
				Provider:  "openai-compatible",
				TokenUsed: result.Usage.CompletionTokens,
			}, nil
		}
		lastErr = fmt.Errorf("openai: status %d", resp.StatusCode)
	}

	return ports.LLMResponse{}, lastErr
}

// Capabilities returns the adapter capabilities.
func (a *Adapter) Capabilities() ports.LLMProviderCapabilities {
	return ports.LLMProviderCapabilities{
		NoRetention: true,
		Region:      "remote",
		Audit:       true,
	}
}

// openaiResponse represents the OpenAI API response structure.
type openaiResponse struct {
	ID      string `json:"id"`
	Model   string `json:"model"`
	Choices []struct {
		Message struct {
			Content string `json:"content"`
			Role    string `json:"role"`
		} `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}
