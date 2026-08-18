// Package otel bridges the GOLEM observability ports to the
// OpenTelemetry SDK (ADR-019: OTel is the observability contract).
// GenAI spans live here: genai.* attributes follow semconv v1.20.0 pinned
// (ADR-068). The SDK is a vendor dependency and stays in adapters/
// (ADR-045); internal code only sees ports.Tracer/Span.
package otel

import (
	"context"
	"errors"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/Rubentxu/golem/internal/ports"
)

// GENAI_SEMCONV_VERSION pins the GenAI semantic convention to v1.20.0
// (ADR-068). Any bump requires a new ADR.
const GenAISemconvVersion = "v1.20.0"

// GenAI attribute keys — internal string keys that map to OTel SDK attrs.
// These are NOT OTel types; they live in our port boundary so the SDK
// never leaks into internal/ (ADR-047).
const (
	// AttrGenaiSystem is the GenAI system name (e.g. "openai", "anthropic").
	AttrGenaiSystem = "genai.system"
	// AttrGenaiRequestModel is the model requested for the completion.
	AttrGenaiRequestModel = "genai.request.model"
	// AttrGenaiUsageInputTokens is the number of input tokens consumed.
	AttrGenaiUsageInputTokens = "genai.usage.input_tokens"
	// AttrGenaiUsageOutputTokens is the number of output tokens produced.
	AttrGenaiUsageOutputTokens = "genai.usage.output_tokens"
	// AttrGenaiResponseFinishReasons is why the model stopped generating.
	AttrGenaiResponseFinishReasons = "genai.response.finish_reasons"
	// AttrGenaiToolName is the name of a tool invoked.
	AttrGenaiToolName = "genai.tool.name"
	// AttrGenaiOperationName is the name of a GenAI operation.
	AttrGenaiOperationName = "genai.operation.name"
)

// ErrInvalidLLMSpanInput is returned when LLM span inputs are invalid.
var ErrInvalidLLMSpanInput = errors.New("otel/genai: invalid LLM span input")

// LLMStartInput carries the fields needed to start an LLM span.
type LLMStartInput struct {
	Tracer        ports.Tracer
	CorrelationID string
	System        string
	Model         string
	OperationName string
}

// LLMEndInput carries the fields needed to end an LLM span.
type LLMEndInput struct {
	Span           ports.Span
	InputTokens    int
	OutputTokens   int
	FinishReasons  []string
	Err           error
}

// StartLLMSpan begins a genai.llm.request span and returns the updated
// context and span. The span is NOT ended — callers must call EndLLMSpan.
//
// The span name follows OTel conventions: "genai.llm.request".
// CorrelationID is recorded as a span attribute so the span is linkable
// to the journal event via ADR-019.
func StartLLMSpan(ctx context.Context, in LLMStartInput) (context.Context, ports.Span) {
	if in.Tracer == nil {
		return ctx, ports.Span(nil)
	}
	attrs := []ports.Attr{
		{Key: AttrGenaiSystem, Value: in.System},
		{Key: AttrGenaiRequestModel, Value: in.Model},
		{Key: AttrGenaiOperationName, Value: in.OperationName},
	}
	if in.CorrelationID != "" {
		attrs = append(attrs, ports.Attr{Key: "correlation_id", Value: in.CorrelationID})
	}
	ctx, span := in.Tracer.Start(ctx, "genai.llm.request", attrs...)
	return ctx, span
}

// EndLLMSpan ends the LLM span and records usage + finish reasons.
// If err is non-nil the span is recorded as an error.
func EndLLMSpan(in LLMEndInput) {
	if in.Span == nil {
		return
	}
	attrs := []ports.Attr{
		{Key: AttrGenaiUsageInputTokens, Value: in.InputTokens},
		{Key: AttrGenaiUsageOutputTokens, Value: in.OutputTokens},
	}
	if len(in.FinishReasons) > 0 {
		attrs = append(attrs, ports.Attr{Key: AttrGenaiResponseFinishReasons, Value: joinFinishReasons(in.FinishReasons)})
	}
	in.Span.SetAttrs(attrs...)
	in.Span.End(in.Err)
}

// ToolStartInput carries the fields needed to start a tool invoke span.
type ToolStartInput struct {
	Tracer        ports.Tracer
	CorrelationID string
	ToolName      string
	OperationName string
}

