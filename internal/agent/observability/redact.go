// Package observability provides agent-facing observability primitives:
// Redactor for PII detection in prompts and responses (ADR-066).
package observability

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

const (
	// MaxRedactedSummaryBytes is the maximum byte length of a RedactedSummary.
	// Content beyond this limit is truncated with TruncationMarker (ADR-066).
	MaxRedactedSummaryBytes = 512

	// RedactedEmail is the replacement text for detected email addresses.
	RedactedEmail = "[REDACTED-email]"
	// RedactedToken is the replacement text for detected API keys / tokens.
	RedactedToken = "[REDACTED-token]"
	// RedactedURL is the replacement text for URLs with embedded credentials.
	RedactedURL = "[REDACTED-url]"
	// RedactedSecret is the replacement text for plain-text secrets/keys.
	RedactedSecret = "[REDACTED-secret]"

	// TruncationMarker is appended when RedactedSummary exceeds MaxRedactedSummaryBytes.
	TruncationMarker = " [TRUNCATED]"
)

// --- PII type constants ---

// PIIKind classifies the category of personally identifiable information.
type PIIKind string

const (
	PIIEmail   PIIKind = "email"
	PIIKindToken PIIKind = "token"
	PIIURL    PIIKind = "url"
	PIISecret PIIKind = "secret"
)

// PIIToken records a detected PII token.
type PIIToken struct {
	Kind   PIIKind // category of the detected PII
	Value  string  // original matched text (may be redacted in summary)
	Offset int     // byte offset in the original string
	Length int     // byte length of the matched text
}

// RedactedSummary is the result of redacting a string. It never contains
// raw PII; the Summary field is guaranteed to be ≤ MaxRedactedSummaryBytes.
// ADR-066: summary = SHA-256 + first line + char count + "# PII tokens".
type RedactedSummary struct {
	Summary        string      // redacted text, max 512 bytes
	DetectedTokens []PIIToken // all PII tokens found (for journal evidence)
}

// Redactor detects PII in arbitrary text and produces a safe-to-journal
// summary (ADR-066). It is used by LLM adapters and proposal journal
// before any content enters the audit log.
type Redactor struct{}

// NewRedactor builds a Redactor.
func NewRedactor() *Redactor { return &Redactor{} }

// Redact scans the input for known PII patterns and returns a RedactedSummary.
// The returned Summary never contains raw PII and is capped at 512 bytes.
// Patterns detected: emails, API keys/tokens, URLs with credentials,
// and plain-text secrets (password=, secret=, api_key=).
//
// Zero or empty input returns a RedactedSummary with an empty Summary.
func (r *Redactor) Redact(input string) RedactedSummary {
	if input == "" {
		return RedactedSummary{}
	}

	tokens := r.detectPII(input)
	if len(tokens) == 0 {
		return RedactedSummary{Summary: firstLineOrValue(input), DetectedTokens: nil}
	}

	// Build redacted text, replacing each PII token with its marker.
	// We process in offset order to avoid double-replacing overlapping regions.
	// Since we replace shorter or equal-length markers, offsets remain valid.
	result := input
	offsetDelta := 0 // accumulated shift as we replace tokens

	for _, tok := range tokens {
		var marker string
		switch tok.Kind {
		case PIIEmail:
			marker = RedactedEmail
		case PIIKindToken:
			marker = RedactedToken
		case PIIURL:
			marker = RedactedURL
		case PIISecret:
			marker = RedactedSecret
		default:
			marker = RedactedToken
		}

		start := tok.Offset + offsetDelta
		end := start + tok.Length
		if start < 0 || end > len(result) {
			continue // out-of-bounds, skip
		}
		result = result[:start] + marker + result[end:]
		offsetDelta += len(marker) - tok.Length
	}

	// Truncate to MaxRedactedSummaryBytes if needed.
	summary := enforceMaxBytes(result, MaxRedactedSummaryBytes)
	return RedactedSummary{Summary: summary, DetectedTokens: tokens}
}

// --- private ---

var (
	emailRe = regexp.MustCompile(`[\w.+-]+@[\w-]+\.[\w.-]+`)

	// apiKeyRe matches "sk-" prefixed API keys (OpenAI, Anthropic, etc.)
	apiKeyRe = regexp.MustCompile(`\bsk-[A-Za-z0-9_-]{20,}\b`)

	// tokenRe matches bare bearer-like token patterns: token=<value>, Bearer <value>
	tokenRe = regexp.MustCompile(`(?i)\b(token| bearer)\s*[:=]\s*[A-Za-z0-9_/-]{10,}`)

	// urlWithCredsRe matches http://user:pass@host patterns — sensitive URLs.
	urlWithCredsRe = regexp.MustCompile(`https?://[^\s:@]+:[^\s:@]+@[^\s]+`)

	// secretRe matches common secret assignments: password=, secret=, api_key=, apikey=
	secretRe = regexp.MustCompile(`(?i)\b(password|secret|api[_-]?key|apikey)\s*[:=]\s*[^\s,}]+`)
)

