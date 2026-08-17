package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	checkpointmem "github.com/Rubentxu/golem/adapters/checkpoint/memstore"
	graphmem "github.com/Rubentxu/golem/adapters/graph/memstore"
	journalmem "github.com/Rubentxu/golem/adapters/journal/memstore"
	otelobs "github.com/Rubentxu/golem/adapters/observability/otel"
	registrymem "github.com/Rubentxu/golem/adapters/registry/memstore"
	searchmem "github.com/Rubentxu/golem/adapters/search/memstore"
	transportmem "github.com/Rubentxu/golem/adapters/transport/memstore"
	"github.com/Rubentxu/golem/internal/api/httpapi"
	appci "github.com/Rubentxu/golem/internal/application/ci"
	appplanning "github.com/Rubentxu/golem/internal/application/planning"
	appprojects "github.com/Rubentxu/golem/internal/application/projects"
	appreq "github.com/Rubentxu/golem/internal/application/requirements"
	"github.com/Rubentxu/golem/internal/application/runtime"
	appscm "github.com/Rubentxu/golem/internal/application/scm"
	appver "github.com/Rubentxu/golem/internal/application/verification"
	appwork "github.com/Rubentxu/golem/internal/application/work"
)

// golem-api is the API edge composition root (ARCHITECTURE stage A:
// modular monolith — the process hosts the kernel and its tail loops;
// adapters are selected here, ADR-045). The default profile wires the
// in-memory reference adapters, suitable for development and the M1
// demo; provider profiles (durable journal, NATS transport) arrive with
// the M5 Provider Profile mechanism.
func main() {
	obsbundle, shutdownObs, err := otelobs.Setup(context.Background(), "golem-api", "0.1.0")
	if err != nil {
		log.Fatalf("observability setup: %v", err)
	}
	defer func() { _ = shutdownObs(context.Background()) }()

	rt, err := runtime.New(runtime.Options{
		Journal:    journalmem.NewJournal(),
		Graph:      graphmem.NewGraph(),
		Registry:   registrymem.NewRegistry(),
		Transport:  transportmem.NewTransport(),
		Checkpoint: checkpointmem.NewCheckpoints(),
		Search:     searchmem.NewSearch(),
		Obs:        obsbundle,
	})
	if err != nil {
		log.Fatal(err)
	}
	rt.Bus.Register(appwork.CmdCreateWorkItem, appwork.CreateWorkItemHandler(rt.IDs, rt.Graph))
	rt.Bus.Register(appwork.CmdUpdateWorkItem, appwork.UpdateWorkItemHandler(rt.Journal, rt.Graph))
	rt.Bus.Register(appwork.CmdLinkWorkItems, appwork.LinkWorkItemsHandler(rt.Graph))
	rt.Bus.Register(appwork.CmdRegisterWorkType, appwork.RegisterWorkTypeHandler())
	rt.Bus.Register(appwork.CmdAddComment, appwork.AddCommentHandler(rt.IDs, rt.Journal))
	rt.Bus.Register(appreq.CmdCreateRequirement, appreq.CreateRequirementHandler(rt.IDs))
	rt.Bus.Register(appprojects.CmdCreateProject, appprojects.CreateProjectHandler(rt.IDs))
	rt.Bus.Register(appplanning.CmdCreateIteration, appplanning.CreateIterationHandler(rt.IDs))
	rt.Bus.Register(appplanning.CmdCreateMilestone, appplanning.CreateMilestoneHandler(rt.IDs))
	rt.Bus.Register(appscm.CmdObserveCommit, appscm.ObserveCommitHandler())
	rt.Bus.Register(appci.CmdCompleteBuild, appci.CompleteBuildHandler(rt.IDs, rt.Journal))
	rt.Bus.Register(appver.CmdReportTestRun, appver.ReportTestRunHandler(rt.IDs, rt.Graph))

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Tail loops: project the journal into the graph and pump the outbox.
	go func() {
		const batchSize = 100
		if err := rt.Run(ctx, batchSize, 100*time.Millisecond); err != nil && ctx.Err() == nil {
			log.Printf("runtime loops stopped: %v", err)
		}
	}()

	addr := os.Getenv("GOLEM_API_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	srv := &http.Server{
		Addr: addr,
		Handler: httpapi.New(rt.Bus, rt.Graph, rt.Journal).
			WithSearch(rt.Search).
			WithObservability(obsbundle).
			Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	log.Printf("golem-api listening on %s (in-memory profile)", addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
	log.Print("golem-api: stopped")
}
