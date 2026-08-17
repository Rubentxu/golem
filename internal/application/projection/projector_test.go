package projection

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/Rubentxu/golem/internal/ports"
	"github.com/Rubentxu/golem/internal/work"
)

func mkEvent(eventType string, payload any) ports.RawEvent {
	b, err := json.Marshal(payload)
	if err != nil {
		panic(err)
	}
	return ports.RawEvent{
		EventID:       "01JTEST0000000000000000EV1",
		TenantID:      "t_test",
		StreamID:      "workitem:wi-1",
		EventType:     eventType,
		SchemaVersion: 1,
		OccurredAt:    time.Now().UTC(),
		Actor:         ports.Actor{Type: "user", ID: "u_1"},
		Payload:       b,
	}
}

func TestProjectItemCreated(t *testing.T) {
	m, err := (Projector{}).Project(mkEvent(work.EventItemCreated, work.ItemCreated{
		ItemID: "wi-1", Title: "Kernel slice", ItemType: "task", Status: "open",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Ops) != 1 {
		t.Fatalf("ops = %d, want 1", len(m.Ops))
	}
	op := m.Ops[0]
	if op.Kind != ports.OpUpsertNode || op.Target != "wi-1" || op.Data["kind"] != KindWorkItem {
		t.Fatalf("op = %+v", op)
	}
	if m.TenantID != "t_test" {
		t.Fatalf("tenant = %q, want t_test", m.TenantID)
	}
}

func TestProjectItemUpdatedOnlyChangedFields(t *testing.T) {
	newTitle := "renamed"
	m, err := (Projector{}).Project(mkEvent(work.EventItemUpdated, work.ItemUpdated{
		ItemID: "wi-1", Title: &newTitle,
	}))
	if err != nil {
		t.Fatal(err)
	}
	attrs := m.Ops[0].Data["attributes"].(map[string]any)
	if _, hasStatus := attrs["status"]; hasStatus {
		t.Fatalf("nil status must not be projected: %+v", attrs)
	}
	if attrs["title"] != newTitle {
		t.Fatalf("title = %v", attrs["title"])
	}
}

func TestProjectItemLinkedUsesEventIDAsEdgeID(t *testing.T) {
	m, err := (Projector{}).Project(mkEvent(work.EventItemLinked, work.ItemLinked{
		FromID: "wi-1", ToID: "wi-2", Relation: " depends_on ",
	}))
	if err != nil {
		t.Fatal(err)
	}
	op := m.Ops[0]
	if op.Kind != ports.OpUpsertEdge || op.Target != "01JTEST0000000000000000EV1" {
		t.Fatalf("op = %+v", op)
	}
	if op.Data["type"] != "DEPENDS_ON" {
		t.Fatalf("relation not canonicalized: %v", op.Data["type"])
	}
}

func TestProjectUnknownEventIsSkipped(t *testing.T) {
	m, err := (Projector{}).Project(mkEvent("planning.iteration.created.v1", map[string]any{"id": "it-1"}))
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Ops) != 0 {
		t.Fatalf("unknown event produced ops: %+v", m.Ops)
	}
}

func TestProjectInvalidPayloadErrors(t *testing.T) {
	ev := mkEvent(work.EventItemCreated, nil)
	ev.Payload = []byte(`{not json`)
	if _, err := (Projector{}).Project(ev); err == nil {
		t.Fatal("expected payload decode error")
	}
}