func (r *Redactor) detectPII(input string) []PIIToken {
	var tokens []PIIToken

	// Email addresses.
	for _, m := range emailRe.FindAllStringIndex(input, -1) {
		tokens = append(tokens, PIIToken{
			Kind:   PIIEmail,
			Value:  input[m[0]:m[1]],
			Offset: m[0],
			Length: m[1] - m[0],
		})
	}

	// API keys (sk-...).
	for _, m := range apiKeyRe.FindAllStringIndex(input, -1) {
		tokens = append(tokens, PIIToken{
			Kind:   PIIKindToken,
			Value:  input[m[0]:m[1]],
			Offset: m[0],
			Length: m[1] - m[0],
		})
	}

	// Bearer / token= patterns.
	for _, m := range tokenRe.FindAllStringIndex(input, -1) {
		tokens = append(tokens, PIIToken{
			Kind:   PIIKindToken,
			Value:  input[m[0]:m[1]],
			Offset: m[0],
			Length: m[1] - m[0],
		})
	}

	// URLs with embedded credentials.
	for _, m := range urlWithCredsRe.FindAllStringIndex(input, -1) {
		tokens = append(tokens, PIIToken{
			Kind:   PIIURL,
			Value:  input[m[0]:m[1]],
			Offset: m[0],
			Length: m[1] - m[0],
		})
	}

	// Secret= / password= / api_key= assignments.
	for _, m := range secretRe.FindAllStringIndex(input, -1) {
		tokens = append(tokens, PIIToken{
			Kind:   PIISecret,
			Value:  input[m[0]:m[1]],
			Offset: m[0],
			Length: m[1] - m[0],
		})
	}

	// Sort by offset ascending, then by length descending (longer first)
	// to avoid overlapping replacements when shorter patterns are inside longer.
	sortPIITokensByOffset(tokens)

	// Remove overlaps: keep the longer (earlier) token when regions overlap.
	tokens = dedupOverlapping(tokens)

	return tokens
}

// sortPIITokensByOffset sorts tokens by offset ascending.
func sortPIITokensByOffset(tokens []PIIToken) {
	for i := 0; i < len(tokens); i++ {
		for j := i + 1; j < len(tokens); j++ {
			if tokens[j].Offset < tokens[i].Offset ||
				(tokens[j].Offset == tokens[i].Offset && tokens[j].Length > tokens[i].Length) {
				tokens[i], tokens[j] = tokens[j], tokens[i]
			}
		}
	}
}

// dedupOverlapping removes tokens whose regions are fully contained within
// an earlier token.
func dedupOverlapping(tokens []PIIToken) []PIIToken {
	if len(tokens) == 0 {
		return tokens
	}
	result := []PIIToken{tokens[0]}
	for i := 1; i < len(tokens); i++ {
		curr := tokens[i]
		contained := false
		for _, prev := range result {
			if curr.Offset >= prev.Offset && curr.Offset+curr.Length <= prev.Offset+prev.Length {
				contained = true
				break
			}
		}
		if !contained {
			result = append(result, curr)
		}
	}
	return result
}

// firstLineOrValue returns the first line of input (up to \n), or the
// whole input if no newline, capped at 200 bytes for the non-PII case.
func firstLineOrValue(input string) string {
	if i := strings.IndexByte(input, '\n'); i >= 0 {
		input = input[:i]
	}
	if len(input) > 200 {
		input = input[:200]
	}
	return input
}

// enforceMaxBytes truncates s to at most maxBytes bytes, counting UTF-8
// runes correctly. It appends TruncationMarker if truncation occurred.
func enforceMaxBytes(s string, maxBytes int) string {
	if maxBytes <= 0 {
		return ""
	}
	if utf8.RuneCountInString(s) <= maxBytes {
		return s
	}

	// Walk runes until we reach the byte limit.
	result := make([]rune, 0, maxBytes)
	byteLen := 0
	for _, r := range s {
		rl := utf8.RuneLen(r)
		if byteLen+rl > maxBytes-len(TruncationMarker) {
			break
		}
		result = append(result, r)
		byteLen += rl
	}
	return string(result) + TruncationMarker
}
