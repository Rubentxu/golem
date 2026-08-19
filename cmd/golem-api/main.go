package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	otelobs "github.com/Rubentxu/golem/adapters/observability/otel"
	fsregistry "github.com/Rubentxu/golem/adapters/registry/filesystem"
	"github.com/Rubentxu/golem/cmd/golem/bootstrap"
	"github.com/Rubentxu/golem/internal/api/httpapi"
	"github.com/Rubentxu/golem/internal/api/httpapi/admin"
	appci "github.com/Rubentxu/golem/internal/application/ci"
	"github.com/Rubentxu/golem/internal/application/ingest"
	appplanning "github.com/Rubentxu/golem/internal/application/planning"
	"github.com/Rubentxu/golem/internal/application/projection"
	appprojects "github.com/Rubentxu/golem/internal/application/projects"
	apprelease "github.com/Rubentxu/golem/internal/application/release"
	appro "github.com/Rubentxu/golem/internal/application/requirements"
	appscm "github.com/Rubentxu/golem/internal/application/scm"
	"github.com/Rubentxu/golem/internal/application/supplychain"
	appver "github.com/Rubentxu/golem/internal/application/verification"
	appwork "github.com/Rubentxu/golem/internal/application/work"
	"github.com/Rubentxu/golem/internal/profile"
)

func main() {
	prof, err := profile.LoadFromEnv()
	if err != nil {
		log.Fatalf("profile load: %v", err)
	}
	log.Printf("profile=%s", prof.Name)

	obsbundle, shutdownObs, err := otelobs.Setup(context.Background(), "golem-api", "0.1.0")
	if err != nil {
		log.Fatalf("observability setup: %v", err)
	}
	defer func() { _ = shutdownObs(context.Background()) }()

	rt, err := bootstrap.NewRuntimeFromProfile(prof, obsbundle)
	if err != nil {
		log.Fatal(err)
	}

	// Register supplychain projection into the global registry (strangler-fig pattern).
	reg := projection.NewRegistry()
	reg.Register(supplychain.NewProjection())
	projection.SetGlobal(reg)

	rt.Bus.Register(appwork.CmdCreateWorkItem, appwork.CreateWorkItemHandler(rt.IDs, rt.Graph))
	rt.Bus.Register(appwork.CmdUpdateWorkItem, appwork.UpdateWorkItemHandler(rt.Journal, rt.Graph))
	rt.Bus.Register(appwork.CmdLinkWorkItems, appwork.LinkWorkItemsHandler(rt.Graph))
	rt.Bus.Register(appwork.CmdRegisterWorkType, appwork.RegisterWorkTypeHandler())
	rt.Bus.Register(appwork.CmdAddComment, appwork.AddCommentHandler(rt.IDs, rt.Journal))
	rt.Bus.Register(appro.CmdCreateRequirement, appro.CreateRequirementHandler(rt.IDs))
	rt.Bus.Register(appprojects.CmdCreateProject, appprojects.CreateProjectHandler(rt.IDs))
	rt.Bus.Register(appplanning.CmdCreateIteration, appplanning.CreateIterationHandler(rt.IDs))
	rt.Bus.Register(appplanning.CmdCreateMilestone, appplanning.CreateMilestoneHandler(rt.IDs))
	rt.Bus.Register(appscm.CmdObserveCommit, appscm.ObserveCommitHandler())
	rt.Bus.Register(appci.CmdCompleteBuild, appci.CompleteBuildHandler(rt.IDs, rt.Journal))
	rt.Bus.Register(appver.CmdReportTestRun, appver.ReportTestRunHandler(rt.IDs, rt.Graph))
	rt.Bus.Register(apprelease.CmdCreateCandidate, apprelease.CreateCandidateHandler(rt.IDs, rt.Graph))
	rt.Bus.Register(apprelease.CmdEvaluateGate, apprelease.EvaluateGateHandler(rt.Graph))

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Tail loops: project the journal into the graph and pump the outbox.
	go func() {
		const batchSize = 100
		if err := rt.Run(ctx, batchSize, 100*time.Millisecond); err != nil && ctx.Err() == nil {
			log.Printf("runtime loops stopped: %v", err)
		}
	}()

	// Wire M8 admin endpoints (ADR-081) — nil-safe: admin mux only mounted if runtime ports exist
	var adminMux *admin.AdminMux
	if rt.CellRouter != nil || rt.TenantCatalog != nil || rt.SLOTracker != nil || rt.UsageMeter != nil {
		cells := admin.NewCellsHandler(rt.CellRouter, rt.TenantCatalog, nil)
		queries := admin.NewQueriesHandler(rt.SLOTracker, rt.UsageMeter)
		adminMux = admin.NewAdminMux(cells, queries)
	}

	addr := os.Getenv("GOLEM_API_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	srv := &http.Server{
		Addr: addr,
		Handler: httpapi.New(rt.Bus, rt.Graph, rt.Journal).
			WithSearch(rt.Search).
			WithIngest(ingest.New(rt.Bus)).
			WithPacks(fsregistry.New(fsregistry.DefaultRoot, rt.Journal, rt.IDs, rt.Clock)).
			WithObservability(obsbundle).
			WithAdminHandlers(adminMux).
			Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	log.Printf("golem-api listening on %s", addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
	log.Print("golem-api: stopped")
}
