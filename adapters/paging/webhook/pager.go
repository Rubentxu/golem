// Package webhook provides a PagerDuty-compatible webhook paging adapter (Q3).
package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/Rubentxu/golem/internal/ports"
)

// Client is a PagerDuty-compatible webhook paging adapter.
type Client struct {
	httpClient *http.Client
	routingKey string
	serviceKey string
}

// NewClient creates a PagerDuty webhook client.
// routingKey is the PagerDuty Routing Key (Events API v2).
// serviceKey is the PagerDuty Service Key (Events API v1).
func NewClient(routingKey, serviceKey string) *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		routingKey: routingKey,
		serviceKey: serviceKey,
	}
}

// Page implements ports.Paging. It sends alerts to PagerDuty.
func (c *Client) Page(ctx context.Context, alert ports.Alert) error {
	if c.routingKey != "" {
		return c.pageEventsAPIV2(ctx, alert)
	}
	if c.serviceKey != "" {
		return c.pageEventsAPIV1(ctx, alert)
	}
	return nil // no-op if neither key is configured
}

// PDRoutingKey returns the PagerDuty routing key.
func (c *Client) PDRoutingKey() string { return c.routingKey }

// PDServiceKey returns the PagerDuty service key.
func (c *Client) PDServiceKey() string { return c.serviceKey }

// --- PagerDuty Events API v2 ---

type pdEventV2 struct {
	RoutingKey  string      `json:"routing_key"`
	EventAction string      `json:"event_action"` // trigger, acknowledge, resolve
	DedupKey    string      `json:"dedup_key,omitempty"`
	Payload     pdPayloadV2 `json:"payload"`
}

type pdPayloadV2 struct {
	Summary       string         `json:"summary"`
	Severity      string         `json:"severity"` // critical, error, warning, info
	Source        string         `json:"source"`
	CustomDetails map[string]any `json:"custom_details,omitempty"`
}

func (c *Client) pageEventsAPIV2(ctx context.Context, alert ports.Alert) error {
	payload := pdPayloadV2{
		Summary:  alert.Message,
		Severity: pdSeverity(alert.Severity),
		Source:   "golem-slo",
		CustomDetails: map[string]any{
			"sli":         alert.SLIName,
			"budget_left": alert.BudgetLeft,
			"route":       alert.Route,
		},
	}

	event := pdEventV2{
		RoutingKey:  c.routingKey,
		EventAction: "trigger",
		DedupKey:    fmt.Sprintf("golem-slo-%s", alert.SLIName),
		Payload:     payload,
	}

	return c.sendEvent(ctx, "https://events.pagerduty.com/v2/enqueue", event)
}

// --- PagerDuty Events API v1 ---

type pdEventV1 struct {
	ServiceKey  string         `json:"service_key"`
	EventType   string         `json:"event_type"` // trigger, acknowledge, resolve
	Description string         `json:"description"`
	IncidentKey string         `json:"incident_key,omitempty"`
	Client      string         `json:"client,omitempty"`
	Details     map[string]any `json:"details,omitempty"`
}

func (c *Client) pageEventsAPIV1(ctx context.Context, alert ports.Alert) error {
	event := pdEventV1{
		ServiceKey:  c.serviceKey,
		EventType:   "trigger",
		Description: alert.Message,
		IncidentKey: fmt.Sprintf("golem-slo-%s", alert.SLIName),
		Client:      "golem-slo-evaluator",
		Details: map[string]any{
			"sli":         alert.SLIName,
			"severity":    alert.Severity,
			"budget_left": alert.BudgetLeft,
			"route":       alert.Route,
		},
	}

	return c.sendEvent(ctx, "https://events.pagerduty.com/generic/2010-04-15/create_event.json", event)
}

func (c *Client) sendEvent(ctx context.Context, url string, event any) error {
	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("pager: marshal error: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("pager: request error: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("pager: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("pager: unexpected status %d", resp.StatusCode)
	}
	return nil
}

func pdSeverity(s ports.AlertSeverity) string {
	switch s {
	case ports.AlertSeverityCritical:
		return "critical"
	case ports.AlertSeverityHigh:
		return "error"
	case ports.AlertSeverityMedium:
		return "warning"
	case ports.AlertSeverityLow:
		return "info"
	default:
		return "info"
	}
}

// Ensure Client implements ports.Paging
var _ ports.Paging = (*Client)(nil)
