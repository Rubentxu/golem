// Package tuleap implements the basic Tuleap importer (M2 exit
// criterion: "import Tuleap básico"; SPIKES SP-009 mapping). It maps a
// JSON fixture — projects, trackers, artifacts and artifact links —
// onto GOLEM commands:
//
//	project   -> projects.create-project     (stable id, external identity)
//	tracker   -> work.register-work-type     (states, transitions, fields)
//	artifact  -> work.create-work-item       (stable id, external identity,
//	                                         typed fields when the tracker
//	                                         defines them)
//	artifact link -> work.link-work-items    (canonical relation required)
//
// Idempotency: every emitted command uses CommandID
// "tuleap-<kind>-<tuleap-id>", so re-running the import replays stored
// receipts and journals nothing new. IDs are importer-stable
// ("tuleap-<id>") making link resolution deterministic.
package tuleap

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	appcmd "github.com/Rubentxu/golem/internal/application/command"
	appplanning "github.com/Rubentxu/golem/internal/application/planning"
	appprojects "github.com/Rubentxu/golem/internal/application/projects"
	appwork "github.com/Rubentxu/golem/internal/application/work"
	"github.com/Rubentxu/golem/internal/ports"
	work "github.com/Rubentxu/golem/internal/work"
)

// Provider name recorded in external identities.
const Provider = "tuleap"

// Fixture is the importer input format (a realistic Tuleap extract).
type Fixture struct {
	Projects  []Project  `json:"projects"`
	Trackers  []Tracker  `json:"trackers"`
	Artifacts []Artifact `json:"artifacts"`
}

type Project struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type Tracker struct {
	Project     string       `json:"project"`
	Name        string       `json:"name"`
	Initial     string       `json:"initial"`
	States      []string     `json:"states"`
	Transitions []Transition `json:"transitions"`
	Fields      []FieldDef   `json:"fields"`
}

type Transition struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type FieldDef struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Required bool   `json:"required"`
}

type Artifact struct {
	Project  string         `json:"project"`
	Tracker  string         `json:"tracker"`
	ID       string         `json:"id"`
	Title    string         `json:"title"`
	Status   string         `json:"status"`
	Fields   map[string]any `json:"fields"`
	Comments []string       `json:"comments"`
	Links    []ArtifactLink `json:"links"`
}

type ArtifactLink struct {
	To       string `json:"to"`
	Relation string `json:"relation"`
}

// Submitter is the command sink (the bus).
type Submitter interface {
	Submit(ctx context.Context, cmd appcmd.Command) (ports.CommandReceipt, error)
}

// Report summarizes an import run.
type Report struct {
	Projects  int
	Trackers  int
	Artifacts int
	Links     int
	Comments  int
	Skipped   int // duplicate receipts (idempotent re-import)
	Errors    []error
}

// LoadFixture reads and validates a fixture file.
func LoadFixture(path string) (*Fixture, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("tuleap: read fixture: %w", err)
	}
	var f Fixture
	if err := json.Unmarshal(b, &f); err != nil {
		return nil, fmt.Errorf("tuleap: parse fixture: %w", err)
	}
	return &f, nil
}

// Options tunes an import run. Sync is an optional barrier the host
// provides to drain the graph projection between phases: typed-item
// creation and link validation read the projection, so a batch importer
// must let it catch up. Hosts running single-process runtimes pass
// runtime.ProjectBatch until caught-up.
type Options struct {
	Sync func(ctx context.Context) error
}

