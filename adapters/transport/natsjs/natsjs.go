// Package natsjs is the NATS JetStream adapter of the EventTransport
// port (ADR-012: reference event transport, never a core dependency;
// ADR-033: the broker is transport, never the source of truth).
//
// Mapping:
//   - One stream per transport instance (default "GOLEM") with subject
//     namespace "golem.>", file storage.
//   - Subject per event: golem.<tenant_id>.<event_type> — tenant and
//     context stay filterable at the broker without exposing payloads.
//   - Publish deduplicates by EventID via the JetStream Nats-Msg-Id
//     header (duplicates window), matching the port's at-least-once
//     contract.
//   - Fetch uses a durable pull consumer with explicit ack; unacked
//     events are redelivered after AckWait. Ack is idempotent.
//
// Vendor types (nats.Msg and friends) never cross the boundary: only
// ports.RawEvent is visible to callers (ADR-047).
package natsjs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/Rubentxu/golem/internal/ports"
)

// Config for the JetStream transport.
type Config struct {
	URL          string
	Stream       string
	Consumer     string // durable pull consumer name
	SubjectRoot  string // default "golem"
	AckWait      time.Duration
	MaxWaitFetch time.Duration
}

func (c *Config) setDefaults() {
	if c.Stream == "" {
		c.Stream = "GOLEM"
	}
	if c.Consumer == "" {
		c.Consumer = "golem-worker"
	}
	if c.SubjectRoot == "" {
		c.SubjectRoot = "golem"
	}
	if c.AckWait == 0 {
		c.AckWait = 30 * time.Second
	}
	if c.MaxWaitFetch == 0 {
		c.MaxWaitFetch = 500 * time.Millisecond
	}
}

// Transport is a JetStream EventTransport.
type Transport struct {
	cfg  Config
	conn *nats.Conn
	js   nats.JetStreamContext
	sub  *nats.Subscription

	mu      sync.Mutex
	pending map[string]*nats.Msg // event_id -> awaiting ack
}

// Connect establishes the connection and ensures stream + consumer exist.
func Connect(ctx context.Context, cfg Config) (*Transport, error) {
	cfg.setDefaults()
	if cfg.URL == "" {
		return nil, fmt.Errorf("natsjs: URL is mandatory")
	}
	_ = ctx // connection lifetime is managed by Close; NATS handles keepalive

	conn, err := nats.Connect(cfg.URL,
		nats.Name("golem-transport"),
		nats.MaxReconnects(-1),
	)
	if err != nil {
		return nil, fmt.Errorf("natsjs: connect: %w", err)
	}
	js, err := conn.JetStream()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("natsjs: jetstream context: %w", err)
	}

	root := cfg.SubjectRoot + ".>"
	if _, err := js.AddStream(&nats.StreamConfig{
		Name:     cfg.Stream,
		Subjects: []string{root},
		Storage:  nats.FileStorage,
	}); err != nil {
		conn.Close()
		return nil, fmt.Errorf("natsjs: ensure stream %s: %w", cfg.Stream, err)
	}

	cons, err := js.AddConsumer(cfg.Stream, &nats.ConsumerConfig{
		Durable:       cfg.Consumer,
		AckPolicy:     nats.AckExplicitPolicy,
		DeliverPolicy: nats.DeliverAllPolicy,
		FilterSubject: root,
		AckWait:       cfg.AckWait,
	})
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("natsjs: ensure consumer %s: %w", cfg.Consumer, err)
	}
	_ = cons

	// Bind a pull subscription to the durable consumer above.
	sub, err := js.PullSubscribe(root, cfg.Consumer, nats.BindStream(cfg.Stream))
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("natsjs: pull subscribe %s: %w", cfg.Consumer, err)
	}

	return &Transport{
		cfg:     cfg,
		conn:    conn,
		js:      js,
		sub:     sub,
		pending: map[string]*nats.Msg{},
	}, nil
}

// Publish encodes and publishes events; duplicates collapse via Msg-Id.
func (t *Transport) Publish(ctx context.Context, events []ports.RawEvent) error {
	for _, e := range events {
		data, err := json.Marshal(e)
		if err != nil {
			return fmt.Errorf("natsjs: encode %s: %w", e.EventID, err)
		}
		if _, err := t.js.PublishMsg(&nats.Msg{
			Subject: t.subject(e),
			Data:    data,
		}, nats.MsgId(e.EventID), nats.AckWait(t.cfg.AckWait)); err != nil {
			return fmt.Errorf("natsjs: publish %s: %w", e.EventID, err)
		}
	}
	_ = ctx
	return nil
}

// Fetch pulls up to max undelivered events. An empty batch is a nil
// slice with nil error.
func (t *Transport) Fetch(ctx context.Context, max int) ([]ports.RawEvent, error) {
	if max <= 0 {
		return nil, nil
	}
	msgs, err := t.sub.Fetch(max, nats.MaxWait(t.cfg.MaxWaitFetch))
	if err != nil {
		if errorsIsTimeout(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("natsjs: fetch: %w", err)
	}
	out := []ports.RawEvent{}
	for _, msg := range msgs {
		var env ports.RawEvent
		if err := json.Unmarshal(msg.Data, &env); err != nil {
			// Poison message: ack it so the queue drains, surface the error.
			_ = msg.Ack()
			return nil, fmt.Errorf("natsjs: decode event: %w", err)
		}
		t.mu.Lock()
		t.pending[env.EventID] = msg
		t.mu.Unlock()
		out = append(out, env)
	}
	_ = ctx
	return out, nil
}

// Ack acknowledges an event; unknown or already-acked ids are no-ops.
func (t *Transport) Ack(_ context.Context, eventID string) error {
	t.mu.Lock()
	msg, ok := t.pending[eventID]
	delete(t.pending, eventID)
	t.mu.Unlock()
	if !ok {
		return nil
	}
	if err := msg.Ack(); err != nil && !errorsIsAlreadyAcked(err) {
		return fmt.Errorf("natsjs: ack %s: %w", eventID, err)
	}
	return nil
}

// Close releases the connection.
func (t *Transport) Close() error {
	t.conn.Close()
	return nil
}

// DeleteStream removes the stream and its state. Admin operation for
// tests and teardown; production streams are long-lived.
func (t *Transport) DeleteStream() error {
	if err := t.js.DeleteStream(t.cfg.Stream); err != nil {
		return fmt.Errorf("natsjs: delete stream %s: %w", t.cfg.Stream, err)
	}
	return nil
}

func (t *Transport) subject(e ports.RawEvent) string {
	return strings.Join([]string{t.cfg.SubjectRoot, e.TenantID, e.EventType}, ".")
}

func errorsIsTimeout(err error) bool {
	return errors.Is(err, nats.ErrTimeout) || errors.Is(err, context.DeadlineExceeded)
}

func errorsIsAlreadyAcked(err error) bool {
	return strings.Contains(err.Error(), "already acknowledged")
}