// ToolEndInput carries the fields needed to end a tool invoke span.
type ToolEndInput struct {
	Span ports.Span
	Err error
}

// StartToolInvokeSpan begins a genai.tool.invoke span and returns the
// updated context and span.
func StartToolInvokeSpan(ctx context.Context, in ToolStartInput) (context.Context, ports.Span) {
	if in.Tracer == nil {
		return ctx, ports.Span(nil)
	}
	attrs := []ports.Attr{
		{Key: AttrGenaiToolName, Value: in.ToolName},
		{Key: AttrGenaiOperationName, Value: in.OperationName},
	}
	if in.CorrelationID != "" {
		attrs = append(attrs, ports.Attr{Key: "correlation_id", Value: in.CorrelationID})
	}
	ctx, span := in.Tracer.Start(ctx, "genai.tool.invoke", attrs...)
	return ctx, span
}

// EndToolInvokeSpan ends the tool invoke span. If err is non-nil the span
// is recorded as an error.
func EndToolInvokeSpan(in ToolEndInput) {
	if in.Span == nil {
		return
	}
	in.Span.End(in.Err)
}

// --- internal helpers ---

// joinFinishReasons produces a comma-separated string for the finish_reasons
// attribute (semconv v1.20.0 type: string array serialized as CSV).
func joinFinishReasons(reasons []string) string {
	if len(reasons) == 0 {
		return ""
	}
	if len(reasons) == 1 {
		return reasons[0]
	}
	result := reasons[0]
	for i := 1; i < len(reasons); i++ {
		result += "," + reasons[i]
	}
	return result
}

// toOtelAttrs converts ports.Attr slice to OTel attribute.KeyValue slice.
// This is the bridge from our internal attr keys (genai.*) to the SDK.
func toOtelAttrs(attrs []ports.Attr) []attribute.KeyValue {
	out := make([]attribute.KeyValue, 0, len(attrs))
	for _, a := range attrs {
		switch v := a.Value.(type) {
		case string:
			out = append(out, attribute.String(a.Key, v))
		case int:
			out = append(out, attribute.Int(a.Key, v))
		case int64:
			out = append(out, attribute.Int64(a.Key, v))
		case uint64:
			out = append(out, attribute.Int64(a.Key, int64(v)))
		case float64:
			out = append(out, attribute.Float64(a.Key, v))
		case bool:
			out = append(out, attribute.Bool(a.Key, v))
		default:
			out = append(out, attribute.String(a.Key, toString(v)))
		}
	}
	return out
}

func toString(v any) string {
	switch x := v.(type) {
	case time.Duration:
		return x.String()
	case error:
		return x.Error()
	default:
		if s, ok := v.(string); ok {
			return s
		}
		return ""
	}
}

// SDKAttr converts a GenAI internal attr key + value to an OTel SDK
// attribute.KeyValue. Used by adapters that need direct SDK-level attrs.
func SDKAttr(key string, value any) attribute.KeyValue {
	switch v := value.(type) {
	case string:
		return attribute.String(key, v)
	case int:
		return attribute.Int(key, v)
	case int64:
		return attribute.Int64(key, int64(v))
	case uint64:
		return attribute.Int64(key, int64(v))
	case float64:
		return attribute.Float64(key, v)
	case bool:
		return attribute.Bool(key, v)
	default:
		return attribute.String(key, toString(v))
	}
}

// SpanFromSDK returns a ports.Span wrapping an OTel span, for cases where
// the caller has a raw SDK span (e.g., Links from OTel SDK).
func SpanFromSDK(s trace.Span) ports.Span {
	if s == nil {
		return nil
	}
	return &sdkSpan{s: s}
}

type sdkSpan struct{ s trace.Span }

func (sp *sdkSpan) End(err error) {
	if sp.s == nil {
		return
	}
	if err != nil {
		sp.s.RecordError(err)
		sp.s.SetStatus(codes.Error, err.Error())
	}
	sp.s.End()
}

func (sp *sdkSpan) SetAttrs(attrs ...ports.Attr) {
	if sp.s == nil {
		return
	}
	sp.s.SetAttributes(toOtelAttrs(attrs)...)
}
