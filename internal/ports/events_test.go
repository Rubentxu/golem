package ports

import (
	"encoding/json"
	"testing"
	"time"
)

func TestEnvelopeJSONRoundTrip(t *testing.T) {
	ts := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)
	env := Envelope[map[string]any]{
		EventID:       "01JTEST0000000000000000000",
		TenantID:      "t_test",
		StreamID:      "workitem:01J...",
		EventType:     "work.item.created.v1",
		SchemaVersion: 1,
		OccurredAt:    ts,
		Actor:         Actor{Type: "user", ID: "u_1"},
		CorrelationID: "01JC...",
		Payload:       map[string]any{"title": "first slice"},
	}

	data, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got Envelope[map[string]any]
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.EventType != env.EventType || got.TenantID != env.TenantID {
		t.Fatalf("roundtrip mismatch: got %+v", got)
	}
	if !got.OccurredAt.Equal(ts) {
		t.Fatalf("timestamp mismatch: got %v want %v", got.OccurredAt, ts)
	}
}
