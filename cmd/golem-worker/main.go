// Command golem-worker runs the journal tail loops of one GOLEM process:
// graph projection (Journal → Engineering Graph) and the outbox publisher
// (Journal → EventTransport), each with its own checkpoint.
//
// It is the composition root for worker deployments: adapter selection
// happens here (ADR-045). The default profile wires the in-memory
// reference adapters — which are process-local — so standalone workers
// are only meaningful with shared adapters (NATS transport, durable
// journal); today this binary exists to exercise the runtime end to end
// and will grow Provider Profiles in M5.
package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"
	"time"

	checkpointmem "github.com/Rubentxu/golem/adapters/checkpoint/memstore"
	graphmem "github.com/Rubentxu/golem/adapters/graph/memstore"
	journalmem "github.com/Rubentxu/golem/adapters/journal/memstore"
	otelobs "github.com/Rubentxu/golem/adapters/observability/otel"
	registrymem "github.com/Rubentxu/golem/adapters/registry/memstore"
	transportmem "github.com/Rubentxu/golem/adapters/transport/memstore"
	"github.com/Rubentxu/golem/internal/application/runtime"
)

func main() {
	obsbundle, shutdownObs, err := otelobs.Setup(context.Background(), "golem-worker", "0.1.0")
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
		Obs:        obsbundle,
	})
	if err != nil {
		log.Fatal(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	const batchSize = 100
	log.Printf("golem-worker: projection + outbox loops (batch=%d)", batchSize)
	if err := rt.Run(ctx, batchSize, 250*time.Millisecond); err != nil && ctx.Err() == nil {
		log.Fatal(err)
	}
	log.Print("golem-worker: stopped")
}