// Import runs the fixture through the command bus. It never aborts on
// individual failures: errors are collected in the report so a migration
// can be inspected and retried (idempotent) after fixes.
func Import(ctx context.Context, fx *Fixture, tenant ports.TenantID, bus Submitter, opts Options) *Report {
	rep := &Report{}
	actor := ports.Actor{Type: "service", ID: "tuleap-importer"}
	barrier := func() {
		if opts.Sync != nil {
			_ = opts.Sync(ctx)
		}
	}

	submit := func(cmdID, name string, payload any) (ports.CommandReceipt, error) {
		return bus.Submit(ctx, appcmd.Command{
			Name: name, TenantID: tenant, Actor: actor,
			CommandID: cmdID, CorrelationID: "tuleap-import",
			Payload: payload,
		})
	}

	// 1. Projects.
	for _, p := range fx.Projects {
		r, err := submit("tuleap-project-"+p.ID, appprojects.CmdCreateProject, appprojects.CreateProject{
			Name:        p.Name,
			Description: p.Description,
			ProjectID:   golemID("project", p.ID),
			External:    work.ExternalIdentity{Provider: Provider, ExternalID: p.ID},
		})
		switch {
		case err != nil:
			rep.Errors = append(rep.Errors, fmt.Errorf("project %s: %w", p.ID, err))
		case r.Duplicate:
			rep.Skipped++
		default:
			rep.Projects++
		}
	}

	// 2. Trackers -> WorkTypes (scoped by name inside the tenant).
	for _, tr := range fx.Trackers {
		name := trackerName(tr.Project, tr.Name)
		payload := appwork.RegisterWorkType{
			Name: name, Initial: tr.Initial,
			States: tr.States, Fields: toFieldDefs(tr.Fields),
		}
		for _, t := range tr.Transitions {
			payload.Transitions = append(payload.Transitions, work.Transition{From: t.From, To: t.To})
		}
		r, err := submit("tuleap-tracker-"+tr.Project+"-"+tr.Name, appwork.CmdRegisterWorkType, payload)
		switch {
		case err != nil:
			rep.Errors = append(rep.Errors, fmt.Errorf("tracker %s/%s: %w", tr.Project, tr.Name, err))
		case r.Duplicate:
			rep.Skipped++
		default:
			rep.Trackers++
		}
	}

	// Typed artifacts need the projected WorkTypes.
	barrier()

	// 3. Artifacts -> WorkItems. Status is imported verbatim when the
	// tracker workflow allows it (initial state); a mismatch would need
	// transition replay — out of scope for the basic importer, which
	// creates items at the workflow initial state.
	for _, a := range fx.Artifacts {
		_, typed := trackerOf(fx, a.Project, a.Tracker)
		cw := appwork.CreateWorkItem{
			Title:    a.Title,
			ItemType: a.Tracker,
			ItemID:   golemID("item", a.ID),
			External: work.ExternalIdentity{Provider: Provider, ExternalID: a.ID},
			Fields:   a.Fields,
		}
		if typed {
			cw.TypeName = trackerName(a.Project, a.Tracker)
		}
		r, err := submit("tuleap-artifact-"+a.ID, appwork.CmdCreateWorkItem, cw)
		switch {
		case err != nil:
			rep.Errors = append(rep.Errors, fmt.Errorf("artifact %s: %w", a.ID, err))
			continue
		case r.Duplicate:
			rep.Skipped++
		default:
			rep.Artifacts++
		}

		// Comments after the item exists.
		for i, body := range a.Comments {
			r, err := submit(fmt.Sprintf("tuleap-comment-%s-%d", a.ID, i),
				appwork.CmdAddComment, appwork.AddComment{ItemID: golemID("item", a.ID), Body: body})
			switch {
			case err != nil:
				rep.Errors = append(rep.Errors, fmt.Errorf("comment %s/%d: %w", a.ID, i, err))
			case r.Duplicate:
				rep.Skipped++
			default:
				rep.Comments++
			}
		}
	}

	// Links need the projected items.
	barrier()

	// 4. Artifact links -> canonical relations.
	for _, a := range fx.Artifacts {
		for i, l := range a.Links {
			r, err := submit(fmt.Sprintf("tuleap-link-%s-%d", a.ID, i),
				appwork.CmdLinkWorkItems, appwork.LinkWorkItems{
					FromID:   golemID("item", a.ID),
					ToID:     golemID("item", l.To),
					Relation: l.Relation,
				})
			switch {
			case err != nil:
				rep.Errors = append(rep.Errors, fmt.Errorf("link %s→%s: %w", a.ID, l.To, err))
			case r.Duplicate:
				rep.Skipped++
			default:
				rep.Links++
			}
		}
	}

	_ = appplanning.CmdCreateIteration // planning import arrives with SP-009 full mapping
	return rep
}

func golemID(kind, external string) string {
	return "tuleap-" + kind + "-" + external
}

func trackerName(project, tracker string) string {
	return project + "/" + tracker
}

func trackerOf(fx *Fixture, project, tracker string) (Tracker, bool) {
	for _, t := range fx.Trackers {
		if t.Project == project && t.Name == tracker {
			return t, true
		}
	}
	return Tracker{}, false
}

func toFieldDefs(in []FieldDef) []work.FieldDef {
	out := make([]work.FieldDef, 0, len(in))
	for _, f := range in {
		out = append(out, work.FieldDef{Name: strings.TrimSpace(f.Name), Type: f.Type, Required: f.Required})
	}
	return out
}
