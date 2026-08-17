// Package otel bridges the GOLEM observability ports to the
// OpenTelemetry SDK (ADR-019: OTel is the observability contract). The
// SDK is a third-party dependency and therefore lives in adapters/
// (ADR-045); internal code only ever sees ports.Logger/Tracer/Meter.
//
// Setup produces a ready-to-use ports.Observability with OTLP exporters
// (HTTP, env-configurable endpoints) falling back to stdout exporters
// when OTEL_EXPORTER_OTLP_ENDPOINT is absent — hermetic for development.
// The returned shutdown flushes and stops all providers.
package otel

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/metric"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"

	"github.com/Rubentxu/golem/internal/obs"
	"github.com/Rubentxu/golem/internal/ports"
)

// Setup wires OTel SDK exporters and returns the observability bundle
// plus a shutdown function that must be called on process exit.
func Setup(ctx context.Context, serviceName, serviceVersion string) (ports.Observability, func(context.Context) error, error) {
	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(semconv.SchemaURL,
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion(serviceVersion),
		),
	)
	if err != nil {
		return ports.Observability{}, nil, fmt.Errorf("otel: resource: %w", err)
	}

	// Traces.
	tracerProvider, closeTrace, err := tracerProvider(res)
	if err != nil {
		return ports.Observability{}, nil, err
	}

	// Metrics.
	meterProvider, closeMeter, err := meterProvider(res)
	if err != nil {
		_ = closeTrace(ctx)
		return ports.Observability{}, nil, err
	}

	shutdown := func(ctx context.Context) error {
		errT := closeTrace(ctx)
		errM := closeMeter(ctx)
		if errT != nil {
			return errT
		}
		return errM
	}

	otel.SetTracerProvider(tracerProvider)
	otel.SetMeterProvider(meterProvider)

	return ports.Observability{
		Logger: obs.NewSlog(slogLevel(), nil),
		Tracer: Tracer{t: tracerProvider.Tracer("golem")},
		Meter:  Meter{m: meterProvider.Meter("golem")},
	}, shutdown, nil
}

func tracerProvider(res *resource.Resource) (*sdktrace.TracerProvider, func(context.Context) error, error) {
	// TODO M5: OTLP exporter when OTEL_EXPORTER_OTLP_ENDPOINT is set.
	exp, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
	if err != nil {
		return nil, nil, fmt.Errorf("otel: stdout trace exporter: %w", err)
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(res),
	)
	return tp, tp.Shutdown, nil
}

func meterProvider(res *resource.Resource) (*sdkmetric.MeterProvider, func(context.Context) error, error) {
	// TODO M5: OTLP exporter when OTEL_EXPORTER_OTLP_ENDPOINT is set.
	exp, err := stdoutmetric.New()
	if err != nil {
		return nil, nil, fmt.Errorf("otel: stdout metric exporter: %w", err)
	}
	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exp, sdkmetric.WithInterval(15*time.Second))),
		sdkmetric.WithResource(res),
	)
	return mp, mp.Shutdown, nil
}

func slogLevel() slog.Level {
	switch os.Getenv("GOLEM_LOG_LEVEL") {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// --- Tracer bridge ---

// Tracer adapts an otel Tracer to ports.Tracer.
type Tracer struct{ t trace.Tracer }

// Start begins a span and injects it into the returned ctx; errors set
// span status on End.
func (tr Tracer) Start(ctx context.Context, name string, attrs ...ports.Attr) (context.Context, ports.Span) {
	otelAttrs := convertAttrs(attrs)
	ctx, s := tr.t.Start(ctx, name, trace.WithAttributes(otelAttrs...))
	return ctx, span{s: s, sc: trace.SpanContextFromContext(ctx)}
}

type span struct {
	s  trace.Span
	sc trace.SpanContext
}

func (sp span) End(err error) {
	if sp.s == nil {
		return
	}
	if err != nil {
		sp.s.RecordError(err)
		sp.s.SetStatus(codes.Error, err.Error())
	}
	sp.s.End()
}

func (sp span) SetAttrs(attrs ...ports.Attr) {
	if sp.s == nil {
		return
	}
	sp.s.SetAttributes(convertAttrs(attrs)...)
}

// TraceID returns the active trace id (empty when untraced) — the
// correlation field of OBSERVABILITY.md.
func (sp span) TraceID() string {
	return sp.sc.TraceID().String()
}

func convertAttrs(attrs []ports.Attr) []attribute.KeyValue {
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
			out = append(out, attribute.String(a.Key, fmt.Sprintf("%v", v)))
		}
	}
	return out
}

// --- Meter bridge ---

// Meter adapts an otel Meter to ports.Meter.
type Meter struct{ m metric.Meter }

// Counter yields an adding monotonic instrument.
func (mt Meter) Counter(name string) ports.Counter {
	c, err := mt.m.Int64Counter(name)
	if err != nil {
		return counterBridge{c: metricnoop.Int64Counter{}}
	}
	return counterBridge{c: c}
}

// Histogram yields a distribution instrument.
func (mt Meter) Histogram(name string) ports.Histogram {
	h, err := mt.m.Float64Histogram(name)
	if err != nil {
		return histogramBridge{h: metricnoop.Float64Histogram{}}
	}
	return histogramBridge{h: h}
}

type counterBridge struct {
	c metric.Int64Counter
}

func (b counterBridge) Add(ctx context.Context, delta int64, attrs ...ports.Attr) {
	b.c.Add(ctx, delta, metric.WithAttributes(convertAttrs(attrs)...))
}

type histogramBridge struct {
	h metric.Float64Histogram
}

func (b histogramBridge) Record(ctx context.Context, v float64, attrs ...ports.Attr) {
	b.h.Record(ctx, v, metric.WithAttributes(convertAttrs(attrs)...))
}
