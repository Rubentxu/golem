// Command golem-worker runs the journal tail loops of one GOLEM process:
// graph projection (Journal → Engineering Graph) and the outbox publisher
// (Journal → EventTransport), each with its own checkpoint.
package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"
	"time"

	otelobs "github.com/Rubentxu/golem/adapters/observability/otel"
	"github.com/Rubentxu/golem/cmd/golem/bootstrap"
	"github.com/Rubentxu/golem/internal/profile"
)

func main() {
	prof, err := profile.LoadFromEnv()
	if err != nil {
		log.Fatalf("profile load: %v", err)
	}
	log.Printf("profile=%s", prof.Name)

	obsbundle, shutdownObs, err := otelobs.Setup(context.Background(), "golem-worker", "0.1.0")
	if err != nil {
		log.Fatalf("observability setup: %v", err)
	}
	defer func() { _ = shutdownObs(context.Background()) }()

	rt, err := bootstrap.NewRuntimeFromProfile(prof, obsbundle)
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
