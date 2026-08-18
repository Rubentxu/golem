package openai

import (
	"os"
	"testing"

	"github.com/Rubentxu/golem/internal/ports"
)

func TestOpenAI_NoRetentionHonest(t *testing.T) {
	// Save original env var state
	orig := os.Getenv("OPENAI_COMPAT_NO_RETENTION")
	os.Unsetenv("OPENAI_COMPAT_NO_RETENTION")
	defer func() {
		if orig != "" {
			os.Setenv("OPENAI_COMPAT_NO_RETENTION", orig)
		}
	}()

	adapter := New(AdapterOptions{
		BaseURL: "https://api.example.com/v1",
		APIKey:  "test-key",
		Model:   "test-model",
	})

	caps := adapter.Capabilities()

	// Default: NoRetention must be false — OpenAI-compatible providers retain
	// data by default; this is the honest, security-correct default per ADR-061.
	if caps.NoRetention {
		t.Errorf("Capabilities().NoRetention = true; want false (default for OpenAI-compatible)")
	}

	// When env var is set to "true", NoRetention should be honoured.
	os.Setenv("OPENAI_COMPAT_NO_RETENTION", "true")
	caps = adapter.Capabilities()
	if !caps.NoRetention {
		t.Errorf("Capabilities().NoRetention = false; want true when OPENAI_COMPAT_NO_RETENTION=true")
	}
}

func TestAdapter_Complete_InvalidRequest(t *testing.T) {
	adapter := New(AdapterOptions{
		BaseURL: "https://api.example.com/v1",
		APIKey:  "test-key",
		Model:   "test-model",
	})

	// Empty prompt should return ErrInvalidLLMRequest.
	_, err := adapter.Complete(nil, ports.LLMRequest{})
	if err != ports.ErrInvalidLLMRequest {
		t.Errorf("Complete with empty request: err = %v; want %v", err, ports.ErrInvalidLLMRequest)
	}
}
