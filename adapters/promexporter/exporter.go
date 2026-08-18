// Package proexporter provides a Prometheus metrics exporter adapter (REQ-OBS-MOD-002).
// It exposes GOLEM metrics via the Prometheus scrape endpoint using prometheus-client-golang.
package proexporter

import (
	"context"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Handler returns an HTTP handler for Prometheus to scrape.
// The handler collects metrics registered with the default registry.
func Handler() http.Handler {
	return promhttp.HandlerFor(
		prometheus.DefaultGatherer,
		promhttp.HandlerOpts{EnableOpenMetrics: true},
	)
}

// NewRegistry creates a new Prometheus registry.
func NewRegistry() *prometheus.Registry {
	return prometheus.NewRegistry()
}

// HandlerFor returns an HTTP handler for a custom registry.
func HandlerFor(registry *prometheus.Registry) http.Handler {
	return promhttp.HandlerFor(
		registry,
		promhttp.HandlerOpts{EnableOpenMetrics: true},
	)
}

// Counter is a Prometheus counter bridge.
type Counter struct {
	counter *prometheus.CounterVec
}

// NewCounter creates a new counter registered with the default registry.
func NewCounter(name, help string, labels ...string) *Counter {
	c := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: name,
			Help: help,
		},
		labels,
	)
	prometheus.MustRegister(c)
	return &Counter{counter: c}
}

// Add implements ports.Counter.
func (c *Counter) Add(ctx context.Context, delta int64, labelValues ...string) {
	c.counter.WithLabelValues(labelValues...).Add(float64(delta))
}

// Histogram is a Prometheus histogram bridge.
type Histogram struct {
	histogram *prometheus.HistogramVec
}

// NewHistogram creates a new histogram registered with the default registry.
func NewHistogram(name, help string, buckets []float64, labels ...string) *Histogram {
	h := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    name,
			Help:    help,
			Buckets: buckets,
		},
		labels,
	)
	prometheus.MustRegister(h)
	return &Histogram{histogram: h}
}

// Record implements ports.Histogram.
func (h *Histogram) Record(ctx context.Context, value float64, labelValues ...string) {
	h.histogram.WithLabelValues(labelValues...).Observe(value)
}
