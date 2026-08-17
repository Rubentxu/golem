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
	appreq "github.com/Rubentxu/golem/internal/application/requirements"
	"github.com/Rubentxu/golem/internal/application/runtime"
	appscm "github.com/Rubentxu/golem/internal/application/scm"
	appver "github.com/Rubentxu/golem/internal/application/verification"
	appwork "github.com/Rubentxu/golem/internal/application/work"
)

// TestM3Lineage proves the M3 exit criterion end to end over HTTP:
// Requirement → Commit → Build → Artifact → TestRun, all queryable from
// the trace endpoint, with content-addressed artifact identity (ADR-022)
// and validation gates (unknown commit rejected, bad digest rejected).
func TestM3Lineage(t *testing.T) {
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
	rt.Bus.Register(appwork.CmdCreateWorkItem, appwork.CreateWorkItemHandler(rt.IDs, rt.Graph))
	rt.Bus.Register(appreq.CmdCreateRequirement, appreq.CreateRequirementHandler(rt.IDs))
	rt.Bus.Register(appscm.CmdObserveCommit, appscm.ObserveCommitHandler())
	rt.Bus.Register(appci.CmdCompleteBuild, appci.CompleteBuildHandler(rt.IDs, rt.Journal))
	rt.Bus.Register(appver.CmdReportTestRun, appver.ReportTestRunHandler(rt.IDs, rt.Graph))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = rt.Run(ctx, 10, 5*time.Millisecond) }()

	srv := httptest.NewServer(httpapi.New(rt.Bus, rt.Graph, rt.Journal).Handler())
	defer srv.Close()
	client := srv.Client()

	post := func(path, idemKey, body string) (int, httpapi.Receipt) {
		t.Helper()
		req, _ := http.NewRequest(http.MethodPost, srv.URL+path, strings.NewReader(body))
		req.Header.Set("X-Golem-Tenant", "t_m3")
		req.Header.Set("Idempotency-Key", idemKey)
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var rec httpapi.Receipt
		if err := json.NewDecoder(resp.Body).Decode(&rec); err != nil {
			t.Fatalf("decode receipt %s: %v", resp.Status, err)
		}
		return resp.StatusCode, rec
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

	// 1. Requirement.
	if code, _ := post("/api/v1/requirements", "m3-req-0001", `{"title":"Artifact lineage","statement":"full chain"}`); code != http.StatusAccepted {
		t.Fatalf("requirement: %d", code)
	}
	drain()
	evs, _, _ := rt.Journal.Replay(ctx, 0, 0)
	reqID := ""
	for _, e := range evs {
		if strings.HasPrefix(e.StreamID, "requirement:") {
			reqID = e.StreamID[len("requirement:"):]
		}
	}
	if reqID == "" {
		t.Fatal("requirement id not recoverable")
	}

	// 2. Commit implementing the requirement.
	sha := strings.Repeat("a1b2c3d4e5", 4) // 40 hex
	if code, _ := post("/api/v1/scm/commits", "m3-cmt-0001",
		`{"sha":"`+sha+`","repository":"github.com/Rubentxu/golem","message":"feat: lineage","implements":["`+reqID+`"]}`); code != http.StatusAccepted {
		t.Fatalf("commit: %d", code)
	}
	// Malformed sha rejected.
	if code, _ := post("/api/v1/scm/commits", "m3-cmt-0002", `{"sha":"abc","repository":"x"}`); code != http.StatusUnprocessableEntity {
		t.Fatalf("bad sha: %d", code)
	}

	// 3. Build of the commit, producing a content-addressed artifact.
	digest := "sha256:" + strings.Repeat("ab", 32)
	buildBody := `{"pipeline":"release","commit":"` + sha + `","status":"success","artifacts":[{"digest":"` + digest + `","name":"golem-api","kind":"ContainerImage"}]}`
	if code, _ := post("/api/v1/ci/builds", "m3-bld-0001", buildBody); code != http.StatusAccepted {
		t.Fatalf("build: %d", code)
	}
	// Build of an unknown commit rejected.
	if code, _ := post("/api/v1/ci/builds", "m3-bld-0002",
		`{"pipeline":"x","commit":"`+strings.Repeat("ff", 20)+`","status":"success"}`); code != http.StatusNotFound {
		t.Fatalf("unknown commit build: %d", code)
	}
	// Non content-addressed digest rejected.
	if code, _ := post("/api/v1/ci/builds", "m3-bld-0003",
		`{"pipeline":"release","commit":"`+sha+`","status":"success","artifacts":[{"digest":"latest","name":"x"}]}`); code != http.StatusUnprocessableEntity {
		t.Fatalf("bad digest: %d", code)
	}

	// 4. Test run verifying the artifact.
	drain()
	if code, _ := post("/api/v1/test/runs", "m3-run-0001",
		`{"case":"smoke","status":"passed","verifies":"`+digest+`"}`); code != http.StatusAccepted {
		t.Fatalf("test run: %d", code)
	}
	// Run against a ghost target rejected.
	if code, _ := post("/api/v1/test/runs", "m3-run-0002",
		`{"case":"smoke","status":"passed","verifies":"sha256:dead"}`); code != http.StatusNotFound {
		t.Fatalf("ghost verifies: %d", code)
	}
	drain()

	// 5. Trace from the requirement: the whole chain must be visible.
	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/v1/trace/"+reqID+"?depth=6", nil)
	req.Header.Set("X-Golem-Tenant", "t_m3")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("trace: %s", resp.Status)
	}
	var trace struct {
		Root     string `json:"root"`
		Subgraph struct {
			Nodes []struct {
				ID   string `json:"id"`
				Kind string `json:"kind"`
			} `json:"nodes"`
			Edges []struct {
				Type     string `json:"type"`
				SourceID string `json:"source_id"`
				TargetID string `json:"target_id"`
			} `json:"edges"`
		} `json:"subgraph"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&trace); err != nil {
		t.Fatal(err)
	}

	kinds := map[string]bool{}
	artifactKinds := map[string]bool{"Artifact": true, "Package": true, "ContainerImage": true, "Release": true}
	hasArtifact := false
	for _, n := range trace.Subgraph.Nodes {
		kinds[n.Kind] = true
		if n.ID == digest && artifactKinds[n.Kind] {
			hasArtifact = true
		}
	}
	for _, want := range []string{"Requirement", "Commit", "Build", "TestRun"} {
		if !kinds[want] {
			t.Fatalf("trace missing %s; got kinds %v (nodes=%d edges=%d)", want, kinds, len(trace.Subgraph.Nodes), len(trace.Subgraph.Edges))
		}
	}
	if !hasArtifact {
		t.Fatalf("trace missing content-addressed artifact %s; got kinds %v", digest, kinds)
	}
	relTypes := map[string]bool{}
	for _, e := range trace.Subgraph.Edges {
		relTypes[e.Type] = true
	}
	for _, want := range []string{"IMPLEMENTS", "BUILT_BY", "PRODUCED", "VERIFIES"} {
		if !relTypes[want] {
			t.Fatalf("trace missing %s edge; got %v", want, relTypes)
		}
	}

	// 6. Trace from the artifact (blast-radius direction) also sees the chain.
	req, _ = http.NewRequest(http.MethodGet, srv.URL+"/api/v1/trace/"+digest, nil)
	req.Header.Set("X-Golem-Tenant", "t_m3")
	resp2, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("trace from artifact: %s", resp2.Status)
	}

	// 7. Unknown entity: 404 problem.
	req, _ = http.NewRequest(http.MethodGet, srv.URL+"/api/v1/trace/ghost", nil)
	req.Header.Set("X-Golem-Tenant", "t_m3")
	resp3, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp3.Body.Close()
	if resp3.StatusCode != http.StatusNotFound {
		t.Fatalf("trace ghost: %s", resp3.Status)
	}
}
