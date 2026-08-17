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
	registrymem "github.com/Rubentxu/golem/adapters/registry/memstore"
	transportmem "github.com/Rubentxu/golem/adapters/transport/memstore"
	"github.com/Rubentxu/golem/internal/api/httpapi"
	"github.com/Rubentxu/golem/internal/application/runtime"
	appwork "github.com/Rubentxu/golem/internal/application/work"
)

// golem-api is the API edge composition root (ARCHITECTURE stage A:
// modular monolith — the process hosts the kernel and its tail loops;
// adapters are selected here, ADR-045). The default profile wires the
// in-memory reference adapters, suitable for development and the M1
// demo; provider profiles (durable journal, NATS transport) arrive with
// the M5 Provider Profile mechanism.
func main() {
	rt, err := runtime.New(runtime.Options{
		Journal:    journalmem.NewJournal(),
		Graph:      graphmem.NewGraph(),
		Registry:   registrymem.NewRegistry(),
		Transport:  transportmem.NewTransport(),
		Checkpoint: checkpointmem.NewCheckpoints(),
	})
	if err != nil {
		log.Fatal(err)
	}
	rt.Bus.Register(appwork.CmdCreateWorkItem, appwork.CreateWorkItemHandler(rt.IDs))

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
		Addr:              addr,
		Handler:           httpapi.New(rt.Bus, rt.Graph).Handler(),
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
