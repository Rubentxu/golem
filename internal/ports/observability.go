package ports

import "context"

// Attr is a structured key/value pair for logs, traces and metrics.
// Values are limited to the OTel-compatible primitives; adapters bridge
// them to their native representation (ADR-019).
type Attr struct {
	Key   string
	Value any
}

// A builds an Attr.
func A(key string, value any) Attr { return Attr{Key: key, Value: value} }

// Span is an in-flight unit of work. Adapters bridge to OTel spans;
// implementations must be safe when End is called twice.
type Span interface {
	// End completes the span; err (if non-nil) is recorded as span error.
	End(err error)
	// SetAttrs annotates the span.
	SetAttrs(attrs ...Attr)
}

// Tracer starts correlated spans (trace path per OBSERVABILITY.md:
// Command → Journal → outbox → projection).
type Tracer interface {
	Start(ctx context.Context, name string, attrs ...Attr) (context.Context, Span)
}

// Counter is a monotonically increasing metric. Attributes must be
// low-cardinality: NEVER tenant ids (OBSERVABILITY.md — tenant is treated
// carefully in metrics); use command names, results, route templates.
type Counter interface {
	Add(ctx context.Context, delta int64, attrs ...Attr)
}

// Histogram records value distributions (latencies, batch sizes).
type Histogram interface {
	Record(ctx context.Context, value float64, attrs ...Attr)
}

// Meter yields named metric instruments.
type Meter interface {
	Counter(name string) Counter
	Histogram(name string) Histogram
}

// Logger emits structured logs: redacted by default — no full event
// payloads (OBSERVABILITY.md). Implementations MUST enrich every record
// with the correlation fields found in ctx (correlation_id, tenant_id,
// actor, command_id) so callers never repeat them.
type Logger interface {
	Debug(ctx context.Context, msg string, attrs ...Attr)
	Info(ctx context.Context, msg string, attrs ...Attr)
	Warn(ctx context.Context, msg string, attrs ...Attr)
	Error(ctx context.Context, msg string, attrs ...Attr)
}

// Observability bundles the three contracts. The zero value is usable:
// all nil members resolve to no-ops (see obs.Noop), so components can
// embed Observability by value without nil checks.
type Observability struct {
	Logger Logger
	Tracer Tracer
	Meter  Meter
}

// --- correlation context (ADR-019: trace_id, correlation_id, event_id,
// command_id flow through context; OTel owns trace propagation) ---

type correlationCtxKey struct{}

// Correlation carries the per-operation identifiers that logs and spans
// must surface.
type Correlation struct {
	CorrelationID string
	TenantID      string
	ActorType     string
	ActorID       string
	CommandID     string
	EventID       string
}

// WithCorrelation returns a ctx carrying the correlation fields.
func WithCorrelation(ctx context.Context, c Correlation) context.Context {
	return context.WithValue(ctx, correlationCtxKey{}, c)
}

// CorrelationFrom extracts the correlation fields; ok is false when the
// ctx carries none.
func CorrelationFrom(ctx context.Context) (Correlation, bool) {
	c, ok := ctx.Value(correlationCtxKey{}).(Correlation)
	return c, ok
}
