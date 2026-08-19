// Package work provides narrow-port adapters over the journal store.
package work

import (
	"context"
	"encoding/json"
	"time"

	"github.com/Rubentxu/golem/internal/ports"
)

// journalStoreWorkItemWriter implements WorkItemWriter over a JournalStore.
type journalStoreWorkItemWriter struct {
	jrnl ports.JournalStore
}

// NewWorkItemWriterOverJournal creates a WorkItemWriter that appends to the journal.
func NewWorkItemWriterOverJournal(jrnl ports.JournalStore) WorkItemWriter {
	return &journalStoreWorkItemWriter{jrnl: jrnl}
}

// AppendCommand implements WorkItemWriter by appending to the work item's own stream.
func (w *journalStoreWorkItemWriter) AppendCommand(ctx context.Context, cmd WorkItemCommand) error {
	// Encode the payload as JSON RawMessage
	var payload json.RawMessage
	if cmd.Payload != nil {
		payload = cmd.Payload
	}
	raw := ports.RawEvent{
		EventID:    cmd.Name + "-" + cmd.ItemID, // synthetic ID for the command record
		TenantID:   string(cmd.TenantID),
		StreamID:   "workitem:" + cmd.ItemID,
		EventType:  cmd.Name,
		OccurredAt: time.Now(),
		Actor:      ports.Actor{Type: "service", ID: "work-item-writer"},
		Payload:    payload,
	}
	_, err := w.jrnl.Append(ctx, []ports.RawEvent{raw})
	return err
}
