package tck_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	checkpointmem "github.com/Rubentxu/golem/adapters/checkpoint/memstore"
	graphmem "github.com/Rubentxu/golem/adapters/graph/memstore"
	journalmem "github.com/Rubentxu/golem/adapters/journal/memstore"
	registrymem "github.com/Rubentxu/golem/adapters/registry/memstore"
	searchmem "github.com/Rubentxu/golem/adapters/search/memstore"
	transportmem "github.com/Rubentxu/golem/adapters/transport/memstore"
	"github.com/Rubentxu/golem/internal/api/httpapi"
	appci "github.com/Rubentxu/golem/internal/application/ci"
	"github.com/Rubentxu/golem/internal/application/ingest"
	apprelease "github.com/Rubentxu/golem/internal/application/release"
	"github.com/Rubentxu/golem/internal/application/runtime"
	appscm "github.com/Rubentxu/golem/internal/application/scm"
	appver "github.com/Rubentxu/golem/internal/application/verification"
	appwork "github.com/Rubentxu/golem/internal/application/work"
	"github.com/Rubentxu/golem/internal/ports"
)

// TestIngestAndReleaseGate proves the provider event sinks (external
// idempotency via derived command ids) and the evidence-based release
// gate: webhooks build the chain, the gate goes green only after a
// passed test run, and webhook redelivery journals nothing.
func TestIngestAndReleaseGate(t *testing.T) {
	rt, err := runtime.New(runtime.Options{
		Journal:    journalmem.NewJournal(),
		Graph:      graphmem.NewGraph(),
		Registry:   registrymem.NewRegistry(),
		Transport:  transportmem.NewTransport(),
		Checkpoint: checkpointmem.NewCheckpoints(),
		Search:     searchmem.NewSearch(),
	})
	if err != nil {
		t.Fatal(err)
	}
	rt.Bus.Register(appwork.CmdCreateWorkItem, appwork.CreateWorkItemHandler(rt.IDs, appwork.NewWorkItemReaderOverGraphStore(rt.Graph)))
	rt.Bus.Register(appscm.CmdObserveCommit, appscm.ObserveCommitHandler())
	rt.Bus.Register(appci.CmdCompleteBuild, appci.CompleteBuildHandler(rt.IDs, appci.NewSCMStreamReaderOverJournal(rt.Journal)))
	rt.Bus.Register(appver.CmdReportTestRun, appver.ReportTestRunHandler(rt.IDs, ports.NewEntityRefReaderOverGraphStore(rt.Graph)))
	rt.Bus.Register(apprelease.CmdCreateCandidate, apprelease.CreateCandidateHandler(rt.IDs, appci.NewArtifactReaderOverGraphStore(rt.Graph)))
	rt.Bus.Register(apprelease.CmdEvaluateGate, apprelease.EvaluateGateHandler(apprelease.NewReleaseGraphReaderOverGraphStore(rt.Graph), apprelease.NewSupplyChainEvidenceReaderOverGraphStore(rt.Graph), apprelease.NewArtifactVerifierOverGraphStore(rt.Graph)))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = rt.Run(ctx, 10, 5*time.Millisecond) }()

	ingestSvc := ingest.New(rt.Bus)
	srv := httptest.NewServer(httpapi.New(rt.Bus, rt.Graph, rt.Journal).
		WithSearch(rt.Search).WithIngest(ingestSvc).Handler())
	defer srv.Close()
	client := srv.Client()

	post := func(path, idemKey, body string) (int, map[string]any) {
		t.Helper()
		req, _ := http.NewRequest(http.MethodPost, srv.URL+path, strings.NewReader(body))
		req.Header.Set("X-Golem-Tenant", "t_sink")
		if idemKey != "" {
			req.Header.Set("Idempotency-Key", idemKey)
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var v map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&v); err != nil {
			t.Fatalf("decode %s: %v", resp.Status, err)
		}
		return resp.StatusCode, v
	}
	get := func(path string) (int, map[string]any) {
		t.Helper()
		req, _ := http.NewRequest(http.MethodGet, srv.URL+path, nil)
		req.Header.Set("X-Golem-Tenant", "t_sink")
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var v map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&v); err != nil {
			t.Fatal(err)
		}
		return resp.StatusCode, v
	}
	drain := func() {
		t.Helper()
		for {
			n, _ := rt.ProjectBatch(ctx, 10)
			m, _ := rt.PublishBatch(ctx, 10)
			s, _ := rt.SearchBatch(ctx, 10)
			if n == 0 && m == 0 && s == 0 {
				return
			}
		}
	}

	sha := strings.Repeat("c0ffee11", 5) // 40 hex
	digest := "sha256:" + strings.Repeat("9d", 32)

	// 1. GitHub push webhook with one commit.
	push := `{"repository":{"full_name":"Rubentxu/golem"},"commits":[{"id":"` + sha + `","message":"feat: sink"}]}`
	code, rep := post("/api/v1/ingest/github", "", push)
	if code != http.StatusAccepted || rep["accepted"].(float64) != 1 {
		t.Fatalf("push ingest: %d %+v", code, rep)
	}

	// Redelivery of the same webhook: duplicate, nothing new.
	headAfterFirst, _ := rt.Journal.Head(ctx)
	code, rep = post("/api/v1/ingest/github", "", push)
	if code != http.StatusAccepted || rep["duplicates"].(float64) != 1 || rep["accepted"].(float64) != 0 {
		t.Fatalf("push redelivery: %d %+v", code, rep)
	}
	if head, _ := rt.Journal.Head(ctx); head != headAfterFirst {
		t.Fatalf("redelivery journaled events: %d -> %d", headAfterFirst, head)
	}

	// Unknown provider: 400.
	if code, _ := post("/api/v1/ingest/bitbucket", "", `{"x":1}`); code != http.StatusBadRequest {
		t.Fatalf("unknown provider: %d", code)
	}

	// 2. CI webhook completes a build with the artifact.
	drain()
	build := `{"external_build_id":"b-77","pipeline":"release","commit":"` + sha + `","status":"success","artifacts":[{"digest":"` + digest + `","name":"golem-api","kind":"ContainerImage"}]}`
	if code, rep = post("/api/v1/ingest/ci-generic", "", build); code != http.StatusAccepted || rep["accepted"].(float64) != 1 {
		t.Fatalf("ci ingest: %d %+v", code, rep)
	}
	// Redelivery: duplicate by external build id.
	if code, rep = post("/api/v1/ingest/ci-generic", "", build); code != http.StatusAccepted || rep["duplicates"].(float64) != 1 {
		t.Fatalf("ci redelivery: %d %+v", code, rep)
	}
	drain()

	// 3. Release candidate composed of the artifact.
	if code, _ = post("/api/v1/releases", "rel-key-0001", `{"name":"v0.1.0-rc1","artifacts":["`+digest+`"]}`); code != http.StatusAccepted {
		t.Fatalf("release create: %d", code)
	}
	// RC referencing an unknown artifact: 404.
	if code, _ = post("/api/v1/releases", "rel-key-0002", `{"name":"bad","artifacts":["sha256:`+strings.Repeat("00", 32)+`"]}`); code != http.StatusNotFound {
		t.Fatalf("rc unknown artifact: %d", code)
	}
	drain()

	// Recover the release id from the journal.
	evs, _, _ := rt.Journal.Replay(ctx, 0, 0)
	releaseID := ""
	for _, e := range evs {
		if strings.HasPrefix(e.StreamID, "release:") && strings.Contains(string(e.Payload), "v0.1.0-rc1") {
			releaseID = e.StreamID[len("release:"):]
		}
	}
	if releaseID == "" {
		t.Fatal("release id not recoverable")
	}

	// 4. Gate evaluation before any test: red.
	if code, _ = post("/api/v1/releases/"+releaseID+"/gate", "gate-key-0001", `{}`); code != http.StatusAccepted {
		t.Fatalf("gate eval 1: %d", code)
	}
	drain()
	code, rel := get("/api/v1/releases/" + releaseID)
	if code != http.StatusOK {
		t.Fatalf("get release: %d", code)
	}
	attrs := rel["attributes"].(map[string]any)
	if attrs["gate_status"] != "red" {
		t.Fatalf("gate before tests = %v, want red", attrs["gate_status"])
	}

	// 5. Passed test run on the artifact, re-evaluate: green.
	if code, _ = post("/api/v1/test/runs", "sink-run-0001", `{"case":"smoke","status":"passed","verifies":"`+digest+`"}`); code != http.StatusAccepted {
		t.Fatalf("test run: %d", code)
	}
	drain()
	if code, _ = post("/api/v1/releases/"+releaseID+"/gate", "gate-key-0002", `{}`); code != http.StatusAccepted {
		t.Fatalf("gate eval 2: %d", code)
	}
	drain()
	_, rel = get("/api/v1/releases/" + releaseID)
	attrs = rel["attributes"].(map[string]any)
	if attrs["gate_status"] != "green" {
		t.Fatalf("gate after passed run = %v, want green", attrs["gate_status"])
	}

	// Unknown release: 404.
	if code, _ = post("/api/v1/releases/ghost/gate", "gate-key-0003", `{}`); code != http.StatusNotFound {
		t.Fatalf("gate unknown release: %d", code)
	}
	if code, _ = get("/api/v1/releases/ghost"); code != http.StatusNotFound {
		t.Fatalf("get unknown release: %d", code)
	}
}
