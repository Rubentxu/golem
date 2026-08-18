package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Rubentxu/golem/internal/ports"
)

// auditEvent is the payload of a migration harness audit event.
type auditEvent struct {
	HarnessID      string      `json:"harness_id"`
	SourceKind     string      `json:"source_kind"`
	TargetKind     string      `json:"target_kind"`
	OccurredAt     time.Time   `json:"occurred_at"`
	DiffCounts     *diffCounts `json:"diff_counts,omitempty"`
	RollbackReason string      `json:"rollback_reason,omitempty"`
	FailedStep     string      `json:"failed_step,omitempty"`
}

type diffCounts struct {
	Nodes int `json:"nodes"`
	Edges int `json:"edges"`
}

// audit emits a migration harness audit event to the journal.
type audit struct {
	journal    ports.JournalStore
	ids        ports.IDGenerator
	clock      ports.Clock
	harnessID  string
	sourceKind string
	targetKind string
}

// newAudit creates an audit emitter for a harness run.
func newAudit(journal ports.JournalStore, ids ports.IDGenerator, clk ports.Clock, harnessID, sourceKind, targetKind string) *audit {
	return &audit{
		journal:    journal,
		ids:        ids,
		clock:      clk,
		harnessID:  harnessID,
		sourceKind: sourceKind,
		targetKind: targetKind,
	}
}

// emit sends an event to the journal.
func (a *audit) emit(ctx context.Context, eventType string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("audit marshal: %w", err)
	}
	env := ports.RawEvent{
		EventID:       a.ids.NewID(),
		TenantID:      "system",
		StreamID:      fmt.Sprintf("harness:%s", a.harnessID),
		EventType:     eventType,
		SchemaVersion: 1,
		OccurredAt:    a.clock.Now(),
		Actor: ports.Actor{
			Type: "service",
			ID:   "migration-harness",
		},
		Payload: data,
	}
	_, err = a.journal.Append(ctx, []ports.RawEvent{env})
	return err
}

// started emits migration.harness.started.v1.
func (a *audit) started(ctx context.Context) error {
	return a.emit(ctx, ports.EventMigrationHarnessStarted, auditEvent{
		HarnessID:  a.harnessID,
		SourceKind: a.sourceKind,
		TargetKind: a.targetKind,
		OccurredAt: a.clock.Now(),
	})
}

// diffed emits migration.harness.diffed.v1 with diff counts.
func (a *audit) diffed(ctx context.Context, nodeDiffs, edgeDiffs int) error {
	return a.emit(ctx, ports.EventMigrationHarnessDiffed, auditEvent{
		HarnessID:  a.harnessID,
		SourceKind: a.sourceKind,
		TargetKind: a.targetKind,
		OccurredAt: a.clock.Now(),
		DiffCounts: &diffCounts{Nodes: nodeDiffs, Edges: edgeDiffs},
	})
}

// cutover emits migration.harness.cutover.v1 with cutover_safe.
func (a *audit) cutover(ctx context.Context, cutoverSafe bool) error {
	return a.emit(ctx, ports.EventMigrationHarnessCutover, auditEvent{
		HarnessID:  a.harnessID,
		SourceKind: a.sourceKind,
		TargetKind: a.targetKind,
		OccurredAt: a.clock.Now(),
	})
}

// completed emits migration.harness.completed.v1.
func (a *audit) completed(ctx context.Context) error {
	return a.emit(ctx, ports.EventMigrationHarnessCompleted, auditEvent{
		HarnessID:  a.harnessID,
		SourceKind: a.sourceKind,
		TargetKind: a.targetKind,
		OccurredAt: a.clock.Now(),
	})
}

// rolledBack emits migration.harness.rolled_back.v1.
func (a *audit) rolledBack(ctx context.Context, reason RollbackReason, failedStep string) error {
	return a.emit(ctx, ports.EventMigrationHarnessRolledBack, auditEvent{
		HarnessID:      a.harnessID,
		SourceKind:     a.sourceKind,
		TargetKind:     a.targetKind,
		OccurredAt:     a.clock.Now(),
		RollbackReason: string(reason),
		FailedStep:     failedStep,
	})
}
