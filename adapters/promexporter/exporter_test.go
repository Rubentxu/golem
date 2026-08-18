package proexporter

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandler(t *testing.T) {
	t.Parallel()

	handler := Handler()
	if handler == nil {
		t.Fatal("Handler() returned nil")
	}

	// Verify it's a valid HTTP handler by making a request
	server := httptest.NewServer(handler)
	defer server.Close()

	resp, err := server.Client().Get(server.URL + "/metrics")
	if err != nil {
		t.Fatalf("Failed to scrape /metrics: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

func TestNewRegistry(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	if registry == nil {
		t.Fatal("NewRegistry() returned nil")
	}
}

func TestNewCounter(t *testing.T) {
	t.Parallel()

	// Note: MustRegister panics if the collector is invalid.
	// In production, counters are created once at startup.
	counter := NewCounter(
		"test_counter",
		"A test counter",
		"status",
	)
	if counter == nil {
		t.Fatal("NewCounter() returned nil")
	}
}

func TestNewHistogram(t *testing.T) {
	t.Parallel()

	histogram := NewHistogram(
		"test_histogram",
		"A test histogram",
		[]float64{1, 5, 10, 50, 100},
		"method",
	)
	if histogram == nil {
		t.Fatal("NewHistogram() returned nil")
	}
}
