// Package obs provides stdlib-only implementations of the observability
// ports: a slog-based structured Logger (Go 1.21+ log/slog is standard
// library, so it stays out of adapters) and no-op Tracer/Meter for tests
// and uninstrumented deployments. The real OpenTelemetry bridge lives in
// adapters/observability/otel (ADR-019: OTel is the contract; the SDK is
// a vendor dependency behind the port, ADR-045).
package obs

import (
	"context"
	"log/slog"
	"os"

	"github.com/Rubentxu/golem/internal/ports"
)

// No-op implementations: zero overhead, safe for concurrent use.

type noopTracer struct{}

func (noopTracer) Start(ctx context.Context, _ string, _ ...ports.Attr) (context.Context, ports.Span) {
	return ctx, noopSpan{}
}

type noopSpan struct{}

func (noopSpan) End(_ error)              {}
func (noopSpan) SetAttrs(_ ...ports.Attr) {}

type noopMeter struct{}

func (noopMeter) Counter(string) ports.Counter     { return noopCounter{} }
func (noopMeter) Histogram(string) ports.Histogram { return noopHistogram{} }

type noopCounter struct{}

func (noopCounter) Add(context.Context, int64, ...ports.Attr) {}

type noopHistogram struct{}

func (noopHistogram) Record(context.Context, float64, ...ports.Attr) {}

// NoopTracer returns a tracer that instruments nothing.
func NoopTracer() ports.Tracer { return noopTracer{} }

// NoopMeter returns a meter that instruments nothing.
func NoopMeter() ports.Meter { return noopMeter{} }

// Fill completes a ports.Observability with no-ops for missing members
// so the struct is always safe to use.
func Fill(o ports.Observability) ports.Observability {
	if o.Logger == nil {
		o.Logger = NoopLogger()
	}
	if o.Tracer == nil {
		o.Tracer = NoopTracer()
	}
	if o.Meter == nil {
		o.Meter = NoopMeter()
	}
	return o
}

// --- slog logger ---

// SlogLogger adapts slog to the ports.Logger contract, enriching every
// record with the correlation fields found in ctx (correlation_id,
// tenant_id, actor, command_id, event_id) and falling back to the
// context tenant when present (ADR-008).
type SlogLogger struct {
	l *slog.Logger
}

// NewSlog builds a JSON structured logger on w (defaults to stderr) at
// the given level.
func NewSlog(level slog.Level, w *os.File) *SlogLogger {
	if w == nil {
		w = os.Stderr
	}
	return &SlogLogger{l: slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{Level: level}))}
}

// NewSlogFrom builds on an existing slog.Logger (tests capture buffers).
func NewSlogFrom(l *slog.Logger) *SlogLogger { return &SlogLogger{l: l} }

func (s *SlogLogger) log(ctx context.Context, level slog.Level, msg string, attrs []ports.Attr) {
	args := make([]any, 0, len(attrs)+5)
	if c, ok := ports.CorrelationFrom(ctx); ok {
		if c.CorrelationID != "" {
			args = append(args, slog.String("correlation_id", c.CorrelationID))
		}
		if c.TenantID != "" {
			args = append(args, slog.String("tenant_id", c.TenantID))
		}
		if c.ActorType != "" || c.ActorID != "" {
			args = append(args, slog.String("actor", c.ActorType+":"+c.ActorID))
		}
		if c.CommandID != "" {
			args = append(args, slog.String("command_id", c.CommandID))
		}
		if c.EventID != "" {
			args = append(args, slog.String("event_id", c.EventID))
		}
	} else if tenant, ok := ports.TenantFrom(ctx); ok {
		args = append(args, slog.String("tenant_id", string(tenant)))
	}
	for _, a := range attrs {
		args = append(args, slog.Any(a.Key, a.Value))
	}
	s.l.Log(ctx, level, msg, args...)
}

func (s *SlogLogger) Debug(ctx context.Context, msg string, attrs ...ports.Attr) {
	s.log(ctx, slog.LevelDebug, msg, attrs)
}
func (s *SlogLogger) Info(ctx context.Context, msg string, attrs ...ports.Attr) {
	s.log(ctx, slog.LevelInfo, msg, attrs)
}
func (s *SlogLogger) Warn(ctx context.Context, msg string, attrs ...ports.Attr) {
	s.log(ctx, slog.LevelWarn, msg, attrs)
}
func (s *SlogLogger) Error(ctx context.Context, msg string, attrs ...ports.Attr) {
	s.log(ctx, slog.LevelError, msg, attrs)
}

// NoopLogger returns a logger that discards everything.
func NoopLogger() ports.Logger { return NewSlogFrom(slog.New(slog.DiscardHandler)) }
