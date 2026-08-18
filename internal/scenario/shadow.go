package scenario

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Rubentxu/golem/internal/behavior"
	"github.com/Rubentxu/golem/internal/ports"
)

// ShadowReport is the behavior v1/v2 diff output — the second half of the
// M6 exit criterion. Deterministic (sequential execution, ordered diffs).
type ShadowReport struct {
	EventCount int          `json:"event_count"`
	Diffs      []ShadowDiff `json:"diffs"`
}

// ShadowDiff records one event where v1 and v2 outcomes diverged.
type ShadowDiff struct {
	EventID    string `json:"event_id"`
	V1Outcome  any    `json:"v1_outcome"`
	V2Outcome  any    `json:"v2_outcome"`
	Difference string `json:"difference"` // human-readable summary
}

// Shadow runs two behavior versions side-by-side over the same events
// with the same engine wiring, sequentially (deterministic — Q5), and
// diffs their observable outcomes. The behaviors must share
// subscriptions; each event is processed once against both.
func Shadow(ctx context.Context, engine *behavior.Engine, graph ports.GraphStore, events []ports.RawEvent, v1, v2 *behavior.Behavior) (*ShadowReport, error) {
	if len(v1.Subscriptions) != len(v2.Subscriptions) {
		return nil, fmt.Errorf("scenario: shadow behaviors must share subscriptions (v1=%v v2=%v)", v1.Subscriptions, v2.Subscriptions)
	}
	report := &ShadowReport{EventCount: len(events), Diffs: []ShadowDiff{}}
	for _, ev := range events {
		o1 := runOne(ctx, engine, graph, ev, v1)
		o2 := runOne(ctx, engine, graph, ev, v2)
		// Compare the SEMANTIC outcome only: outputs and skip reason. The
		// outcome identity (BehaviorID/Version) intentionally differs
		// between versions.
		type semantic struct {
			Output  behavior.HandlerOutput `json:"output"`
			Skipped string                 `json:"skipped,omitempty"`
		}
		j1, _ := json.Marshal(semantic{Output: o1.Output, Skipped: o1.Skipped})
		j2, _ := json.Marshal(semantic{Output: o2.Output, Skipped: o2.Skipped})
		if string(j1) != string(j2) {
			report.Diffs = append(report.Diffs, ShadowDiff{
				EventID:    ev.EventID,
				V1Outcome:  o1,
				V2Outcome:  o2,
				Difference: diffSummary(o1, o2),
			})
		}
	}
	return report, nil
}

// runOne executes one behavior version for one event in isolation: the
// engine is re-wired with a single-behavior registry so outcomes never
// interleave between versions.
func runOne(ctx context.Context, engine *behavior.Engine, graph ports.GraphStore, ev ports.RawEvent, b *behavior.Behavior) behavior.Outcome {
	reg := behavior.NewRegistry()
	_ = reg.Register(b)
	isolated := behavior.NewEngine(reg, graph, engine.Clock())
	outcomes, err := isolated.Handle(ctx, ev)
	if err != nil {
		return behavior.Outcome{BehaviorID: b.ID, Version: b.Version, Skipped: "error: " + err.Error()}
	}
	if len(outcomes) == 0 {
		return behavior.Outcome{BehaviorID: b.ID, Version: b.Version, Skipped: "no candidates"}
	}
	return outcomes[0]
}

func diffSummary(o1, o2 behavior.Outcome) string {
	e1 := len(o1.Output.Events)
	e2 := len(o2.Output.Events)
	p1 := len(o1.Output.Proposals)
	p2 := len(o2.Output.Proposals)
	return fmt.Sprintf("events v1=%d v2=%d; proposals v1=%d v2=%d; skipped v1=%q v2=%q", e1, e2, p1, p2, o1.Skipped, o2.Skipped)
}
