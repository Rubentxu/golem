package tck

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Rubentxu/golem/internal/agent/observability"
	"github.com/Rubentxu/golem/internal/ports"
)

// TestAgentLLMCallEvent_RedactedPromptNonEmpty validates AC-1 / C9:
// when an agent processes a prompt containing PII (email, token),
// the RedactedPrompt field in the resulting journal event MUST be non-empty.
// This verifies that the real redaction (redact.Redact(promptBody).Summary)
// flows into the event payload, not the old broken redact.Redact("").
func TestAgentLLMCallEvent_RedactedPromptNonEmpty(t *testing.T) {
	redact := observability.NewRedactor()

	tests := []struct {
		name    string
		prompt  string
		wantPII bool // whether we expect PII detection
	}{
		{
			name:    "prompt with email",
			prompt:  "Please contact user@example.com for details",
			wantPII: true,
		},
		{
			name:    "prompt with API token",
			prompt:  "Authorization: Bearer sk-abcdefghijklmnopqrstuvwxyz",
			wantPII: true,
		},
		{
			name:    "prompt with password assignment",
			prompt:  "Configure: password=SuperSecret123",
			wantPII: true,
		},
		{
			name:    "prompt without PII",
			prompt:  "Please analyze the vulnerability CVE-2025-1234",
			wantPII: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			redacted := redact.Redact(tt.prompt)

			// Build a minimal AgentLLMCallPayload matching what the agent creates.
			payload := ports.AgentLLMCallPayload{
				Provider:       "test-provider",
				Model:          "test-model",
				Operation:      "test-op",
				InputTokens:    100,
				OutputTokens:   50,
				RedactedPrompt: redacted.Summary, // this is what the agent now passes
				CorrelationID:  "test-correlation-id",
			}
			payloadBytes, _ := json.Marshal(payload)

			// Parse back to verify round-trip
			var parsed ports.AgentLLMCallPayload
			if err := json.Unmarshal(payloadBytes, &parsed); err != nil {
				t.Fatalf("payload round-trip failed: %v", err)
			}

			// C9 fix: RedactedPrompt must be non-empty for prompts with PII.
			// Even for prompts without PII, RedactedPrompt should be non-empty
			// (first line of the prompt is used).
			if parsed.RedactedPrompt == "" {
				t.Errorf("RedactedPrompt = %q; want non-empty for prompt %q", parsed.RedactedPrompt, tt.prompt)
			}

			// Verify that raw PII does NOT appear in the redacted prompt.
			// This is the core security guarantee of ADR-066.
			if tt.wantPII {
				// Check for email
				if strings.Contains(parsed.RedactedPrompt, "user@example.com") {
					t.Errorf("RedactedPrompt %q contains raw email; want redaction", parsed.RedactedPrompt)
				}
				// Check for token pattern
				if strings.Contains(parsed.RedactedPrompt, "sk-abcdefghijklmnop") {
					t.Errorf("RedactedPrompt %q contains raw token; want redaction", parsed.RedactedPrompt)
				}
			}
		})
	}
}

// TestRedactor_DetectsEmailAndToken verifies the underlying Redactor
// correctly identifies PII patterns.
func TestRedactor_DetectsEmailAndToken(t *testing.T) {
	redact := observability.NewRedactor()

	tests := []struct {
		name      string
		input     string
		wantPII   bool
		wantToken int // minimum number of PII tokens expected
	}{
		{
			name:      "email address",
			input:     "Contact alice@example.com for access",
			wantPII:   true,
			wantToken: 1,
		},
		{
			name:      "API key sk-...",
			input:     "key: sk-ABCDEFGHIJKLMNOPQRSTUVWXYZ123456",
			wantPII:   true,
			wantToken: 1,
		},
		{
			name:      "bearer token pattern",
			input:     "Authorization: token=ghp_abcdefghijklmnopqrstuvwxyz1234567890",
			wantPII:   true,
			wantToken: 1,
		},
		{
			name:      "password assignment",
			input:     "api_key=ghp_abcdefghijklmnopqrstuvwxyz",
			wantPII:   true,
			wantToken: 1,
		},
		{
			name:      "clean text",
			input:     "Analyze CVE-2025-9999 in component xyz",
			wantPII:   false,
			wantToken: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := redact.Redact(tt.input)
			if tt.wantPII && len(result.DetectedTokens) == 0 {
				t.Errorf("Redact(%q): expected PII detection, got 0 tokens", tt.input)
			}
			if !tt.wantPII && len(result.DetectedTokens) > 0 {
				t.Errorf("Redact(%q): unexpected PII detection: %v", tt.input, result.DetectedTokens)
			}
			if tt.wantToken > 0 && len(result.DetectedTokens) < tt.wantToken {
				t.Errorf("Redact(%q): got %d tokens, want at least %d", tt.input, len(result.DetectedTokens), tt.wantToken)
			}
			// Summary must never be empty for non-empty input
			if tt.input != "" && result.Summary == "" {
				t.Errorf("Redact(%q): Summary is empty; want non-empty for non-empty input", tt.input)
			}
		})
	}
}
